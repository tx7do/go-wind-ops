// cloud_account_service.go 云平台账号管理:CRUD + 在线校验。
// SK 落库前经 pkgcrypto 加密;校验时解密后调厂商身份 API,回填账号 ID/名。
package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	pkgcrypto "go-wind-ops/pkg/crypto"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent/cloudaccount"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/cloudprovider"
)

type CloudAccountService struct {
	opsV1.UnimplementedCloudAccountServiceServer

	log  *log.Helper
	repo *data.CloudAccountRepo
}

func NewCloudAccountService(ctx *bootstrap.Context, repo *data.CloudAccountRepo) *CloudAccountService {
	return &CloudAccountService{
		log:  ctx.NewLoggerHelper("cloud-account/service/core-service"),
		repo: repo,
	}
}

func (s *CloudAccountService) ListCloudAccounts(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCloudAccountsResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *CloudAccountService) GetCloudAccount(ctx context.Context, req *opsV1.GetCloudAccountRequest) (*opsV1.CloudAccount, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.repo.Get(ctx, req.GetId())
}

func (s *CloudAccountService) CreateCloudAccount(ctx context.Context, req *opsV1.CreateCloudAccountRequest) (*opsV1.CloudAccount, error) {
	if req == nil || req.Data == nil || req.Data.GetName() == "" ||
		req.Data.GetProvider() == opsV1.CloudAccount_PROVIDER_UNSPECIFIED ||
		req.Data.GetAccessKey() == "" || req.GetSecretKey() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: name/provider/accessKey/secretKey required")
	}
	if taken, _ := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0); taken {
		return nil, opsV1.ErrorConflict("cloud account name already exists")
	}

	secretEncrypted, encErr := pkgcrypto.EncryptIfNeeded(req.GetSecretKey())
	if encErr != nil {
		return nil, opsV1.ErrorInternalServerError("encrypt secret failed")
	}

	status := opsV1.CloudAccount_ENABLED
	if req.Data.Status != nil {
		status = req.Data.GetStatus()
	}

	dto, err := s.repo.Create(ctx, req.Data.GetName(), req.Data.GetProvider(),
		req.Data.GetAccessKey(), secretEncrypted, req.Data.GetRegion(),
		status, req.Data.Description, operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("create cloud account failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create failed")
	}

	s.log.Infof("cloud account created (id=%d name=%s provider=%s operator=%d)",
		dto.GetId(), dto.GetName(), req.Data.GetProvider(), operatorIDOf(ctx))
	return dto, nil
}

func (s *CloudAccountService) UpdateCloudAccount(ctx context.Context, req *opsV1.UpdateCloudAccountRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if name := req.Data.GetName(); name != "" {
		if taken, _ := s.repo.IsNameTaken(ctx, name, req.GetId()); taken {
			return nil, opsV1.ErrorConflict("cloud account name already exists")
		}
	}

	var secretEncrypted *string
	if req.SecretKey != nil && req.GetSecretKey() != "" {
		enc, encErr := pkgcrypto.EncryptIfNeeded(req.GetSecretKey())
		if encErr != nil {
			return nil, opsV1.ErrorInternalServerError("encrypt secret failed")
		}
		secretEncrypted = &enc
	}

	if _, err := s.repo.Update(ctx, req.GetId(),
		req.Data.GetName(), req.Data.Provider, req.Data.AccessKey, secretEncrypted,
		req.Data.Region, req.Data.Status, req.Data.Description); err != nil {
		s.log.Errorf("update cloud account [%d] failed: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("update failed")
	}
	return &emptypb.Empty{}, nil
}

func (s *CloudAccountService) DeleteCloudAccount(ctx context.Context, req *opsV1.DeleteCloudAccountRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}

// VerifyCloudAccount 在线校验:解密 SK → 调厂商身份 API → 回填账号 ID/名。
func (s *CloudAccountService) VerifyCloudAccount(ctx context.Context, req *opsV1.VerifyCloudAccountRequest) (*opsV1.VerifyCloudAccountResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	entity, err := s.repo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("cloud account not found")
	}
	if entity.Provider == nil || entity.AccessKey == nil || *entity.AccessKey == "" ||
		entity.SecretKey == nil || *entity.SecretKey == "" {
		return nil, opsV1.ErrorBadRequest("cloud account missing provider/accessKey/secretKey")
	}

	secret, decErr := pkgcrypto.DecryptIfNeeded(*entity.SecretKey)
	if decErr != nil {
		return nil, opsV1.ErrorInternalServerError("decrypt secret failed")
	}

	provider := providerVerifyName(*entity.Provider)
	var region string
	if entity.Region != nil {
		region = *entity.Region
	}
	result, vErr := cloudprovider.Verify(ctx, provider, *entity.AccessKey, secret, region)
	if vErr != nil {
		s.log.Warnf("cloud account [%d] verify failed: %s", req.GetId(), vErr.Error())
		return nil, opsV1.ErrorInternalServerError("%s", vErr.Error())
	}

	now := time.Now()
	if err := s.repo.UpdateVerifyResult(ctx, req.GetId(), result.AccountID, result.AccountName, now); err != nil {
		s.log.Errorf("cloud account [%d] verify result writeback failed: %s", req.GetId(), err.Error())
	}

	s.log.Infof("cloud account [%d] verified (accountId=%s operator=%d)", req.GetId(), result.AccountID, operatorIDOf(ctx))
	return &opsV1.VerifyCloudAccountResponse{
		AccountId:   &result.AccountID,
		AccountName: &result.AccountName,
		VerifiedAt:  timestamppb.New(now),
	}, nil
}

// providerVerifyName ent 枚举 → cloudprovider 包使用的厂商标识。
func providerVerifyName(p cloudaccount.Provider) string {
	switch p {
	case cloudaccount.ProviderTencentCloud:
		return "TENCENT_CLOUD"
	case cloudaccount.ProviderHuaweiCloud:
		return "HUAWEI_CLOUD"
	case cloudaccount.ProviderAWS:
		return "AWS"
	default:
		return "ALIYUN"
	}
}
