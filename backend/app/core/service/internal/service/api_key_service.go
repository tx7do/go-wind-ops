// api_key_service.go OpenAPI 密钥管理:创建(明文仅返一次)/CRUD/校验。
// 密钥格式 opsk_<48hex>,库内仅存 SHA-256;校验时异步回写最近使用时间。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent/apikey"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type ApiKeyService struct {
	opsV1.UnimplementedApiKeyServiceServer

	log  *log.Helper
	repo *data.ApiKeyRepo
}

func NewApiKeyService(ctx *bootstrap.Context, repo *data.ApiKeyRepo) *ApiKeyService {
	return &ApiKeyService{
		log:  ctx.NewLoggerHelper("api-key/service/core-service"),
		repo: repo,
	}
}

// generatePlainKey 生成明文密钥 opsk_<48hex>。
func generatePlainKey() (plain, hash, prefix string, err error) {
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("generate key: %w", err)
	}
	plain = "opsk_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	prefix = plain[:12]
	return plain, hash, prefix, nil
}

// hashKey 明文 → SHA-256 hex(常数时间比较由调用方用 subtle 完成)。
func hashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func (s *ApiKeyService) ListApiKeys(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApiKeyResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *ApiKeyService) GetApiKey(ctx context.Context, req *opsV1.GetApiKeyRequest) (*opsV1.ApiKey, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.repo.Get(ctx, req.GetId())
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, req *opsV1.CreateApiKeyRequest) (*opsV1.CreateApiKeyResponse, error) {
	if req == nil || req.Data == nil || req.Data.GetName() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: name required")
	}
	if taken, _ := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0); taken {
		return nil, opsV1.ErrorConflict("api key name already exists")
	}

	plain, hash, prefix, err := generatePlainKey()
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("%s", err.Error())
	}

	var expiresAt *time.Time
	if t := req.Data.GetExpiresAt(); t != nil {
		tt := t.AsTime()
		expiresAt = &tt
	}
	status := opsV1.ApiKey_ENABLED
	if req.Data.Status != nil {
		status = req.Data.GetStatus()
	}

	dto, err := s.repo.Create(ctx, req.Data.GetName(), hash, prefix, status, req.Data.Description, expiresAt, operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("create api key failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create failed")
	}

	s.log.Infof("api key created (id=%d name=%s prefix=%s operator=%d)", dto.GetId(), dto.GetName(), prefix, operatorIDOf(ctx))
	return &opsV1.CreateApiKeyResponse{Data: dto, PlainKey: &plain}, nil
}

func (s *ApiKeyService) UpdateApiKey(ctx context.Context, req *opsV1.UpdateApiKeyRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	var expiresAt *time.Time
	var expiresAtPtr **time.Time
	if req.Data.ExpiresAt != nil {
		tt := req.Data.GetExpiresAt().AsTime()
		expiresAt = &tt
		expiresAtPtr = &expiresAt
	}
	if _, err := s.repo.Update(ctx, req.GetId(), req.Data.Status, req.Data.Description, expiresAtPtr); err != nil {
		s.log.Errorf("update api key [%d] failed: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("update failed")
	}
	return &emptypb.Empty{}, nil
}

func (s *ApiKeyService) DeleteApiKey(ctx context.Context, req *opsV1.DeleteApiKeyRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}

// ValidateApiKey 校验明文:存在 + ENABLED + 未过期。命中后异步回写最近使用。
func (s *ApiKeyService) ValidateApiKey(ctx context.Context, req *opsV1.ValidateApiKeyRequest) (*opsV1.ApiKey, error) {
	if req == nil || req.GetKey() == "" {
		return nil, opsV1.ErrorBadRequest("key required")
	}
	wantHash := hashKey(req.GetKey())

	entity, err := s.repo.GetByHash(ctx, wantHash)
	if err != nil {
		return nil, opsV1.ErrorForbidden("invalid api key")
	}
	if entity.Status != nil && *entity.Status == apikey.StatusDisabled {
		return nil, opsV1.ErrorForbidden("api key disabled")
	}
	if entity.ExpiresAt != nil && entity.ExpiresAt.Before(time.Now()) {
		return nil, opsV1.ErrorForbidden("api key expired")
	}

	// 异步回写最近使用(不影响主流程)
	go s.repo.TouchLastUsed(context.WithoutCancel(ctx), entity.ID)

	return s.repo.Get(ctx, entity.ID)
}
