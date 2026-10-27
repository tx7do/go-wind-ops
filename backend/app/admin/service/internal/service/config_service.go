package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
)

// ConfigService 配置文件中心门面:透传 core。
type ConfigService struct {
	adminV1.ConfigServiceHTTPServer

	log *log.Helper

	configServiceClient opsV1.ConfigServiceClient
}

func NewConfigService(ctx *bootstrap.Context, configServiceClient opsV1.ConfigServiceClient) *ConfigService {
	return &ConfigService{
		log:                 ctx.NewLoggerHelper("config/service/admin-service"),
		configServiceClient: configServiceClient,
	}
}

func (s *ConfigService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListConfigTemplateResponse, error) {
	return s.configServiceClient.ListConfigTemplates(ctx, req)
}

func (s *ConfigService) Get(ctx context.Context, req *opsV1.GetConfigTemplateRequest) (*opsV1.ConfigTemplate, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.configServiceClient.GetConfigTemplate(ctx, req)
}

func (s *ConfigService) Create(ctx context.Context, req *opsV1.CreateConfigTemplateRequest) (*opsV1.ConfigTemplate, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.configServiceClient.CreateConfigTemplate(ctx, req)
}

func (s *ConfigService) Update(ctx context.Context, req *opsV1.UpdateConfigTemplateRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.configServiceClient.UpdateConfigTemplate(ctx, req)
}

func (s *ConfigService) Delete(ctx context.Context, req *opsV1.DeleteConfigTemplateRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	_, err := s.configServiceClient.DeleteConfigTemplate(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ConfigService) Distribute(ctx context.Context, req *opsV1.DistributeConfigRequest) (*opsV1.ConfigDistribution, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetTemplateId() == 0 || req.GetNodeId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: templateId/nodeId required")
	}
	return s.configServiceClient.DistributeConfig(ctx, req)
}

func (s *ConfigService) ListDistributions(ctx context.Context, req *opsV1.ListConfigDistributionsRequest) (*opsV1.ListConfigDistributionResponse, error) {
	if req == nil || req.GetTemplateId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: templateId required")
	}
	return s.configServiceClient.ListConfigDistributions(ctx, req)
}

// CheckConfigDrift 配置漂移校验(透传 core)。
func (s *ConfigService) CheckConfigDrift(ctx context.Context, req *opsV1.CheckConfigDriftRequest) (*opsV1.ConfigDriftReport, error) {
	return s.configServiceClient.CheckConfigDrift(ctx, req)
}
