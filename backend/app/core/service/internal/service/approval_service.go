package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ApprovalService 操作审批:危险操作(生产分发/脚本执行)需审批人确认后才执行。
// 发起方创建审批单并暂存操作载荷;审批通过后由发起方重新提交操作(带审批单 ID)。
type ApprovalService struct {
	opsV1.UnimplementedApprovalServiceServer

	log  *log.Helper
	repo *data.ApprovalRepo
}

func NewApprovalService(ctx *bootstrap.Context, repo *data.ApprovalRepo) *ApprovalService {
	return &ApprovalService{
		log:  ctx.NewLoggerHelper("approval/service/core-service"),
		repo: repo,
	}
}

func (s *ApprovalService) ListApprovals(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApprovalResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *ApprovalService) GetApproval(ctx context.Context, req *opsV1.GetApprovalRequest) (*opsV1.Approval, error) {
	return s.repo.Get(ctx, req)
}

// ApproveApproval 审批通过(状态流转 PENDING→APPROVED)。
// 发起方收到审批通过通知后重新提交操作,并携带审批单 ID 证明已获批准。
func (s *ApprovalService) ApproveApproval(ctx context.Context, req *opsV1.ApproveApprovalRequest) (*opsV1.OpApprovalResult, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Resolve(ctx, req.GetId(), true, operatorIDOf(ctx), ""); err != nil {
		return nil, err
	}
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
