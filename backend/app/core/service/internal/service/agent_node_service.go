package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/metadata"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// AgentNodeService 受管机器(agent 节点)管理服务:gRPC 面向 admin BFF,
// 指令下发经 MQTT 信令通道(AgentService.PublishCommand)。
type AgentNodeService struct {
	opsV1.UnimplementedAgentNodeServiceServer

	log        *log.Helper
	repo       *data.AgentNodeRepo
	metricRepo *data.NodeMetricRepo

	agentService *AgentService
	approvalRepo *data.ApprovalRepo
}

func NewAgentNodeService(ctx *bootstrap.Context, repo *data.AgentNodeRepo, metricRepo *data.NodeMetricRepo, agentService *AgentService, approvalRepo *data.ApprovalRepo) *AgentNodeService {
	return &AgentNodeService{
		log:          ctx.NewLoggerHelper("agent-node/service/core-service"),
		repo:         repo,
		metricRepo:   metricRepo,
		agentService: agentService,
		approvalRepo: approvalRepo,
	}
}

func (s *AgentNodeService) ListAgentNodes(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAgentNodeResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *AgentNodeService) GetAgentNode(ctx context.Context, req *opsV1.GetAgentNodeRequest) (*opsV1.AgentNode, error) {
	return s.repo.Get(ctx, req)
}

func (s *AgentNodeService) UpdateAgentNode(ctx context.Context, req *opsV1.UpdateAgentNodeRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	// 仅 name/labels 可更新;其余字段即使误入 mask 也由仓储忽略。
	req.Data.AgentId = nil
	req.Data.Status = nil
	req.Data.Hostname = nil

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AgentNodeService) DeleteAgentNode(ctx context.Context, req *opsV1.DeleteAgentNodeRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SendCommand 向节点下发指令。action 当前支持 ping;载荷为 JSON 字符串,
// 原样透传进 MQTT 指令信封。返回的 request_id 是执行结果回执的幂等键。
func (s *AgentNodeService) SendCommand(ctx context.Context, req *opsV1.SendCommandRequest) (*opsV1.SendCommandResponse, error) {
	if req == nil || req.GetAction() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: action required")
	}
	// action 白名单:本端点是通用指令透传面,不设白名单等价于把全部指令面
	// (脚本/文件/终端/升级...)暴露给单个"节点指令"权限。现仅开放 ping,
	// 其余能力走各自域的专用入口(经权限码门)。
	if req.GetAction() != string(signal.CommandActionPing) {
		return nil, opsV1.ErrorBadRequest("action %q not allowed (ping only)", req.GetAction())
	}
	if req.GetId() == 0 && req.GetAgentId() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id or agentId required")
	}

	agentId := req.GetAgentId()
	nodeID := uint32(0)
	if req.GetId() != 0 {
		node, err := s.repo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &req.Id})
		if err != nil {
			return nil, err
		}
		agentId = node.GetAgentId()
		nodeID = node.GetId()
	}

	var payload json.RawMessage
	if raw := req.GetPayload(); raw != "" {
		if !json.Valid([]byte(raw)) {
			return nil, opsV1.ErrorBadRequest("payload must be valid JSON")
		}
		payload = json.RawMessage(raw)
	}

	requestId, err := s.agentService.PublishCommand(ctx, nodeID, agentId, signal.CommandAction(req.GetAction()), payload, s.operatorID(ctx))
	if err != nil {
		s.log.Errorf("dispatch command failed (agent=%s action=%s): %s", agentId, req.GetAction(), err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	return &opsV1.SendCommandResponse{RequestId: requestId}, nil
}

// GetNodeMetricsHistory 查询节点心跳指标时间序列。
func (s *AgentNodeService) GetNodeMetricsHistory(ctx context.Context, req *opsV1.GetNodeMetricsHistoryRequest) (*opsV1.GetNodeMetricsHistoryResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	hours := int32(req.GetHours())
	if hours <= 0 {
		hours = 1
	}
	if hours > 168 {
		hours = 168
	}

	to := time.Now()
	from := to.Add(-time.Duration(hours) * time.Hour)

	// 先解析 agentId(指标表按 node_id 索引,这里 id 即 node_id)
	points, err := s.metricRepo.ListByNodeAndTimeRange(ctx, req.GetId(), from, to)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("query metrics failed")
	}

	resp := &opsV1.GetNodeMetricsHistoryResponse{}
	for _, p := range points {
		resp.Points = append(resp.GetPoints(), &opsV1.GetNodeMetricsHistoryResponse_MetricPoint{
			Time:              p.Time.Format(time.RFC3339),
			CpuPercent:        p.CPUPercent,
			MemoryUsedPercent: p.MemoryUsedPercent,
			DiskUsedPercent:   p.DiskUsedPercent,
		})
	}
	return resp, nil
}

// operatorID 从传输层元数据提取操作者(由 admin BFF 的 gRPC 客户端中间件注入)。
func (s *AgentNodeService) operatorID(ctx context.Context) uint32 {
	operator, err := metadata.FromServerContext(ctx)
	if err != nil || operator == nil {
		return 0
	}
	return uint32(operator.GetUserId())
}

// GetAgentUpgradeUploadUrl 签发 agent 升级包上传预签名 URL(浏览器直传)。
func (s *AgentNodeService) GetAgentUpgradeUploadUrl(ctx context.Context, req *opsV1.GetAgentUpgradeUploadUrlRequest) (*opsV1.GetAgentUpgradeUploadUrlResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	url, objectKey, err := s.agentService.PresignAgentUpgradeUpload(ctx, req.GetFilename())
	if err != nil {
		s.log.Errorf("presign agent upgrade upload: %s", err.Error())
		return nil, opsV1.ErrorServiceUnavailable("presign upload failed")
	}
	return &opsV1.GetAgentUpgradeUploadUrlResponse{
		UploadUrl: &url,
		ObjectKey: &objectKey,
	}, nil
}

// UpgradeAgent 远程升级:校验对象键前缀 → 预签名下载 URL → 下发
// agent_upgrade 指令(agent 落地新二进制后自替换重启)。
func (s *AgentNodeService) UpgradeAgent(ctx context.Context, req *opsV1.UpgradeAgentRequest) (*opsV1.UpgradeAgentResponse, error) {
	if req == nil || req.GetNodeId() == 0 || req.GetVersion() == "" || req.GetObjectKey() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: nodeId/version/objectKey required")
	}
	if !strings.HasPrefix(req.GetObjectKey(), "agent-uploads/") {
		return nil, opsV1.ErrorBadRequest("objectKey must be under agent-uploads/")
	}

	nodeID := req.GetNodeId()
	node, err := s.repo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		return nil, err
	}

	downloadUrl, err := s.agentService.PresignAgentUpgradeDownload(ctx, req.GetObjectKey())
	if err != nil {
		s.log.Errorf("presign agent upgrade download: %s", err.Error())
		return nil, opsV1.ErrorServiceUnavailable("presign download failed")
	}

	// 实测对象摘要随指令下发:agent 下载后比对一致才自替换(升级通道完整性)
	expectedSha, err := s.agentService.HashAgentUpgradeObject(ctx, req.GetObjectKey())
	if err != nil {
		s.log.Errorf("hash agent upgrade object: %s", err.Error())
		return nil, opsV1.ErrorServiceUnavailable("hash upgrade package failed")
	}

	payload := &signal.AgentUpgradePayload{
		Version:        req.GetVersion(),
		Url:            downloadUrl,
		ExpectedSha256: expectedSha,
	}

	// 审批闸门:APPROVAL_REQUIRED_ACTIONS 含 agent_upgrade 时先建审批单待批
	if upJSON, merr := json.Marshal(payload); merr == nil {
		if aerr := approvalGate(ctx, s.log, s.approvalRepo, string(signal.CommandActionAgentUpgrade),
			node.GetId(), node.GetAgentId(), req.GetVersion(), string(upJSON), s.operatorID(ctx)); aerr != nil {
			return nil, aerr
		}
	}

	requestId, err := s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionAgentUpgrade, payload, s.operatorID(ctx))
	if err != nil {
		s.log.Errorf("dispatch agent upgrade failed (node=%d): %s", nodeID, err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	s.log.Infof("agent upgrade dispatched (node=%d version=%s request_id=%s)", nodeID, req.GetVersion(), requestId)
	return &opsV1.UpgradeAgentResponse{RequestId: requestId}, nil
}
