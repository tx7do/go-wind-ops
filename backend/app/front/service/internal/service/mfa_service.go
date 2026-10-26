// mfa_service.go C 端 MFA(TOTP)二步验证 BFF 门面:VerifyMFAChallenge 免鉴权
// (白名单)并叠加失败锁定;绑定/解绑三件套鉴权透传 core。
package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	appV1 "go-wind-ops/api/gen/go/app/service/v1"
	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	"go-wind-ops/pkg/logalock"
)

type MfaService struct {
	appV1.MfaServiceHTTPServer

	log *log.Helper

	mfaServiceClient authenticationV1.MfaServiceClient

	loginLock *logalock.Locker
}

func NewMfaService(
	ctx *bootstrap.Context,
	mfaServiceClient authenticationV1.MfaServiceClient,
	redisClient *redis.Client,
) *MfaService {
	locker := logalock.New(redisClient, ctx.NewLoggerHelper("mfa-lock/front-service"))
	return &MfaService{
		log:              ctx.NewLoggerHelper("mfa/service/front-service"),
		mfaServiceClient: mfaServiceClient,
		loginLock:        locker,
	}
}

// VerifyMFAChallenge MFA 二步验证第二步:凭挑战令牌+验证码换正式令牌(免鉴权)。
// 失败锁定以挑战令牌为键(此时无用户名可依);core 侧已有挑战级错误计数与
// 2 分钟 TTL 双重压制,此处叠加跨挑战的常规锁定语义。
func (s *MfaService) VerifyMFAChallenge(ctx context.Context, req *authenticationV1.VerifyMFAChallengeRequest) (*authenticationV1.LoginResponse, error) {
	if req == nil {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}

	lockKey := "mfa:" + req.GetOperationId()
	if lerr := s.loginLock.Check(ctx, lockKey); lerr != nil {
		return nil, lerr
	}
	resp, err := s.mfaServiceClient.VerifyMFAChallenge(ctx, req)
	if err != nil {
		s.loginLock.RecordFailure(ctx, lockKey)
		return nil, err
	}
	s.loginLock.Clear(ctx, lockKey)
	return resp, nil
}

// MfaSetupStart 发起 MFA 绑定(鉴权透传 core)。
func (s *MfaService) MfaSetupStart(ctx context.Context, req *authenticationV1.MfaSetupStartRequest) (*authenticationV1.MfaSetupStartResponse, error) {
	return s.mfaServiceClient.MfaSetupStart(ctx, req)
}

// MfaSetupConfirm 确认 MFA 绑定(鉴权透传 core)。
func (s *MfaService) MfaSetupConfirm(ctx context.Context, req *authenticationV1.MfaSetupConfirmRequest) (*emptypb.Empty, error) {
	return s.mfaServiceClient.MfaSetupConfirm(ctx, req)
}

// MfaDisable 解绑 MFA(鉴权透传 core)。
func (s *MfaService) MfaDisable(ctx context.Context, req *authenticationV1.MfaDisableRequest) (*emptypb.Empty, error) {
	return s.mfaServiceClient.MfaDisable(ctx, req)
}
