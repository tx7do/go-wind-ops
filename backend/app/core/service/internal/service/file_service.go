// file_service.go 远端文件管理:core 经 MQTT 同步指令代理 agent 侧的
// 文件浏览/读写/删除。路径合法性由 agent 的授权目录白名单兜底校验。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tx7do/go-utils/id"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// fileSyncTimeout 单条文件指令的回执等待上限。
const fileSyncTimeout = 15 * time.Second

type FileService struct {
	opsV1.UnimplementedFileServiceServer

	log      *log.Helper
	agentSvc *AgentService
	nodeRepo *data.AgentNodeRepo
	ossClient *oss.MinIOClient
}

func NewFileService(ctx *bootstrap.Context, agentSvc *AgentService, nodeRepo *data.AgentNodeRepo, ossClient *oss.MinIOClient) *FileService {
	return &FileService{
		log:      ctx.NewLoggerHelper("file/service/core-service"),
		agentSvc: agentSvc,
		nodeRepo: nodeRepo,
		ossClient: ossClient,
	}
}

// callSync 下发文件指令并等待回执;agent 侧失败时转为 gRPC 错误。
func (s *FileService) callSync(ctx context.Context, agentId string, action signal.CommandAction, payload any) (*signal.CommandResult, error) {
	res, err := s.agentSvc.PublishCommandSync(ctx, 0, agentId, action, payload, operatorIDOf(ctx), fileSyncTimeout)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("file command failed: %s", err.Error())
	}
	if res == nil || !res.Success {
		msg := "unknown error"
		if res != nil && res.Error != "" {
			msg = res.Error
		}
		return nil, opsV1.ErrorInternalServerError("agent rejected %s: %s", action, msg)
	}
	return res, nil
}

// resolveAgentID 兼容前端传节点 ID 或 agent 标识;返回可用于 MQTT 主题的 agentId。
func (s *FileService) resolveAgentID(ctx context.Context, agentId string) (string, error) {
	if agentId == "" {
		return "", opsV1.ErrorBadRequest("agentId required")
	}
	if s.nodeRepo != nil {
		if _, err := s.nodeRepo.GetByAgentID(ctx, agentId); err != nil {
			return "", opsV1.ErrorNotFound("agent not enrolled: %s", agentId)
		}
	}
	return agentId, nil
}

// ── 大文件通道:agent ↔ 对象存储 ↔ 浏览器 ──

const (
	filePresignExpiry  = 15 * time.Minute // 预签名有效期
	fileTransferSync   = 10 * time.Minute // agent 传输回执等待上限
	fileObjectPrefix   = "agent-files"    // 传输对象键前缀(Complete 时防越权)
	fileBucketTransfer = oss.BucketFiles
)

// fileTransferObjectKey 生成传输对象键:agent-files/{agentId}/{uuid}{ext}。
func fileTransferObjectKey(agentId, path string) string {
	return fmt.Sprintf("%s/%s/%s%s", fileObjectPrefix, agentId, id.NewGUIDv4(false), filepath.Ext(path))
}

// presignPut/Get 预签名工具(含桶确保存在)。
func (s *FileService) presignPut(ctx context.Context, objectKey string) (string, error) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return "", opsV1.ErrorServiceUnavailable("oss client not configured")
	}
	if err := s.ossClient.EnsureBucketExists(ctx, fileBucketTransfer); err != nil {
		return "", opsV1.ErrorInternalServerError("ensure bucket failed")
	}
	u, err := s.ossClient.GetClient().PresignedPutObject(ctx, fileBucketTransfer, objectKey, filePresignExpiry)
	if err != nil {
		return "", opsV1.ErrorInternalServerError("presign put failed")
	}
	return u.String(), nil
}

func (s *FileService) presignGet(ctx context.Context, objectKey string) (string, error) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return "", opsV1.ErrorServiceUnavailable("oss client not configured")
	}
	u, err := s.ossClient.GetClient().PresignedGetObject(ctx, fileBucketTransfer, objectKey, filePresignExpiry, nil)
	if err != nil {
		return "", opsV1.ErrorInternalServerError("presign get failed")
	}
	return u.String(), nil
}

// pullResult agent 传输回执(size/sha256)。
func decodeTransferResult(data json.RawMessage) (int64, string, error) {
	var out struct {
		Size   int64  `json:"size"`
		Sha256 string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, "", err
	}
	return out.Size, out.Sha256, nil
}

// PullAgentFile 拉取大文件:agent 上传对象存储 → 返回浏览器下载 URL。
func (s *FileService) PullAgentFile(ctx context.Context, req *opsV1.PullAgentFileRequest) (*opsV1.PullAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" {
		return nil, opsV1.ErrorBadRequest("path required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	objectKey := fileTransferObjectKey(agentId, req.GetPath())
	uploadURL, err := s.presignPut(ctx, objectKey)
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFilePull,
		&signal.FilePullPayload{Path: req.GetPath(), Url: uploadURL})
	if err != nil {
		return nil, err
	}
	size, sha256hex, err := decodeTransferResult(res.Data)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("decode transfer result: %s", err.Error())
	}

	downloadURL, err := s.presignGet(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	s.log.Infof("agent file pulled (agent=%s path=%s size=%d)", agentId, req.GetPath(), size)
	return &opsV1.PullAgentFileResponse{Url: &downloadURL, Size: &size, Sha256: &sha256hex}, nil
}

// PrepareAgentFilePush 签发浏览器直传 URL。
func (s *FileService) PrepareAgentFilePush(ctx context.Context, req *opsV1.PrepareAgentFilePushRequest) (*opsV1.PrepareAgentFilePushResponse, error) {
	if req == nil || req.GetPath() == "" {
		return nil, opsV1.ErrorBadRequest("path required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}
	objectKey := fileTransferObjectKey(agentId, req.GetPath())
	uploadURL, err := s.presignPut(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	return &opsV1.PrepareAgentFilePushResponse{UploadUrl: &uploadURL, ObjectKey: &objectKey}, nil
}

// CompleteAgentFilePush 指令 agent 从对象存储下载落盘。
func (s *FileService) CompleteAgentFilePush(ctx context.Context, req *opsV1.CompleteAgentFilePushRequest) (*opsV1.CompleteAgentFilePushResponse, error) {
	if req == nil || req.GetPath() == "" || req.GetObjectKey() == "" {
		return nil, opsV1.ErrorBadRequest("path/objectKey required")
	}
	// 防越权:对象键必须落在传输前缀下
	if !strings.HasPrefix(req.GetObjectKey(), fileObjectPrefix+"/") {
		return nil, opsV1.ErrorBadRequest("invalid objectKey")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	downloadURL, err := s.presignGet(ctx, req.GetObjectKey())
	if err != nil {
		return nil, err
	}
	res, err := s.callSync(ctx, agentId, signal.CommandActionFilePush,
		&signal.FilePushPayload{Path: req.GetPath(), Url: downloadURL})
	if err != nil {
		return nil, err
	}
	size, sha256hex, err := decodeTransferResult(res.Data)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("decode transfer result: %s", err.Error())
	}
	s.log.Infof("agent file pushed (agent=%s path=%s size=%d)", agentId, req.GetPath(), size)
	return &opsV1.CompleteAgentFilePushResponse{Size: &size, Sha256: &sha256hex}, nil
}

func (s *FileService) ListAgentFiles(ctx context.Context, req *opsV1.ListAgentFilesRequest) (*opsV1.ListAgentFilesResponse, error) {
	if req == nil || req.GetAgentId() == "" {
		return nil, opsV1.ErrorBadRequest("agentId required")
	}
	// dir 为空 = agent 进程工作目录(agent 端解析,响应 dir 回带实际路径)
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileList, map[string]string{"dir": req.GetDir()})
	if err != nil {
		return nil, err
	}

	var out struct {
		Dir     string `json:"dir"`
		Entries []struct {
			Name  string `json:"name"`
			Size  int64  `json:"size"`
			IsDir bool   `json:"is_dir"`
		} `json:"entries"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode file list: %s", err.Error())
	}

	resp := &opsV1.ListAgentFilesResponse{Dir: &out.Dir}
	for _, e := range out.Entries {
		name, size, isDir := e.Name, e.Size, e.IsDir
		resp.Entries = append(resp.Entries, &opsV1.AgentFileEntry{
			Name:  &name,
			Size:  &size,
			IsDir: &isDir,
		})
	}
	return resp, nil
}

func (s *FileService) ReadAgentFile(ctx context.Context, req *opsV1.ReadAgentFileRequest) (*opsV1.ReadAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" {
		return nil, opsV1.ErrorBadRequest("path required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileRead, map[string]string{"path": req.GetPath()})
	if err != nil {
		return nil, err
	}

	var out struct {
		Path          string `json:"path"`
		Size          int64  `json:"size"`
		Truncated     bool   `json:"truncated"`
		ContentBase64 string `json:"content_base64"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode file content: %s", err.Error())
	}

	return &opsV1.ReadAgentFileResponse{
		Path:          &out.Path,
		Size:          &out.Size,
		Truncated:     &out.Truncated,
		ContentBase64: &out.ContentBase64,
	}, nil
}

func (s *FileService) WriteAgentFile(ctx context.Context, req *opsV1.WriteAgentFileRequest) (*opsV1.WriteAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" || req.GetContentBase64() == "" {
		return nil, opsV1.ErrorBadRequest("path/contentBase64 required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileWrite, map[string]string{
		"path":    req.GetPath(),
		"content": req.GetContentBase64(),
	})
	if err != nil {
		return nil, err
	}

	var out struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode write result: %s", err.Error())
	}

	return &opsV1.WriteAgentFileResponse{Path: &out.Path, Size: &out.Size}, nil
}

func (s *FileService) DeleteAgentFile(ctx context.Context, req *opsV1.DeleteAgentFileRequest) (*opsV1.DeleteAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" {
		return nil, opsV1.ErrorBadRequest("path required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileDelete, map[string]string{"path": req.GetPath()})
	if err != nil {
		return nil, err
	}

	var out struct {
		Deleted string `json:"deleted"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode delete result: %s", err.Error())
	}

	return &opsV1.DeleteAgentFileResponse{Deleted: &out.Deleted}, nil
}
