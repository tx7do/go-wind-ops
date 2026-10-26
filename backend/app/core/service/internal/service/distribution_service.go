package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/distributionrecord"
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
	if req.Data.GetName() == "" || req.Data.GetBuildId() == 0 || len(req.Data.GetProjectIds()) == 0 {
		return nil, opsV1.ErrorBadRequest("name/buildId/projectIds are required")
	}

	for _, pid := range req.Data.GetProjectIds() {
		pid := pid
		if _, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: &pid}); err != nil {
			return nil, opsV1.ErrorBadRequest("project not found")
		}
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

// distRolloutContext 一次分发执行(触发或批次推进)的公共上下文:
// 产物/镜像与指令载荷的派生参数。
type distRolloutContext struct {
	dist        *opsV1.Distribution
	history     *opsV1.BuildHistory
	isDocker    bool
	downloadUrl string
	afterOpt    string
	operatorID  uint32
}

// buildRolloutContext 组装执行上下文:FILE 部署在此签发新的产物下载 URL
// (批次推进时旧 URL 可能临近过期,必须重签)。
func (s *DistributionService) buildRolloutContext(ctx context.Context, dist *opsV1.Distribution, history *opsV1.BuildHistory, operatorID uint32) (*distRolloutContext, error) {
	isDocker := dist.GetDeployType() == opsV1.Distribution_DOCKER
	if isDocker {
		if dist.GetImage() == "" || dist.GetContainerName() == "" {
			return nil, opsV1.ErrorBadRequest("docker deployment requires image and containerName")
		}
		if history.GetStatus() != opsV1.BuildHistory_SUCCESS {
			return nil, opsV1.ErrorBadRequest("build history is not successful")
		}
	} else {
		if history.GetStatus() != opsV1.BuildHistory_SUCCESS || history.GetArtifactObjectKey() == "" {
			return nil, opsV1.ErrorBadRequest("build history has no successful artifact")
		}
		if s.ossClient == nil || s.ossClient.GetClient() == nil {
			return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
		}
		urlObj, err := s.ossClient.GetClient().PresignedGetObject(ctx, history.GetArtifactBucket(), history.GetArtifactObjectKey(), artifactUrlExpiry, nil)
		if err != nil {
			s.log.Errorf("presign artifact url (history=%d): %s", history.GetId(), err.Error())
			return nil, opsV1.ErrorInternalServerError("presign url failed")
		}
		return &distRolloutContext{dist: dist, history: history, isDocker: isDocker, downloadUrl: urlObj.String(), operatorID: operatorID}, nil
	}
	return &distRolloutContext{dist: dist, history: history, isDocker: isDocker, operatorID: operatorID}, nil
}

// rolloutAction 组装单目标指令(部署类型分支)。
func (s *DistributionService) rolloutAction(rc *distRolloutContext, project *opsV1.Project) (signal.CommandAction, any) {
	if rc.isDocker {
		// 镜像 tag 支持 {buildNo} 占位符,替换为本次构建号
		imageTag := strings.ReplaceAll(rc.dist.GetImage(), "{buildNo}", fmt.Sprintf("%d", rc.history.GetBuildNo()))
		return signal.CommandActionDockerDeploy, &signal.DockerDeployPayload{
			DistributionId: rc.dist.GetId(),
			ProjectId:      project.GetId(),
			Image:          imageTag,
			ContainerName:  rc.dist.GetContainerName(),
			Ports:          rc.dist.GetDockerPorts(),
			Envs:           rc.dist.GetDockerEnvs(),
			Volumes:        rc.dist.GetDockerVolumes(),
			Restart:        rc.dist.GetDockerRestart(),
			Network:        rc.dist.GetDockerNetwork(),
		}
	}
	return signal.CommandActionDistApply, &signal.DistributionPayload{
		DistributionId: rc.dist.GetId(),
		ProjectId:      project.GetId(),
		ProjectName:    project.GetName(),
		ProjectDir:     project.GetProjectDir(),
		ClearOld:       rc.dist.GetClearOld(),
		DownloadUrl:    rc.downloadUrl,
		FileName:       artifactFileName(rc.history.GetArtifactObjectKey()),
		AfterOpt:       rc.afterOpt,
		StartCommand:   project.GetStartCommand(),
		StopCommand:    project.GetStopCommand(),
		StatusCommand:  project.GetStatusCommand(),
	}
}

// dispatchRecords 投递一批占位记录:逐条"占位键 → 真实 UUID"抢占领取后
// 下发指令;领取失败(并发他副本已领)跳过,发布失败把记录直接定案 FAILED。
// 返回成功投递的记录数与首条真实指令幂等键。
func (s *DistributionService) dispatchRecords(ctx context.Context, rc *distRolloutContext, records []*ent.DistributionRecord) (int, string) {
	dispatched := 0
	firstRequestID := ""
	for _, rec := range records {
		if !data.IsPendingRequestID(rec.CommandRequestID) {
			continue // 已投递过的记录不重复下发
		}
		project, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: &rec.ProjectID})
		if err != nil {
			s.log.Warnf("distribution [%d] batch target project [%d] not found", rc.dist.GetId(), rec.ProjectID)
			_ = s.recordRepo.FailPending(ctx, rec.ID, "target project not found")
			continue
		}
		nodeID := project.GetNodeId()
		node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
		if err != nil {
			s.log.Warnf("distribution [%d] batch target project [%d] node not found", rc.dist.GetId(), rec.ProjectID)
			_ = s.recordRepo.FailPending(ctx, rec.ID, "target node not found")
			continue
		}

		action, payload := s.rolloutAction(rc, project)
		requestId, err := s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(), action, payload, rc.operatorID)
		if err != nil {
			s.log.Errorf("dispatch distribution failed (dist=%d project=%d): %s", rc.dist.GetId(), rec.ProjectID, err.Error())
			_ = s.recordRepo.FailPending(ctx, rec.ID, "dispatch failed: "+err.Error())
			continue
		}
		ok, err := s.recordRepo.ClaimForDispatch(ctx, rec.ID, rec.CommandRequestID, requestId)
		if err != nil || !ok {
			s.log.Warnf("distribution [%d] record [%d] claim lost (concurrent advance?), skip dispatch", rc.dist.GetId(), rec.ID)
			continue
		}
		if firstRequestID == "" {
			firstRequestID = requestId
		}
		dispatched++
	}
	return dispatched, firstRequestID
}

// TriggerDistribution 触发分发:解析目标 → 取产物 → 全部目标先建记录
// (分批时后批为占位记录,批次视图完整)→ 投递首批,后续批次由回执事件
// 驱动推进(AdvanceBatches):前一批全部成功才投下一批,任一失败自动中止
// 并把未执行批次置 SKIPPED。
func (s *DistributionService) TriggerDistribution(ctx context.Context, req *opsV1.TriggerDistributionRequest) (*opsV1.TriggerDistributionResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}

	dist, err := s.repo.Get(ctx, &opsV1.GetDistributionRequest{Id: &req.Id})
	if err != nil {
		return nil, err
	}
	if len(dist.GetProjectIds()) == 0 {
		return nil, opsV1.ErrorBadRequest("distribution has no target projects")
	}

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
	rc, err := s.buildRolloutContext(ctx, dist, history, operatorIDOf(ctx))
	if err != nil {
		return nil, err
	}
	rc.afterOpt = signal.AfterNone
	switch dist.GetAfterOpt() {
	case opsV1.Distribution_RESTART:
		rc.afterOpt = signal.AfterRestart
	case opsV1.Distribution_START:
		rc.afterOpt = signal.AfterStart
	}

	// 上一轮触发遗留的未投递占位记录直接置 SKIPPED(重新触发即取代旧 rollout)
	if skipped, err := s.recordRepo.SkipAllPendingByDistribution(ctx, dist.GetId(), "superseded by a new trigger"); err != nil {
		s.log.Warnf("distribution [%d] skip stale placeholder records: %s", dist.GetId(), err.Error())
	} else if skipped > 0 {
		s.log.Infof("distribution [%d] superseded %d stale placeholder record(s)", dist.GetId(), skipped)
	}

	// 解析目标(project → node),解析失败的照旧剔除
	type distTarget struct {
		project *opsV1.Project
		node    *opsV1.AgentNode
	}
	targets := make([]distTarget, 0, len(dist.GetProjectIds()))
	for _, pid := range dist.GetProjectIds() {
		pid := pid
		project, err := s.projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: &pid})
		if err != nil {
			s.log.Warnf("distribution target project [%d] not found, skip", pid)
			continue
		}
		nodeID := project.GetNodeId()
		node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
		if err != nil {
			s.log.Warnf("distribution target project [%d] node not found, skip", pid)
			continue
		}
		targets = append(targets, distTarget{project: project, node: node})
	}
	if len(targets) == 0 {
		return nil, opsV1.ErrorServiceUnavailable("no distribution target dispatched successfully")
	}

	batchSize := int(dist.GetBatchSize())
	if batchSize <= 0 || batchSize >= len(targets) {
		batchSize = len(targets)
	}

	// 全部目标先建记录(占位幂等键,后批批次号>1),首批记录随后立即领取投递
	firstRecordID := uint32(0)
	var recordIDs []uint32
	batch1 := make([]*ent.DistributionRecord, 0, batchSize)
	for i, t := range targets {
		batchNo := uint32(i/batchSize + 1)
		placeholder := data.MakePendingRequestID(dist.GetId(), batchNo, t.project.GetId())
		record, err := s.recordRepo.Create(ctx, dist.GetId(), historyID, t.project.GetId(), t.node.GetId(), placeholder, rc.operatorID, batchNo)
		if err != nil {
			continue
		}
		recordIDs = append(recordIDs, record.ID)
		if firstRecordID == 0 {
			firstRecordID = record.ID
		}
		if batchNo == 1 {
			batch1 = append(batch1, record)
		}
	}
	if len(recordIDs) == 0 {
		return nil, opsV1.ErrorServiceUnavailable("no distribution target dispatched successfully")
	}

	dispatched, firstRequestID := s.dispatchRecords(ctx, rc, batch1)
	if dispatched == 0 {
		return nil, opsV1.ErrorServiceUnavailable("no distribution target dispatched successfully")
	}

	s.log.Infof("distribution triggered (dist=%d history=%d targets=%d batchSize=%d batch1Dispatched=%d)",
		dist.GetId(), historyID, len(targets), batchSize, dispatched)
	return &opsV1.TriggerDistributionResponse{
		RecordId:         firstRecordID,
		CommandRequestId: firstRequestID,
		RecordIds:        recordIDs,
	}, nil
}

// AdvanceBatches 分批推进引擎(事件驱动:分发记录回执终态/超时定案后调用):
// 找到最早未完成批次——在投则等待;有失败则中止并把后续批次占位记录置
// SKIPPED;全部成功则投下一批。多副本并发调用由记录领取幂等兜底。
func (s *DistributionService) AdvanceBatches(ctx context.Context, distributionID uint32) {
	dist, err := s.repo.Get(ctx, &opsV1.GetDistributionRequest{Id: &distributionID})
	if err != nil {
		return
	}
	batchSize := int(dist.GetBatchSize())
	if batchSize <= 0 {
		return
	}
	records, err := s.recordRepo.ListByDistribution(ctx, distributionID)
	if err != nil || len(records) == 0 {
		return
	}
	// 租户归因:推进补发的指令归属分发记录的租户——钩子运行在系统视图上下文
	// (MQTT 回执/超时清扫驱动),不覆盖会让补发指令落 tenant 0。
	// 同一分发的记录同租户(单次触发创建),取首条即可。
	ctx = tenantScopedCtx(ctx, records[0].TenantID)
	if batchSize >= len(records) {
		return
	}

	batches := map[uint32][]*ent.DistributionRecord{}
	maxBatch := uint32(1)
	for _, rec := range records {
		batches[rec.BatchNo] = append(batches[rec.BatchNo], rec)
		if rec.BatchNo > maxBatch {
			maxBatch = rec.BatchNo
		}
	}

	for batchNo := uint32(1); batchNo <= maxBatch; batchNo++ {
		recs := batches[batchNo]
		inFlight, failed := 0, false
		for _, rec := range recs {
			switch {
			case rec.Status == nil || *rec.Status == distributionrecord.StatusPending:
				if !data.IsPendingRequestID(rec.CommandRequestID) {
					inFlight++
				}
			case *rec.Status == distributionrecord.StatusFailed:
				failed = true
			}
		}
		if inFlight > 0 {
			return // 当前批仍在执行,等待下一次事件
		}

		if failed {
			// 中止:本批之后的全部占位记录置 SKIPPED
			skipRemainingBatches(ctx, s.recordRepo, batches, batchNo+1,
				fmt.Sprintf("batch %d failed, rollout aborted", batchNo))
			return
		}

		// 本批全部成功(或遗留待投):投下一批
		next := batches[batchNo+1]
		if len(next) == 0 {
			return
		}
		// 执行上下文重建:URL 重签,镜像 tag 按该 rollout 的构建号重算
		historyID := next[0].BuildHistoryID
		history, err := s.historyRepo.Get(ctx, &opsV1.GetBuildHistoryRequest{Id: &historyID})
		if err != nil {
			s.log.Warnf("distribution [%d] advance: history [%d] not found, stop", distributionID, historyID)
			return
		}
		rc, err := s.buildRolloutContext(ctx, dist, history, 0)
		if err != nil {
			s.log.Warnf("distribution [%d] advance: build rollout context: %s", distributionID, err.Error())
			return
		}
		rc.afterOpt = signal.AfterNone
		switch dist.GetAfterOpt() {
		case opsV1.Distribution_RESTART:
			rc.afterOpt = signal.AfterRestart
		case opsV1.Distribution_START:
			rc.afterOpt = signal.AfterStart
		}
		dispatched, _ := s.dispatchRecords(ctx, rc, next)
		if dispatched == 0 {
			// 本批投递全部失败(已逐条定案 FAILED):继续循环,由失败分支中止后续批次
			continue
		}
		return // 已投递(在途)或已是最后一批
	}
}

// skipRemainingBatches 把 batchFrom 起的全部占位记录置 SKIPPED。
func skipRemainingBatches(ctx context.Context, recordRepo *data.DistributionRecordRepo, batches map[uint32][]*ent.DistributionRecord, batchFrom uint32, reason string) {
	skipped := 0
	for b := batchFrom; ; b++ {
		recs, ok := batches[b]
		if !ok {
			break
		}
		for _, rec := range recs {
			if yes, _ := recordRepo.SkipPending(ctx, rec.ID, reason); yes {
				skipped++
			}
		}
	}
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
