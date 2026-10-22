package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/minio/minio-go/v7"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"
	pkgcrypto "go-wind-ops/pkg/crypto"
	"go-wind-ops/pkg/machine"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/shellexec"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const (
	// artifactBucket 构建产物的对象存储桶。
	artifactBucket = "artifacts"
	// cloneTimeout git clone 超时。
	cloneTimeout = 10 * time.Minute
	// buildLogLimit 构建日志落库上限 512KB。
	buildLogLimit = 512 << 10
	// defaultBuildTimeoutSeconds 构建超时默认(秒)。
	defaultBuildTimeoutSeconds = 600
	// maxBuildTimeoutSeconds 构建超时上限(秒)。
	maxBuildTimeoutSeconds = 3600
)

// BuildService 构建配置管理与执行:v1 在 core 所在机器构建(git clone →
// 构建命令 → 产物打包 → MinIO 上传),执行器为进程内 goroutine,core 重启
// 时本实例遗留的未完成历史由启动清扫定案 FAILED(多副本部署下按 runner_id
// 限定范围,不误伤其他副本正在跑的构建)。
type BuildService struct {
	opsV1.UnimplementedBuildServiceServer

	log       *log.Helper
	repo      *data.BuildRepo
	repoRepo  *data.RepositoryRepo
	historyRepo *data.BuildHistoryRepo
	ossClient *oss.MinIOClient

	workspaceRoot string
	// runnerID 本实例的执行标识(机器指纹):写入构建历史,启动清扫与孤儿
	// 清扫据此区分"本实例遗留"与"其他副本在跑"。
	runnerID string
}

func NewBuildService(ctx *bootstrap.Context, repo *data.BuildRepo, repoRepo *data.RepositoryRepo, historyRepo *data.BuildHistoryRepo, ossClient *oss.MinIOClient) *BuildService {
	workspaceRoot := os.Getenv("OPS_BUILD_WORKSPACE")
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join("data", "builds")
	}

	runnerID := machine.MachineID()

	s := &BuildService{
		log:           ctx.NewLoggerHelper("build/service/core-service"),
		repo:          repo,
		repoRepo:      repoRepo,
		historyRepo:   historyRepo,
		ossClient:     ossClient,
		workspaceRoot: workspaceRoot,
		runnerID:      runnerID,
	}

	// 启动清扫:本实例(同机器指纹)上次重启中断的构建定案 FAILED。
	// 其他实例的未终态历史归对方的孤儿清扫(阈值内)或周期兜底(超阈值)管。
	if n, err := historyRepo.FinalizeInterruptedByRunner(ctx.Context(), runnerID); err == nil && n > 0 {
		s.log.Infof("finalized %d interrupted build history record(s) (runner=%s)", n, runnerID)
	}

	return s
}

func (s *BuildService) ListBuilds(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBuildResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *BuildService) GetBuild(ctx context.Context, req *opsV1.GetBuildRequest) (*opsV1.Build, error) {
	return s.repo.Get(ctx, req)
}

func (s *BuildService) CreateBuild(ctx context.Context, req *opsV1.CreateBuildRequest) (*opsV1.Build, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetRepositoryId() == 0 || req.Data.GetBuildCommand() == "" || req.Data.GetArtifactPath() == "" {
		return nil, opsV1.ErrorBadRequest("name/repositoryId/buildCommand/artifactPath are required")
	}
	if req.Data.GetTimeoutSeconds() > maxBuildTimeoutSeconds {
		return nil, opsV1.ErrorBadRequest("timeout exceeds limit (3600s)")
	}

	if _, err := s.repoRepo.Get(ctx, &opsV1.GetRepositoryRequest{Id: req.Data.RepositoryId}); err != nil {
		return nil, opsV1.ErrorBadRequest("repository not found")
	}

	taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("build name already exists")
	}

	return s.repo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *BuildService) UpdateBuild(ctx context.Context, req *opsV1.UpdateBuildRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetTimeoutSeconds() > maxBuildTimeoutSeconds {
		return nil, opsV1.ErrorBadRequest("timeout exceeds limit (3600s)")
	}

	if req.Data.Name != nil {
		taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("build name already exists")
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *BuildService) DeleteBuild(ctx context.Context, req *opsV1.DeleteBuildRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// TriggerBuild 触发构建:独占式创建历史(事务内 FOR UPDATE 锁配置行,复查
// 未终态 + 推进构建号原子完成)→ goroutine 异步执行。手动/API/webhook/cron
// 全部路径收敛于此,并发触发由 CreateExclusive 去重(冲突 409)。
func (s *BuildService) TriggerBuild(ctx context.Context, req *opsV1.TriggerBuildRequest) (*opsV1.TriggerBuildResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}

	buildDto, err := s.repo.Get(ctx, &opsV1.GetBuildRequest{Id: &req.Id})
	if err != nil {
		return nil, err
	}

	repoEntity, err := s.repoRepo.GetEntity(ctx, buildDto.GetRepositoryId())
	if err != nil {
		return nil, opsV1.ErrorBadRequest("repository not found")
	}

	branch := resolveBranch(buildDto.GetBranch(), repoEntity)

	history, conflict, err := s.historyRepo.CreateExclusive(ctx, buildDto.GetId(), branch, operatorIDOf(ctx), s.runnerID)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, opsV1.ErrorConflict("build already running")
	}

	go s.runBuild(history.ID)

	s.log.Infof("build triggered (build=%d history=%d no=%d branch=%s runner=%s)",
		buildDto.GetId(), history.ID, history.BuildNo, branch, s.runnerID)
	return &opsV1.TriggerBuildResponse{HistoryId: history.ID, BuildNo: history.BuildNo}, nil
}

// ============================================================================
// 构建执行器(进程内 goroutine)
// ============================================================================

func (s *BuildService) runBuild(historyID uint32) {
	ctx := context.Background()
	// 日志缓冲由主 goroutine 写、flush goroutine 每 5s 读,统一走互斥锁
	// (string 复合结构非原子,无锁读写是数据竞争)。
	var (
		logMu    sync.Mutex
		logLines string
	)
	appendLog := func(delta string) {
		logMu.Lock()
		logLines += delta
		logMu.Unlock()
	}
	setLog := func(text string) {
		logMu.Lock()
		logLines = text
		logMu.Unlock()
	}
	logSnapshot := func() string {
		logMu.Lock()
		defer logMu.Unlock()
		return logLines
	}
	fail := func(errMsg string) {
		logText := logSnapshot()
		if errMsg != "" {
			logText = strings.TrimSpace(logText + "\n[ERROR] " + errMsg)
		}
		if updErr := s.historyRepo.UpdateFinished(ctx, historyID, "FAILED", truncateBuildLog(logText), errMsg, "", "", 0); updErr != nil {
			s.log.Errorf("finalize build history [%d] failed: %s", historyID, updErr.Error())
		}
	}

	history, err := s.historyRepo.GetEntity(ctx, historyID)
	if err != nil {
		s.log.Errorf("runBuild: load history [%d]: %s", historyID, err.Error())
		return
	}

	buildDto, err := s.repo.Get(ctx, &opsV1.GetBuildRequest{Id: &history.BuildID})
	if err != nil {
		fail("load build config: "+err.Error())
		return
	}
	repoEntity, err := s.repoRepo.GetEntity(ctx, buildDto.GetRepositoryId())
	if err != nil {
		fail("load repository: "+err.Error())
		return
	}

	_ = s.historyRepo.UpdateRunning(ctx, historyID)

	branch := resolveBranch(buildDto.GetBranch(), repoEntity)
	workspace := filepath.Join(s.workspaceRoot, fmt.Sprintf("%d", history.BuildID), fmt.Sprintf("%d", history.BuildNo))
	srcDir := filepath.Join(workspace, "src")
	defer os.RemoveAll(workspace)

	// ── 1. git clone(argv 直传,URL 不经 shell)──
	if err = os.MkdirAll(workspace, 0o755); err != nil {
		fail("prepare workspace: " + err.Error())
		return
	}
	code, out, timedOut, err := shellexec.RunGit(ctx, workspace, cloneTimeout,
		"clone", "--depth", "1", "--single-branch", "--branch", branch, buildGitURL(repoEntity), "src")
	setLog("$ git clone --depth 1 --single-branch --branch " + branch + " <url> src\n" + out)

	// 克隆完成即增量落库(前端立即可见)
	_ = s.historyRepo.UpdateLogText(ctx, historyID, truncateBuildLog(logSnapshot()))

	if timedOut {
		fail("git clone timeout")
		return
	}
	if err != nil || code != 0 {
		fail(fmt.Sprintf("git clone failed (exit=%d): %s", code, err))
		return
	}

	// ── 2. 构建命令(后台执行 + 定期刷新日志到 DB)──
	timeoutSeconds := buildDto.GetTimeoutSeconds()
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultBuildTimeoutSeconds
	}
	if timeoutSeconds > maxBuildTimeoutSeconds {
		timeoutSeconds = maxBuildTimeoutSeconds
	}
	buildCmd := buildDto.GetBuildCommand()
	appendLog("\n\n$ " + buildCmd + "\n")
	// 先落库"构建命令开始执行",让前端看到当前状态
	_ = s.historyRepo.UpdateLogText(ctx, historyID, truncateBuildLog(logSnapshot()))

	// 定期刷新日志(构建命令可能运行几分钟;让前端 3s 轮询可见进展)
	flushDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-flushDone:
				return
			case <-ticker.C:
				_ = s.historyRepo.UpdateLogText(ctx, historyID, truncateBuildLog(logSnapshot()))
			}
		}
	}()

	code, out, timedOut, err = shellexec.Run(ctx, srcDir, buildCmd, time.Duration(timeoutSeconds)*time.Second)
	close(flushDone)
	appendLog(out)
	if timedOut {
		fail(fmt.Sprintf("build command timeout after %ds", timeoutSeconds))
		return
	}
	if err != nil || code != 0 {
		fail(fmt.Sprintf("build command failed (exit=%d): %s", code, err))
		return
	}

	// ── 3. 收集产物并打包 ──
	artifactFile, err := collectArtifacts(workspace, srcDir, buildDto.GetArtifactPath())
	if err != nil {
		fail("collect artifacts: " + err.Error())
		return
	}

	// ── 4. 上传 MinIO ──
	objectKey := fmt.Sprintf("builds/%d/%d/%s", history.BuildID, history.BuildNo, filepath.Base(artifactFile))
	size, err := s.uploadArtifact(ctx, artifactFile, objectKey)
	if err != nil {
		fail("upload artifact: " + err.Error())
		return
	}

	// ── 成功终态 ──
	if updErr := s.historyRepo.UpdateFinished(ctx, historyID, "SUCCESS", truncateBuildLog(logSnapshot()), "", artifactBucket, objectKey, size); updErr != nil {
		s.log.Errorf("finalize build history [%d]: %s", historyID, updErr.Error())
	}
	s.log.Infof("build succeeded (history=%d no=%d artifact=%s size=%d)", historyID, history.BuildNo, objectKey, size)
}

// collectArtifacts 按 glob 收集产物;单个文件原样返回,多个匹配打包 zip。
func collectArtifacts(workspace, srcDir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
	if err != nil {
		return "", fmt.Errorf("invalid artifact pattern: %w", err)
	}

	var files []string
	for _, m := range matches {
		info, statErr := os.Stat(m)
		if statErr != nil || info.IsDir() {
			continue
		}
		files = append(files, m)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("artifact %q not found in repository", pattern)
	}

	if len(files) == 1 {
		// 单文件:平移到 workspace 下,保持原始文件名
		target := filepath.Join(workspace, filepath.Base(files[0]))
		if err = os.Rename(files[0], target); err != nil {
			// 跨目录 rename 失败时回退复制
			if cpErr := copyFile(files[0], target); cpErr != nil {
				return "", cpErr
			}
		}
		return target, nil
	}

	// 多文件:打包 zip(内部路径相对 srcDir)
	zipPath := filepath.Join(workspace, "artifact.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	for _, f := range files {
		arcName, relErr := filepath.Rel(srcDir, f)
		if relErr != nil {
			arcName = filepath.Base(f)
		}
		if err = addToZip(zipWriter, f, arcName); err != nil {
			return "", err
		}
	}
	if err = zipWriter.Close(); err != nil {
		return "", err
	}
	return zipPath, nil
}

func addToZip(zipWriter *zip.Writer, path, arcName string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(arcName)
	header.Method = zip.Deflate
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// uploadArtifact 上传产物到 MinIO(artifacts 桶),返回大小。
func (s *BuildService) uploadArtifact(ctx context.Context, filePath, objectKey string) (uint64, error) {
	if s.ossClient == nil {
		return 0, fmt.Errorf("oss client not configured")
	}
	client := s.ossClient.GetClient()
	if client == nil {
		return 0, fmt.Errorf("minio client not initialized")
	}

	if err := s.ossClient.EnsureBucketExists(ctx, artifactBucket); err != nil {
		return 0, fmt.Errorf("ensure bucket: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	contentType := "application/octet-stream"
	if strings.HasSuffix(filePath, ".zip") {
		contentType = "application/zip"
	}
	_, err = client.PutObject(ctx, artifactBucket, objectKey, file, info.Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return 0, err
	}
	return uint64(info.Size()), nil
}

// buildGitURL 组装克隆地址:
//   - 本地盘符路径(D:/...)转 file:/// URL——否则 git 把 `D:` 当 scp 语法的
//     主机名,报 "hostname contains invalid characters";
//   - HTTPS 且配置了凭据时注入 userinfo(密码解密)。
func buildGitURL(repoEntity *ent.Repository) string {
	u := repoEntity.GitURL

	// 本地路径 → file:/// URL
	if len(u) >= 2 && u[1] == ':' && (u[0] >= 'A' && u[0] <= 'Z' || u[0] >= 'a' && u[0] <= 'z') {
		return "file:///" + strings.ReplaceAll(filepath.ToSlash(u), "\\", "/")
	}
	if strings.HasPrefix(u, "/") || strings.HasPrefix(u, "\\\\") {
		return "file://" + filepath.ToSlash(u)
	}

	if repoEntity.Username == nil || repoEntity.Password == nil {
		return u
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return u
	}
	pu, err := url.Parse(u)
	if err != nil {
		return u
	}
	decUser, err1 := pkgcrypto.DecryptIfNeeded(*repoEntity.Username)
	decPass, err2 := pkgcrypto.DecryptIfNeeded(*repoEntity.Password)
	if err1 != nil || err2 != nil {
		return u
	}
	pu.User = url.UserPassword(decUser, decPass)
	return pu.String()
}

func resolveBranch(buildBranch string, repoEntity *ent.Repository) string {
	if buildBranch != "" {
		return buildBranch
	}
	if repoEntity.Branch != nil && *repoEntity.Branch != "" {
		return *repoEntity.Branch
	}
	return "main"
}

func truncateBuildLog(s string) string {
	if len(s) <= buildLogLimit {
		return s
	}
	return s[:buildLogLimit]
}
