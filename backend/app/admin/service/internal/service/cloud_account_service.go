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

// CloudAccountService 云平台账号管理门面:透传 core。
type CloudAccountService struct {
	adminV1.CloudAccountServiceHTTPServer

	log *log.Helper

	cloudAccountServiceClient opsV1.CloudAccountServiceClient
}

func NewCloudAccountService(ctx *bootstrap.Context, cloudAccountServiceClient opsV1.CloudAccountServiceClient) *CloudAccountService {
	return &CloudAccountService{
		log:                       ctx.NewLoggerHelper("cloud-account/service/admin-service"),
		cloudAccountServiceClient: cloudAccountServiceClient,
	}
}

func (s *CloudAccountService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCloudAccountsResponse, error) {
	return s.cloudAccountServiceClient.ListCloudAccounts(ctx, req)
}

func (s *CloudAccountService) Get(ctx context.Context, req *opsV1.GetCloudAccountRequest) (*opsV1.CloudAccount, error) {
	return s.cloudAccountServiceClient.GetCloudAccount(ctx, req)
}

func (s *CloudAccountService) Create(ctx context.Context, req *opsV1.CreateCloudAccountRequest) (*opsV1.CloudAccount, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.cloudAccountServiceClient.CreateCloudAccount(ctx, req)
}

func (s *CloudAccountService) Update(ctx context.Context, req *opsV1.UpdateCloudAccountRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	_, err := s.cloudAccountServiceClient.UpdateCloudAccount(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CloudAccountService) Delete(ctx context.Context, req *opsV1.DeleteCloudAccountRequest) (*emptypb.Empty, error) {
	_, err := s.cloudAccountServiceClient.DeleteCloudAccount(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CloudAccountService) Verify(ctx context.Context, req *opsV1.VerifyCloudAccountRequest) (*opsV1.VerifyCloudAccountResponse, error) {
	return s.cloudAccountServiceClient.VerifyCloudAccount(ctx, req)
}
