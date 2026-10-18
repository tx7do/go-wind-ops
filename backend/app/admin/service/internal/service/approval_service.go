package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ApprovalService 操作审批门面:透传 core 的 ApprovalService。
// Approve 由 core 侧完成状态流转后重新下发存储的 MQTT 指令。
type ApprovalService struct {
	adminV1.ApprovalServiceHTTPServer

	log *log.Helper

	approvalServiceClient opsV1.ApprovalServiceClient
}

func NewApprovalService(ctx *bootstrap.Context, approvalServiceClient opsV1.ApprovalServiceClient) *ApprovalService {
	return &ApprovalService{
		log:                   ctx.NewLoggerHelper("approval/service/admin-service"),
		approvalServiceClient: approvalServiceClient,
	}
}

func (s *ApprovalService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApprovalResponse, error) {
	return s.approvalServiceClient.ListApprovals(ctx, req)
}

func (s *ApprovalService) Get(ctx context.Context, req *opsV1.GetApprovalRequest) (*opsV1.Approval, error) {
	return s.approvalServiceClient.GetApproval(ctx, req)
}

func (s *ApprovalService) Approve(ctx context.Context, req *opsV1.ApproveApprovalRequest) (*opsV1.OpApprovalResult, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.approvalServiceClient.ApproveApproval(ctx, req)
}

func (s *ApprovalService) Reject(ctx context.Context, req *opsV1.RejectApprovalRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	_, err := s.approvalServiceClient.RejectApproval(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
