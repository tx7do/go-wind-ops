package service

import (
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ApprovalService 操作审批:危险操作(生产分发/脚本执行)需审批人确认后才执行。
// 审批通过后由本服务重新下发 MQTT 指令(使用审批单存储的 agent_id + payload)。
type ApprovalService struct {
	opsV1.UnimplementedApprovalServiceServer

	log      *log.Helper
	repo     *data.ApprovalRepo
	agentSvc *AgentService
}

func NewApprovalService(ctx *bootstrap.Context, repo *data.ApprovalRepo, agentSvc *AgentService) *ApprovalService {
	return &ApprovalService{
		log:      ctx.NewLoggerHelper("approval/service/core-service"),
		repo:     repo,
		agentSvc: agentSvc,
	}
}

func (s *ApprovalService) ListApprovals(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApprovalResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *ApprovalService) GetApproval(ctx context.Context, req *opsV1.GetApprovalRequest) (*opsV1.Approval, error) {
	return s.repo.Get(ctx, req)
}

// ApproveApproval 审批通过:标记 APPROVED 并重新下发存储的 MQTT 指令。
func (s *ApprovalService) ApproveApproval(ctx context.Context, req *opsV1.ApproveApprovalRequest) (*opsV1.OpApprovalResult, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	record, err := s.repo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("approval not found")
	}

	// CAS 标记 APPROVED
	if err = s.repo.Resolve(ctx, req.GetId(), true, operatorIDOf(ctx), ""); err != nil {
		return nil, err
	}

	// 重新下发指令:用审批单存储的 agent_id 和 payload
	var payload json.RawMessage
	if record.Payload != nil && *record.Payload != "" {
		payload = json.RawMessage(*record.Payload)
	}

	_, dispatchErr := s.agentSvc.PublishCommand(ctx, 0, record.AgentID,
		signal.CommandAction(record.OperationType), payload, operatorIDOf(ctx))
	if dispatchErr != nil {
		s.log.Errorf("approved operation dispatch failed (approval=%d agent=%s action=%s): %s",
			req.GetId(), record.AgentID, record.OperationType, dispatchErr.Error())
		// 执行结果回写失败详情,审批人可见;状态仍为 APPROVED(不回退,便于人工排查后重发)
		if merr := s.repo.MarkExecuted(ctx, req.GetId(), "dispatch failed: "+dispatchErr.Error()); merr != nil {
			s.log.Warnf("mark approval dispatch-failed [%d]: %s", req.GetId(), merr.Error())
		}
		return nil, opsV1.ErrorInternalServerError("approved operation dispatch failed")
	}

	// 执行闭环:重下发成功后回写审批单执行结果
	if merr := s.repo.MarkExecuted(ctx, req.GetId(), "dispatched request completed"); merr != nil {
		s.log.Warnf("mark approval executed [%d]: %s", req.GetId(), merr.Error())
	}

	s.log.Infof("approved operation dispatched (approval=%d agent=%s action=%s)",
		req.GetId(), record.AgentID, record.OperationType)
	return &opsV1.OpApprovalResult{}, nil
}

// RejectApproval 驳回审批。
func (s *ApprovalService) RejectApproval(ctx context.Context, req *opsV1.RejectApprovalRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Resolve(ctx, req.GetId(), false, operatorIDOf(ctx), req.GetReason()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
