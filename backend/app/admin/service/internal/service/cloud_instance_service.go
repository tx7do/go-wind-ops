package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CloudInstanceService 云资源实例清单门面:透传 core。
type CloudInstanceService struct {
	adminV1.CloudInstanceServiceHTTPServer

	log *log.Helper

	cloudInstanceServiceClient opsV1.CloudInstanceServiceClient
}

func NewCloudInstanceService(ctx *bootstrap.Context, cloudInstanceServiceClient opsV1.CloudInstanceServiceClient) *CloudInstanceService {
	return &CloudInstanceService{
		log:                        ctx.NewLoggerHelper("cloud-instance/service/admin-service"),
		cloudInstanceServiceClient: cloudInstanceServiceClient,
	}
}

func (s *CloudInstanceService) List(ctx context.Context, req *opsV1.ListCloudInstancesRequest) (*opsV1.ListCloudInstancesResponse, error) {
	if req == nil {
		req = &opsV1.ListCloudInstancesRequest{}
	}
	return s.cloudInstanceServiceClient.ListCloudInstances(ctx, req)
}

func (s *CloudInstanceService) Sync(ctx context.Context, req *opsV1.SyncCloudAccountInstancesRequest) (*opsV1.SyncCloudAccountInstancesResponse, error) {
	if req == nil || req.GetCloudAccountId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: cloudAccountId required")
	}
	return s.cloudInstanceServiceClient.SyncCloudAccountInstances(ctx, req)
}
