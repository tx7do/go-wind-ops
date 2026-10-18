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

// ApiKeyService OpenAPI 密钥管理门面:透传 core(Validate 不对外暴露)。
type ApiKeyService struct {
	adminV1.ApiKeyServiceHTTPServer

	log *log.Helper

	apiKeyServiceClient opsV1.ApiKeyServiceClient
}

func NewApiKeyService(ctx *bootstrap.Context, apiKeyServiceClient opsV1.ApiKeyServiceClient) *ApiKeyService {
	return &ApiKeyService{
		log:                 ctx.NewLoggerHelper("api-key/service/admin-service"),
		apiKeyServiceClient: apiKeyServiceClient,
	}
}

func (s *ApiKeyService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApiKeyResponse, error) {
	return s.apiKeyServiceClient.ListApiKeys(ctx, req)
}

func (s *ApiKeyService) Get(ctx context.Context, req *opsV1.GetApiKeyRequest) (*opsV1.ApiKey, error) {
	return s.apiKeyServiceClient.GetApiKey(ctx, req)
}

func (s *ApiKeyService) Create(ctx context.Context, req *opsV1.CreateApiKeyRequest) (*opsV1.CreateApiKeyResponse, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.apiKeyServiceClient.CreateApiKey(ctx, req)
}

func (s *ApiKeyService) Update(ctx context.Context, req *opsV1.UpdateApiKeyRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	_, err := s.apiKeyServiceClient.UpdateApiKey(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ApiKeyService) Delete(ctx context.Context, req *opsV1.DeleteApiKeyRequest) (*emptypb.Empty, error) {
	_, err := s.apiKeyServiceClient.DeleteApiKey(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
