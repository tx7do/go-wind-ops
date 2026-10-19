// tier3_service.go 第三梯队三服务:巡检报告(聚合各域健康快照)、日志快照
// (项目日志尾部采集+关键字检索)、配置文件中心(模板 CRUD+下发记录)。
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/go-utils/id"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"github.com/minio/minio-go/v7"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ═══════════════════════════ 巡检报告 ═══════════════════════════

// InspectionService 平台巡检:聚合节点/项目/告警/证书/备份当前状态。
type InspectionService struct {
	opsV1.UnimplementedInspectionServiceServer

	log           *log.Helper
	inspectionRepo *data.InspectionRepo
	nodeRepo       *data.AgentNodeRepo
	projectRepo    *data.ProjectRepo
	alertRepo      *data.AlertRepo
	certRepo       *data.CertRepo
	backupRepo     *data.BackupRecordRepo
}

func NewInspectionService(ctx *bootstrap.Context, inspectionRepo *data.InspectionRepo, nodeRepo *data.AgentNodeRepo, projectRepo *data.ProjectRepo, alertRepo *data.AlertRepo, certRepo *data.CertRepo, backupRepo *data.BackupRecordRepo) *InspectionService {
	return &InspectionService{
		log:            ctx.NewLoggerHelper("inspection/service/core-service"),
		inspectionRepo: inspectionRepo,
		nodeRepo:       nodeRepo,
		projectRepo:    projectRepo,
		alertRepo:      alertRepo,
		certRepo:       certRepo,
		backupRepo:     backupRepo,
	}
}

func (s *InspectionService) ListInspections(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListInspectionResponse, error) {
	return s.inspectionRepo.List(ctx, req)
}

func (s *InspectionService) GetInspection(ctx context.Context, req *opsV1.GetInspectionRequest) (*opsV1.Inspection, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	row, err := s.inspectionRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("inspection not found")
	}
	return inspectionToDTO(row), nil
}

// RunInspection 发起巡检:建行后异步聚合(脱离请求生命周期),前端轮询列表。
func (s *InspectionService) RunInspection(ctx context.Context, req *opsV1.RunInspectionRequest) (*opsV1.Inspection, error) {
	row, err := s.inspectionRepo.Create(ctx, operatorIDOf(ctx))
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("create inspection failed")
	}
	go s.execute(context.Background(), row.ID)
	return inspectionToDTO(mustInspection(s, ctx, row.ID)), nil
}

func mustInspection(s *InspectionService, ctx context.Context, id uint32) *ent.Inspection {
	row, _ := s.inspectionRepo.GetEntity(ctx, id)
	return row
}

// inspectionReport 巡检报告 JSON 结构。
type inspectionReport struct {
	GeneratedAt string              `json:"generatedAt"`
	Nodes       []inspectionNode    `json:"nodes"`
	Projects    []inspectionProject `json:"projects"`
	Alerts      struct {
		Open int `json:"open"`
	} `json:"alerts"`
	CertExpiring []data.CertExpiringSoonRow `json:"certExpiring"`
}

type inspectionNode struct {
	Name    string  `json:"name"`
	AgentID string  `json:"agentId"`
	Status  string  `json:"status"`
	Env     string  `json:"env"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	Disk    float64 `json:"disk"`
	Version string  `json:"version"`
}

type inspectionProject struct {
	Name   string `json:"name"`
	NodeID uint32 `json:"nodeId"`
	Status string `json:"status"`
}

// execute 聚合各域数据并终态化(失败置 FAILED)。
func (s *InspectionService) execute(ctx context.Context, rowID uint32) {
	defer func() {
		if r := recover(); r != nil {
			_ = s.inspectionRepo.Finish(ctx, rowID, "FAILED", "", 0, 0, 0, 0, 0, 0, fmt.Sprintf("panic: %v", r))
		}
	}()

	report := inspectionReport{GeneratedAt: time.Now().Format(time.RFC3339)}

	// 节点
	nodeTotal, nodeOnline := 0, 0
	if resp, err := s.nodeRepo.List(ctx, allPaging()); err == nil {
		for _, n := range resp.GetItems() {
			nodeTotal++
			status := n.GetStatus().String()
			if status == "ONLINE" {
				nodeOnline++
			}
			cpu, mem, disk := 0.0, 0.0, 0.0
			if n.CpuPercent != nil {
				cpu = *n.CpuPercent
			}
			if n.MemoryUsedPercent != nil {
				mem = *n.MemoryUsedPercent
			}
			if n.DiskUsedPercent != nil {
				disk = *n.DiskUsedPercent
			}
			report.Nodes = append(report.Nodes, inspectionNode{
				Name: n.GetName(), AgentID: n.GetAgentId(), Status: status,
				Env: n.GetEnv(), CPU: cpu, Mem: mem, Disk: disk, Version: n.GetAgentVersion(),
			})
		}
	}

	// 项目
	projectTotal, projectRunning := 0, 0
	if resp, err := s.projectRepo.List(ctx, allPaging()); err == nil {
		for _, p := range resp.GetItems() {
			projectTotal++
			if p.GetLastStatus() == opsV1.ProjectStatus_RUNNING {
				projectRunning++
			}
			report.Projects = append(report.Projects, inspectionProject{
				Name: p.GetName(), NodeID: p.GetNodeId(), Status: p.GetLastStatus().String(),
			})
		}
	}

	// 未恢复告警
	alertOpen := 0
	if resp, err := s.alertRepo.List(ctx, &paginationV1.PagingRequest{NoPaging: boolPtr(true)}); err == nil {
		for _, a := range resp.GetItems() {
			if !a.GetResolved() {
				alertOpen++
			}
		}
	}
	report.Alerts.Open = alertOpen

	// 30 天内到期证书
	expiring := s.inspectionRepo.ListCertsExpiringWithin(ctx, s.certRepo, 30)
	report.CertExpiring = expiring

	summary, _ := json.Marshal(report)
	ferr := ""
	if err := s.inspectionRepo.Finish(ctx, rowID, "FINISHED", string(summary),
		nodeTotal, nodeOnline, projectTotal, projectRunning, alertOpen, len(expiring), ferr); err != nil {
		s.log.Errorf("inspection finish failed (id=%d): %s", rowID, err.Error())
	}
	s.log.Infof("inspection finished (id=%d nodes=%d/%d projects=%d/%d alerts=%d certs=%d)",
		rowID, nodeOnline, nodeTotal, projectRunning, projectTotal, alertOpen, len(expiring))
}

func inspectionToDTO(row *ent.Inspection) *opsV1.Inspection {
	if row == nil {
		return nil
	}
	return &opsV1.Inspection{
		Id:             &row.ID,
		Status:         strPtr(string(row.Status)),
		Summary:        row.Summary,
		NodeTotal:      transPtrInt32(int32(row.NodeTotal)),
		NodeOnline:     transPtrInt32(int32(row.NodeOnline)),
		ProjectTotal:   transPtrInt32(int32(row.ProjectTotal)),
		ProjectRunning: transPtrInt32(int32(row.ProjectRunning)),
		AlertOpen:      transPtrInt32(int32(row.AlertOpen)),
		CertExpiring:   transPtrInt32(int32(row.CertExpiring)),
		Error:          row.Error,
		FinishedAt:     timestamppbOfPtr(row.FinishedAt),
		CreatedAt:      timestamppbOfPtr(row.CreatedAt),
	}
}

// ═══════════════════════════ 日志快照 ═══════════════════════════

// LogSnapshotService 项目日志快照:采集 + 检索。
type LogSnapshotService struct {
	opsV1.UnimplementedLogSnapshotServiceServer

	log        *log.Helper
	snapshotRepo *data.LogSnapshotRepo
	projectRepo  *data.ProjectRepo
	nodeRepo     *data.AgentNodeRepo
	agentSvc     *AgentService
}

func NewLogSnapshotService(ctx *bootstrap.Context, snapshotRepo *data.LogSnapshotRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, agentSvc *AgentService) *LogSnapshotService {
	return &LogSnapshotService{
		log:          ctx.NewLoggerHelper("log-snapshot/service/core-service"),
		snapshotRepo: snapshotRepo,
		projectRepo:  projectRepo,
		nodeRepo:     nodeRepo,
		agentSvc:     agentSvc,
	}
}

func (s *LogSnapshotService) ListLogSnapshots(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListLogSnapshotResponse, error) {
	return s.snapshotRepo.List(ctx, req)
}

func (s *LogSnapshotService) GetLogSnapshot(ctx context.Context, req *opsV1.GetLogSnapshotRequest) (*opsV1.LogSnapshot, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	row, err := s.snapshotRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("log snapshot not found")
	}
	return logSnapshotToDTO(row), nil
}

// CaptureLogSnapshot 采集一次项目日志尾部(256KB)落库。
func (s *LogSnapshotService) CaptureLogSnapshot(ctx context.Context, req *opsV1.CaptureLogSnapshotRequest) (*opsV1.LogSnapshot, error) {
	if req == nil || req.GetProjectId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: projectId required")
	}
	projectID := req.GetProjectId()
	project, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: &projectID})
	if err != nil {
		return nil, err
	}
	nodeID := project.GetNodeId()
	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		return nil, opsV1.ErrorBadRequest("project node not found")
	}

	res, err := s.agentSvc.PublishCommandSync(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionProjectLog,
		&signal.ProjectLogPayload{ProjectId: project.GetId(), Dir: project.GetProjectDir(), MaxBytes: 256 << 10},
		operatorIDOf(ctx), logQueryTimeout)
	if err != nil {
		return nil, opsV1.ErrorServiceUnavailable("capture log failed: " + err.Error())
	}
	if !res.Success {
		return nil, opsV1.ErrorInternalServerError("capture log: " + res.Error)
	}
	var out signal.ProjectLogResult
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode project log result")
	}
	content := logContentToUTF8(out)

	row, err := s.snapshotRepo.Create(ctx, project.GetId(), nodeID, project.GetName(),
		content, "MANUAL", operatorIDOf(ctx))
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("save log snapshot failed")
	}
	s.log.Infof("log snapshot captured (project=%d size=%d)", project.GetId(), out.Size)
	return logSnapshotToDTO(row), nil
}

// CaptureAll 定时采集入口:全部项目逐个采集(trigger=SCHEDULED),单个
// 失败仅记日志。串行下发,依赖 30 分钟窗口上限。
func (s *LogSnapshotService) CaptureAll(ctx context.Context) error {
	resp, err := s.projectRepo.List(ctx, &paginationV1.PagingRequest{NoPaging: boolPtr(true)})
	if err != nil {
		return err
	}
	okCount, failCount := 0, 0
	for _, p := range resp.GetItems() {
		projectID := p.GetId()
		nodeID := p.GetNodeId()
		node, nerr := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
		if nerr != nil {
			failCount++
			continue
		}
		res, rerr := s.agentSvc.PublishCommandSync(ctx, node.GetId(), node.GetAgentId(),
			signal.CommandActionProjectLog,
			&signal.ProjectLogPayload{ProjectId: projectID, Dir: p.GetProjectDir(), MaxBytes: 256 << 10},
			0, logQueryTimeout)
		if rerr != nil || !res.Success {
			failCount++
			continue
		}
		var out signal.ProjectLogResult
		if json.Unmarshal(res.Data, &out) != nil {
			failCount++
			continue
		}
		if _, cerr := s.snapshotRepo.Create(ctx, projectID, nodeID, p.GetName(),
			logContentToUTF8(out), "SCHEDULED", 0); cerr != nil {
			failCount++
			continue
		}
		okCount++
	}
	s.log.Infof("scheduled log snapshot done (ok=%d fail=%d)", okCount, failCount)
	return nil
}

// logContentToUTF8 快照落库内容:优先取 ContentB64 原始字节,UTF-8 校验
// 失败(Windows 控制台 GBK 输出)按 GBK 解码转 UTF-8,避免有损字符串入库。
func logContentToUTF8(out signal.ProjectLogResult) string {
	if out.ContentB64 == "" {
		return out.Content
	}
	raw, err := base64.StdEncoding.DecodeString(out.ContentB64)
	if err != nil {
		return out.Content
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw); err == nil {
		return string(decoded)
	}
	return out.Content
}

// SearchLogSnapshots 关键字检索:ILIKE 命中最近 50 份快照,每份生成命中
// 摘要(首处命中行 ± 上下文,截断 400 字符)。
func (s *LogSnapshotService) SearchLogSnapshots(ctx context.Context, req *opsV1.SearchLogSnapshotsRequest) (*opsV1.SearchLogSnapshotResponse, error) {
	if req == nil || strings.TrimSpace(req.GetKeyword()) == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: keyword required")
	}
	keyword := strings.TrimSpace(req.GetKeyword())
	rows, err := s.snapshotRepo.SearchByKeyword(ctx, keyword, req.GetProjectId(), 50)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("search log snapshots failed")
	}

	resp := &opsV1.SearchLogSnapshotResponse{Keyword: &keyword}
	for _, row := range rows {
		hit := &opsV1.SearchLogSnapshotResponse_SearchHit{
			SnapshotId:  &row.ID,
			ProjectId:   uint32Ptr(row.ProjectID),
			ProjectName: &row.ProjectName,
			CapturedAt:  timestamppbOfPtr(row.CreatedAt),
			Excerpt:     strPtr(logExcerpt(row.Content, keyword)),
			MatchCount:  transPtrInt32(int32(strings.Count(row.Content, keyword))),
		}
		resp.Hits = append(resp.Hits, hit)
	}
	return resp, nil
}

// logExcerpt 生成命中摘要:首个命中行及其前后各 1 行,超长截断。
func logExcerpt(content, keyword string) string {
	lines := strings.Split(content, "\n")
	first := -1
	for i, line := range lines {
		if strings.Contains(line, keyword) {
			first = i
			break
		}
	}
	if first < 0 {
		if len(content) > 400 {
			return content[:400] + "..."
		}
		return content
	}
	lo := first - 1
	if lo < 0 {
		lo = 0
	}
	hi := first + 2
	if hi > len(lines) {
		hi = len(lines)
	}
	excerpt := strings.Join(lines[lo:hi], "\n")
	if len(excerpt) > 400 {
		excerpt = excerpt[:400] + "..."
	}
	return excerpt
}

func (s *LogSnapshotService) DeleteLogSnapshot(ctx context.Context, req *opsV1.DeleteLogSnapshotRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	if err := s.snapshotRepo.DeleteEntity(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}

func logSnapshotToDTO(row *ent.LogSnapshot) *opsV1.LogSnapshot {
	if row == nil {
		return nil
	}
	return &opsV1.LogSnapshot{
		Id:          &row.ID,
		ProjectId:   uint32Ptr(row.ProjectID),
		NodeId:      uint32Ptr(row.NodeID),
		ProjectName: &row.ProjectName,
		SizeBytes:   &row.SizeBytes,
		Content:     &row.Content,
		Trigger:     strPtr(row.Trigger),
		CreatedAt:   timestamppbOfPtr(row.CreatedAt),
	}
}

// ═══════════════════════════ 配置文件中心 ═══════════════════════════

const (
	// configObjectPrefix 配置下发对象键前缀(内容经 MinIO 中转)。
	configObjectPrefix = "config-templates"
	// configTransferSync 配置下发回执等待上限(与文件传输一致)。
	configTransferSync = 10 * time.Minute
)

// ConfigService 配置文件中心:模板 CRUD + 下发(file_push 复用,回执终态)。
type ConfigService struct {
	opsV1.UnimplementedConfigServiceServer

	log           *log.Helper
	templateRepo  *data.ConfigTemplateRepo
	distRepo      *data.ConfigDistributionRepo
	nodeRepo      *data.AgentNodeRepo
	projectRepo   *data.ProjectRepo
	ossClient     *oss.MinIOClient
	agentSvc      *AgentService
}

func NewConfigService(ctx *bootstrap.Context, templateRepo *data.ConfigTemplateRepo, distRepo *data.ConfigDistributionRepo, nodeRepo *data.AgentNodeRepo, projectRepo *data.ProjectRepo, ossClient *oss.MinIOClient, agentSvc *AgentService) *ConfigService {
	return &ConfigService{
		log:          ctx.NewLoggerHelper("config/service/core-service"),
		templateRepo: templateRepo,
		distRepo:     distRepo,
		nodeRepo:     nodeRepo,
		projectRepo:  projectRepo,
		ossClient:    ossClient,
		agentSvc:     agentSvc,
	}
}

func (s *ConfigService) ListConfigTemplates(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListConfigTemplateResponse, error) {
	return s.templateRepo.List(ctx, req)
}

func (s *ConfigService) GetConfigTemplate(ctx context.Context, req *opsV1.GetConfigTemplateRequest) (*opsV1.ConfigTemplate, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	row, err := s.templateRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("config template not found")
	}
	return configTemplateToDTO(row), nil
}

func (s *ConfigService) CreateConfigTemplate(ctx context.Context, req *opsV1.CreateConfigTemplateRequest) (*opsV1.ConfigTemplate, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetTargetPath() == "" {
		return nil, opsV1.ErrorBadRequest("name/targetPath are required")
	}
	if len(req.Data.GetContent()) > signal.ScriptContentLimit {
		return nil, opsV1.ErrorBadRequest("content exceeds limit (256KB)")
	}
	if req.Data.GetEnv() == "" {
		req.Data.Env = strPtr("default")
	}
	taken, err := s.templateRepo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("config template name already exists")
	}
	return s.templateRepo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *ConfigService) UpdateConfigTemplate(ctx context.Context, req *opsV1.UpdateConfigTemplateRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.Content != nil && len(req.Data.GetContent()) > signal.ScriptContentLimit {
		return nil, opsV1.ErrorBadRequest("content exceeds limit (256KB)")
	}
	if req.Data.Name != nil {
		taken, err := s.templateRepo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("config template name already exists")
		}
	}
	if err := s.templateRepo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ConfigService) DeleteConfigTemplate(ctx context.Context, req *opsV1.DeleteConfigTemplateRequest) (*emptypb.Empty, error) {
	if err := s.templateRepo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// DistributeConfig 下发模板:内容直传 MinIO → 预签名 GET → file_push 同步
// 指令(等待回执)→ 建 PENDING 记录,终态由回执对账钩子驱动。
func (s *ConfigService) DistributeConfig(ctx context.Context, req *opsV1.DistributeConfigRequest) (*opsV1.ConfigDistribution, error) {
	if req == nil || req.GetTemplateId() == 0 || req.GetNodeId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: templateId/nodeId required")
	}
	tpl, err := s.templateRepo.GetEntity(ctx, req.GetTemplateId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("config template not found")
	}
	nodeID := req.GetNodeId()
	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		return nil, opsV1.ErrorBadRequest("target node not found")
	}
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
	}
	if err = s.ossClient.EnsureBucketExists(ctx, oss.BucketFiles); err != nil {
		return nil, opsV1.ErrorInternalServerError("ensure bucket failed")
	}

	content := s.renderTemplate(ctx, tpl, req.GetProjectId(), node)

	objectKey := fmt.Sprintf("%s/%s.conf", configObjectPrefix, id.NewGUIDv4(false))
	if _, err = s.ossClient.GetClient().PutObject(ctx, oss.BucketFiles, objectKey,
		strings.NewReader(content), int64(len(content)), minio.PutObjectOptions{}); err != nil {
		s.log.Errorf("upload config content failed (tpl=%d): %s", tpl.ID, err.Error())
		return nil, opsV1.ErrorInternalServerError("upload config content failed")
	}
	urlObj, err := s.ossClient.GetClient().PresignedGetObject(ctx, oss.BucketFiles, objectKey, filePresignExpiry, nil)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("presign url failed")
	}

	requestId, err := s.agentSvc.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionFilePush,
		&signal.FilePushPayload{Path: tpl.TargetPath, Url: urlObj.String()},
		operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("dispatch config push failed (tpl=%d node=%d): %s", tpl.ID, nodeID, err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	row, err := s.distRepo.Create(ctx, tpl.ID, tpl.Name, nodeID, req.GetProjectId(),
		tpl.TargetPath, requestId, operatorIDOf(ctx))
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("create distribution record failed")
	}
	s.log.Infof("config distribution dispatched (tpl=%d node=%d request_id=%s)", tpl.ID, nodeID, requestId)
	return configDistributionToDTO(row), nil
}

func (s *ConfigService) ListConfigDistributions(ctx context.Context, req *opsV1.ListConfigDistributionsRequest) (*opsV1.ListConfigDistributionResponse, error) {
	if req == nil || req.GetTemplateId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: templateId required")
	}
	rows, total, err := s.distRepo.ListByTemplate(ctx, req.GetTemplateId(), int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("query distributions failed")
	}
	resp := &opsV1.ListConfigDistributionResponse{Total: uint64(total)}
	for _, row := range rows {
		resp.Items = append(resp.Items, configDistributionToDTO(row))
	}
	return resp, nil
}

// renderTemplate 下发前占位符替换:{{env}}/{{node.*}}/{{project.*}}/{{date}}/{{datetime}};
// 项目类占位符在未关联项目时替换为空串。
func (s *ConfigService) renderTemplate(ctx context.Context, tpl *ent.ConfigTemplate, projectID uint32, node *opsV1.AgentNode) string {
	content := tpl.Content
	nodeID := node.GetId()
	repl := map[string]string{
		"{{env}}":           tpl.Env,
		"{{node.id}}":       fmt.Sprintf("%d", nodeID),
		"{{node.name}}":     node.GetName(),
		"{{node.hostname}}": node.GetHostname(),
		"{{node.agentId}}":  node.GetAgentId(),
		"{{date}}":          time.Now().Format("2006-01-02"),
		"{{datetime}}":      time.Now().Format("2006-01-02 15:04:05"),
		"{{project.id}}":    "",
		"{{project.name}}":  "",
		"{{project.dir}}":   "",
	}
	if projectID > 0 {
		if p, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: &projectID}); err == nil && p != nil {
			repl["{{project.id}}"] = fmt.Sprintf("%d", p.GetId())
			repl["{{project.name}}"] = p.GetName()
			repl["{{project.dir}}"] = p.GetProjectDir()
		}
	}
	for k, v := range repl {
		content = strings.ReplaceAll(content, k, v)
	}
	return content
}

func configTemplateToDTO(row *ent.ConfigTemplate) *opsV1.ConfigTemplate {
	if row == nil {
		return nil
	}
	return &opsV1.ConfigTemplate{
		Id:          &row.ID,
		Name:        &row.Name,
		Env:         &row.Env,
		TargetPath:  &row.TargetPath,
		Content:     &row.Content,
		Description: row.Description,
		CreatedAt:   timestamppbOfPtr(row.CreatedAt),
		UpdatedAt:   timestamppbOfPtr(row.UpdatedAt),
	}
}

func configDistributionToDTO(row *ent.ConfigDistribution) *opsV1.ConfigDistribution {
	if row == nil {
		return nil
	}
	return &opsV1.ConfigDistribution{
		Id:          &row.ID,
		TemplateId:  &row.TemplateID,
		TemplateName: &row.TemplateName,
		NodeId:      &row.NodeID,
		ProjectId:   uint32Ptr(row.ProjectID),
		TargetPath:  &row.TargetPath,
		Status:      strPtr(string(row.Status)),
		Error:       row.Error,
		FinishedAt:  timestamppbOfPtr(row.FinishedAt),
		CreatedAt:   timestamppbOfPtr(row.CreatedAt),
	}
}
