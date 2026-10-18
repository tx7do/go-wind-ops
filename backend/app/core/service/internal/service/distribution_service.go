package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// DistributionService 构建产物分发:配置 CRUD + 触发。触发时取构建配置的
// 最新成功产物,签发预签名下载 URL,经 dist_apply 指令下发到目标节点;
// agent 下载解包进项目目录(白名单校验),按 afterOpt 联动项目操作。
// 执行结果由 MQTT 回执驱动回写分发记录(HandleCommandResult)。
type DistributionService struct {
	opsV1.UnimplementedDistributionServiceServer

	log        *log.Helper
	repo       *data.DistributionRepo
	recordRepo *data.DistributionRecordRepo
	buildRepo  *data.BuildRepo
	historyRepo *data.BuildHistoryRepo
	projectRepo *data.ProjectRepo
	nodeRepo    *data.AgentNodeRepo
	ossClient   *oss.MinIOClient

	agentService *AgentService
}

func NewDistributionService(ctx *bootstrap.Context, repo *data.DistributionRepo, recordRepo *data.DistributionRecordRepo, buildRepo *data.BuildRepo, historyRepo *data.BuildHistoryRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, ossClient *oss.MinIOClient, agentService *AgentService) *DistributionService {
	return &DistributionService{
		log:          ctx.NewLoggerHelper("distribution/service/core-service"),
		repo:         repo,
		recordRepo:   recordRepo,
		buildRepo:    buildRepo,
		historyRepo:  historyRepo,
		projectRepo:  projectRepo,
		nodeRepo:     nodeRepo,
		ossClient:    ossClient,
		agentService: agentService,
	}
}

func (s *DistributionService) ListDistributions(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListDistributionResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *DistributionService) GetDistribution(ctx context.Context, req *opsV1.GetDistributionRequest) (*opsV1.Distribution, error) {
	return s.repo.Get(ctx, req)
}

func (s *DistributionService) CreateDistribution(ctx context.Context, req *opsV1.CreateDistributionRequest) (*opsV1.Distribution, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetBuildId() == 0 || req.Data.GetProjectId() == 0 {
		return nil, opsV1.ErrorBadRequest("name/buildId/projectId are required")
	}

	if _, err := s.buildRepo.Get(ctx, &opsV1.GetBuildRequest{Id: req.Data.BuildId}); err != nil {
		return nil, opsV1.ErrorBadRequest("build not found")
	}
	if _, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: req.Data.ProjectId}); err != nil {
		return nil, opsV1.ErrorBadRequest("project not found")
	}

	taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("distribution name already exists")
	}

	return s.repo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *DistributionService) UpdateDistribution(ctx context.Context, req *opsV1.UpdateDistributionRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	if req.Data.Name != nil {
		taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("distribution name already exists")
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *DistributionService) DeleteDistribution(ctx context.Context, req *opsV1.DeleteDistributionRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *DistributionService) ListDistributionRecords(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListDistributionRecordResponse, error) {
	return s.recordRepo.List(ctx, req)
}

func (s *DistributionService) GetDistributionRecord(ctx context.Context, req *opsV1.GetDistributionRecordRequest) (*opsV1.DistributionRecord, error) {
	return s.recordRepo.Get(ctx, req)
}

// TriggerDistribution 触发分发:解析目标 → 取产物 → 签发下载 URL →
// dist_apply 指令下发 → 创建分发记录(回执驱动终态)。
func (s *DistributionService) TriggerDistribution(ctx context.Context, req *opsV1.TriggerDistributionRequest) (*opsV1.TriggerDistributionResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}

	dist, err := s.repo.Get(ctx, &opsV1.GetDistributionRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	project, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: dist.ProjectId})
	if err != nil {
		return nil, opsV1.ErrorBadRequest("project not found")
	}
	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: project.NodeId})
	if err != nil {
		return nil, opsV1.ErrorBadRequest("project node not found")
	}

	// 产物来源:显式指定历史,否则取该构建配置最新成功产物
	historyID := req.GetHistoryId()
	if historyID == 0 {
		historyID, err = s.latestSuccessHistoryID(ctx, dist.GetBuildId())
		if err != nil {
			return nil, err
		}
	}
	history, err := s.historyRepo.Get(ctx, &opsV1.GetBuildHistoryRequest{Id: &historyID})
	if err != nil {
		return nil, opsV1.ErrorBadRequest("build history not found")
	}
	if history.GetStatus() != opsV1.BuildHistory_SUCCESS || history.GetArtifactObjectKey() == "" {
		return nil, opsV1.ErrorBadRequest("build history has no successful artifact")
	}

	// 签发产物下载预签名 URL(1h 有效)
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
	}
	urlObj, err := s.ossClient.GetClient().PresignedGetObject(ctx, history.GetArtifactBucket(), history.GetArtifactObjectKey(), artifactUrlExpiry, nil)
	if err != nil {
		s.log.Errorf("presign artifact url (history=%d): %s", historyID, err.Error())
		return nil, opsV1.ErrorInternalServerError("presign url failed")
	}

	afterOpt := signal.AfterNone
	switch dist.GetAfterOpt() {
	case opsV1.Distribution_RESTART:
		afterOpt = signal.AfterRestart
	case opsV1.Distribution_START:
		afterOpt = signal.AfterStart
	}
	payload := &signal.DistributionPayload{
		DistributionId: dist.GetId(),
		ProjectId:      project.GetId(),
		ProjectName:    project.GetName(),
		ProjectDir:     project.GetProjectDir(),
		ClearOld:       dist.GetClearOld(),
		DownloadUrl:    urlObj.String(),
		FileName:       artifactFileName(history.GetArtifactObjectKey()),
		AfterOpt:       afterOpt,
		StartCommand:   project.GetStartCommand(),
		StopCommand:    project.GetStopCommand(),
		StatusCommand:  project.GetStatusCommand(),
	}

	nodeID := node.GetId()
	requestId, err := s.agentService.PublishCommand(ctx, nodeID, node.GetAgentId(),
		signal.CommandActionDistApply, payload, operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("dispatch distribution failed (dist=%d): %s", dist.GetId(), err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	historyIDValue := historyID
	record, err := s.recordRepo.Create(ctx, dist.GetId(), historyIDValue, project.GetId(), nodeID, requestId, operatorIDOf(ctx))
	if err != nil {
		return nil, err
	}

	s.log.Infof("distribution triggered (dist=%d history=%d project=%d record=%d request=%s)",
		dist.GetId(), historyID, project.GetId(), record.ID, requestId)
	return &opsV1.TriggerDistributionResponse{RecordId: record.ID, CommandRequestId: requestId}, nil
}

// latestSuccessHistoryID 取构建配置最新的 SUCCESS 历史;无则报错。
func (s *DistributionService) latestSuccessHistoryID(ctx context.Context, buildID uint32) (uint32, error) {
	resp, err := s.historyRepo.List(ctx, &paginationV1.PagingRequest{
		NoPaging: boolPtr(true),
	})
	if err != nil {
		return 0, err
	}
	var latestID uint32
	for _, h := range resp.GetItems() {
		if h.GetBuildId() != buildID || h.GetStatus() != opsV1.BuildHistory_SUCCESS {
			continue
		}
		if h.GetId() > latestID {
			latestID = h.GetId()
		}
	}
	if latestID == 0 {
		return 0, opsV1.ErrorBadRequest("no successful build history for this build")
	}
	return latestID, nil
}

// artifactFileName 从对象键提取产物文件名。
func artifactFileName(objectKey string) string {
	for i := len(objectKey) - 1; i >= 0; i-- {
		if objectKey[i] == '/' {
			return objectKey[i+1:]
		}
	}
	return objectKey
}

func boolPtr(v bool) *bool { return &v }
