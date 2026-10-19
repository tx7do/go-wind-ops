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

// CertService 证书到期台账门面:透传 core 的 CertService。
type CertService struct {
	adminV1.CertServiceHTTPServer

	log *log.Helper

	certServiceClient opsV1.CertServiceClient
}

func NewCertService(ctx *bootstrap.Context, certServiceClient opsV1.CertServiceClient) *CertService {
	return &CertService{
		log:               ctx.NewLoggerHelper("cert/service/admin-service"),
		certServiceClient: certServiceClient,
	}
}

func (s *CertService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCertResponse, error) {
	return s.certServiceClient.ListCerts(ctx, req)
}

func (s *CertService) Get(ctx context.Context, req *opsV1.GetCertRequest) (*opsV1.Cert, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.certServiceClient.GetCert(ctx, req)
}

func (s *CertService) Create(ctx context.Context, req *opsV1.CreateCertRequest) (*opsV1.Cert, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.certServiceClient.CreateCert(ctx, req)
}

func (s *CertService) Update(ctx context.Context, req *opsV1.UpdateCertRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}

	req.Data.Id = req.Id

	return s.certServiceClient.UpdateCert(ctx, req)
}

func (s *CertService) Delete(ctx context.Context, req *opsV1.DeleteCertRequest) (*emptypb.Empty, error) {
	_, err := s.certServiceClient.DeleteCert(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CertService) Probe(ctx context.Context, req *opsV1.ProbeCertRequest) (*opsV1.ProbeCertResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.certServiceClient.ProbeCert(ctx, req)
}
