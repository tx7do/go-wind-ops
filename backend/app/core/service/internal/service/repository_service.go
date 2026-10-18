package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	pkgcrypto "go-wind-ops/pkg/crypto"
	"go-wind-ops/pkg/metadata"

	"go-wind-ops/app/core/service/internal/data"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// RepositoryService 构建源仓库管理:密码经全局加密器落库,对外恒为掩码。
type RepositoryService struct {
	opsV1.UnimplementedRepositoryServiceServer

	log  *log.Helper
	repo *data.RepositoryRepo
}

func NewRepositoryService(ctx *bootstrap.Context, repo *data.RepositoryRepo) *RepositoryService {
	return &RepositoryService{
		log:  ctx.NewLoggerHelper("repository/service/core-service"),
		repo: repo,
	}
}

func (s *RepositoryService) ListRepositories(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListRepositoryResponse, error) {
	resp, err := s.repo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range resp.GetItems() {
		data.MaskPassword(item)
	}
	return resp, nil
}

func (s *RepositoryService) GetRepository(ctx context.Context, req *opsV1.GetRepositoryRequest) (*opsV1.Repository, error) {
	dto, err := s.repo.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	return data.MaskPassword(dto), nil
}

func (s *RepositoryService) CreateRepository(ctx context.Context, req *opsV1.CreateRepositoryRequest) (*opsV1.Repository, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetGitUrl() == "" {
		return nil, opsV1.ErrorBadRequest("name/gitUrl are required")
	}

	taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("repository name already exists")
	}

	if pw := req.Data.GetPassword(); pw != "" && pw != data.PasswordMask {
		encrypted, encErr := pkgcrypto.EncryptIfNeeded(pw)
		if encErr != nil {
			return nil, opsV1.ErrorInternalServerError("encrypt credential failed")
		}
		req.Data.Password = &encrypted
	}

	dto, err := s.repo.Create(ctx, req, operatorIDOf(ctx))
	if err != nil {
		return nil, err
	}
	return data.MaskPassword(dto), nil
}

func (s *RepositoryService) UpdateRepository(ctx context.Context, req *opsV1.UpdateRepositoryRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	// 密码:掩码或空 → 不更新;明文 → 加密后落库
	switch pw := req.Data.GetPassword(); {
	case pw == "" || pw == data.PasswordMask:
		req.Data.Password = nil
	default:
		encrypted, encErr := pkgcrypto.EncryptIfNeeded(pw)
		if encErr != nil {
			return nil, opsV1.ErrorInternalServerError("encrypt credential failed")
		}
		req.Data.Password = &encrypted
	}

	if req.Data.Name != nil {
		taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("repository name already exists")
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *RepositoryService) DeleteRepository(ctx context.Context, req *opsV1.DeleteRepositoryRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func operatorIDOf(ctx context.Context) uint32 {
	if operator, err := metadata.FromServerContext(ctx); err == nil && operator != nil {
		return uint32(operator.GetUserId())
	}
	return 0
}
