package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
)

// InspectionService 平台巡检门面:透传 core。
type InspectionService struct {
	adminV1.InspectionServiceHTTPServer

	log *log.Helper

	inspectionServiceClient opsV1.InspectionServiceClient
}

func NewInspectionService(ctx *bootstrap.Context, inspectionServiceClient opsV1.InspectionServiceClient) *InspectionService {
	return &InspectionService{
		log:                     ctx.NewLoggerHelper("inspection/service/admin-service"),
		inspectionServiceClient: inspectionServiceClient,
	}
}

func (s *InspectionService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListInspectionResponse, error) {
	return s.inspectionServiceClient.ListInspections(ctx, req)
}

func (s *InspectionService) Get(ctx context.Context, req *opsV1.GetInspectionRequest) (*opsV1.Inspection, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.inspectionServiceClient.GetInspection(ctx, req)
}

func (s *InspectionService) Run(ctx context.Context, req *opsV1.RunInspectionRequest) (*opsV1.Inspection, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	return s.inspectionServiceClient.RunInspection(ctx, req)
}
