// mfa_service.go MFA(TOTP)二步验证:登录换证与自助绑定管理。
//
// 流程:Login(密码授权)命中已启用 MFA 的用户 → 返回一次性挑战令牌
// (Redis NX+TTL,GETDEL 单次消费)→ 前端跳挑战页 → VerifyMFAChallenge
// 凭挑战令牌+验证码换正式令牌对。绑定走 MfaSetupStart(落密钥未启用)→
// MfaSetupConfirm(验码启用);解绑 MfaDisable(验码清除)。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/totp"
)

const (
	// mfaChallengeKeyPrefix Redis 挑战令牌键前缀。
	mfaChallengeKeyPrefix = "mfa:chal:"
	// mfaChallengeTTL 挑战令牌有效期:密码验证通过后给验证器输码留出时间。
	mfaChallengeTTL = 2 * time.Minute
	// mfaChallengeMaxFails 单个挑战内验证码错误上限(6 位码 ±1 窗口共 3 个有效值,
	// 无上限可在线暴力枚举;超限销毁挑战,须重新走密码授权)。
	mfaChallengeMaxFails = 5
	// mfaIssuer otpauth URI 的 issuer 标识。
	mfaIssuer = "GoWind Ops"
)

type MfaService struct {
	authenticationV1.UnimplementedMfaServiceServer

	log         *log.Helper
	userRepo    data.UserRepo
	redisClient *redis.Client

	// loginTokensIssuer 换证成功的令牌签发尾部,由装配层注入
	// AuthenticationService.IssueLoginTokens(同包直引,避免构造互相依赖)。
	loginTokensIssuer func(ctx context.Context, user *identityV1.User, clientType authenticationV1.ClientType, clientId, deviceId *string) (*authenticationV1.LoginResponse, error)
}

// SetLoginTokensIssuer 装配层注入令牌签发尾部。
func (s *MfaService) SetLoginTokensIssuer(fn func(ctx context.Context, user *identityV1.User, clientType authenticationV1.ClientType, clientId, deviceId *string) (*authenticationV1.LoginResponse, error)) {
	s.loginTokensIssuer = fn
}

func NewMfaService(ctx *bootstrap.Context, userRepo data.UserRepo, redisClient *redis.Client) *MfaService {
	return &MfaService{
		log:         ctx.NewLoggerHelper("mfa/service/core-service"),
		userRepo:    userRepo,
		redisClient: redisClient,
	}
}

// CreateMfaChallenge 生成一次性 MFA 挑战令牌(NX+TTL),载荷为用户/租户/客户端类型
// (换证时回放校验,防跨端挪用)。供 AuthenticationService 密码授权路径调用
// (同进程装配,不走 gRPC)。
func (s *MfaService) CreateMfaChallenge(ctx context.Context, userID, tenantID uint32, clientType authenticationV1.ClientType) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate challenge: %w", err)
	}
	token := hex.EncodeToString(raw)
	val := fmt.Sprintf("%d|%d|%d", userID, tenantID, int32(clientType))
	ok, err := s.redisClient.SetNX(ctx, mfaChallengeKeyPrefix+token, val, mfaChallengeTTL).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("challenge collision")
	}
	return token, nil
}

// consumeMfaChallenge 一次性消费挑战(GETDEL),返回绑定载荷。
func (s *MfaService) consumeMfaChallenge(ctx context.Context, token string) (userID, tenantID uint32, clientType authenticationV1.ClientType, err error) {
	val, err := s.redisClient.GetDel(ctx, mfaChallengeKeyPrefix+token).Result()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("challenge invalid or expired")
	}
	parts := strings.Split(val, "|")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("challenge payload malformed")
	}
	uid, _ := strconv.ParseUint(parts[0], 10, 32)
	tid, _ := strconv.ParseUint(parts[1], 10, 32)
	ct, _ := strconv.ParseInt(parts[2], 10, 32)
	return uint32(uid), uint32(tid), authenticationV1.ClientType(ct), nil
}

// VerifyMFAChallenge MFA 二步验证:兑现挑战令牌 + TOTP 验证码,签发正式令牌对。
// 令牌签发复用 AuthenticationService 的公共尾部(经 LoginTokensIssuer 注入,
// 避免两服务互相依赖)。
func (s *MfaService) VerifyMFAChallenge(ctx context.Context, req *authenticationV1.VerifyMFAChallengeRequest) (*authenticationV1.LoginResponse, error) {
	if req == nil || req.GetOperationId() == "" || len(req.GetTotpCode()) != 6 {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}

	// 防在线枚举:挑战粒度失败计数,超限销毁挑战(重新走密码授权)
	failKey := "mfa:fail:" + req.GetOperationId()
	fails, _ := s.redisClient.Incr(ctx, failKey).Result()
	if fails == 1 {
		s.redisClient.Expire(ctx, failKey, mfaChallengeTTL)
	}
	if fails > mfaChallengeMaxFails {
		s.redisClient.Del(ctx, mfaChallengeKeyPrefix+req.GetOperationId())
		return nil, authenticationV1.ErrorForbidden("too many mfa attempts")
	}

	userID, tenantID, clientType, err := s.consumeMfaChallenge(ctx, req.GetOperationId())
	if err != nil {
		s.redisClient.Del(ctx, failKey)
		return nil, authenticationV1.ErrorUnauthorized("invalid mfa operation id")
	}

	secret, enabled, err := s.userRepo.GetMfaState(ctx, userID)
	if err != nil || !enabled || secret == nil {
		return nil, authenticationV1.ErrorForbidden("mfa not enabled")
	}
	if !totp.Verify(*secret, req.GetTotpCode(), time.Now()) {
		// 备份码兜底:哈希命中即消费(一次性),与 TOTP 等效放行
		if !s.consumeBackupCode(ctx, userID, req.GetTotpCode()) {
			return nil, authenticationV1.ErrorUnauthorized("invalid mfa code")
		}
	}
	s.redisClient.Del(ctx, failKey)

	user, err := s.userRepo.Get(ctx, &identityV1.GetUserRequest{
		QueryBy: &identityV1.GetUserRequest_Id{Id: userID},
	})
	if err != nil {
		return nil, err
	}
	// 挑战签发后用户租户被变更的纵深防御
	if user.GetTenantId() != tenantID {
		return nil, authenticationV1.ErrorForbidden("tenant changed")
	}

	resp, err := s.loginTokensIssuer(ctx, user, clientType, req.ClientId, req.DeviceId)
	if err != nil {
		return nil, err
	}
	resp.MfaRequired = trans.Ptr(true)
	return resp, nil
}

// MfaSetupStart 发起绑定:生成密钥落库(未启用),返回 Base32 密钥与 otpauth URI。
func (s *MfaService) MfaSetupStart(ctx context.Context, _ *authenticationV1.MfaSetupStartRequest) (*authenticationV1.MfaSetupStartResponse, error) {
	userID := operatorIDOf(ctx)
	if userID == 0 {
		return nil, authenticationV1.ErrorUnauthorized("unauthenticated")
	}
	user, err := s.userRepo.Get(ctx, &identityV1.GetUserRequest{
		QueryBy: &identityV1.GetUserRequest_Id{Id: userID},
	})
	if err != nil {
		return nil, err
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		return nil, authenticationV1.ErrorInternalServerError("generate secret failed")
	}
	if err := s.userRepo.SaveMfaSecret(ctx, userID, secret); err != nil {
		return nil, authenticationV1.ErrorInternalServerError("save secret failed")
	}

	account := user.GetUsername()
	if account == "" {
		account = user.GetEmail()
	}
	return &authenticationV1.MfaSetupStartResponse{
		Secret:     secret,
		OtpauthUri: totp.OtpauthURI(mfaIssuer, account, secret),
	}, nil
}

// MfaSetupConfirm 确认绑定:验证码校验通过后启用。
func (s *MfaService) MfaSetupConfirm(ctx context.Context, req *authenticationV1.MfaSetupConfirmRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.GetCode()) != 6 {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}
	userID := operatorIDOf(ctx)
	if userID == 0 {
		return nil, authenticationV1.ErrorUnauthorized("unauthenticated")
	}
	secret, enabled, err := s.userRepo.GetMfaState(ctx, userID)
	if err != nil || secret == nil {
		return nil, authenticationV1.ErrorBadRequest("mfa setup not started")
	}
	if enabled {
		return &emptypb.Empty{}, nil
	}
	if !totp.Verify(*secret, req.GetCode(), time.Now()) {
		return nil, authenticationV1.ErrorUnauthorized("invalid mfa code")
	}
	if err := s.userRepo.SetMfaEnabled(ctx, userID, true); err != nil {
		return nil, authenticationV1.ErrorInternalServerError("enable mfa failed")
	}
	s.log.Infof("mfa enabled for user [%d]", userID)
	return &emptypb.Empty{}, nil
}

// MfaDisable 解绑:验证码校验通过后清除密钥并停用。
func (s *MfaService) MfaDisable(ctx context.Context, req *authenticationV1.MfaDisableRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.GetCode()) != 6 {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}
	userID := operatorIDOf(ctx)
	if userID == 0 {
		return nil, authenticationV1.ErrorUnauthorized("unauthenticated")
	}
	secret, enabled, err := s.userRepo.GetMfaState(ctx, userID)
	if err != nil || !enabled || secret == nil {
		return nil, authenticationV1.ErrorBadRequest("mfa not enabled")
	}
	if !totp.Verify(*secret, req.GetCode(), time.Now()) {
		return nil, authenticationV1.ErrorUnauthorized("invalid mfa code")
	}
	if err := s.userRepo.ClearMfa(ctx, userID); err != nil {
		return nil, authenticationV1.ErrorInternalServerError("disable mfa failed")
	}
	s.log.Infof("mfa disabled for user [%d]", userID)
	return &emptypb.Empty{}, nil
}

// ── 备份码 ───────────────────────────────────────────────────────────

// MfaBackupCodesRegenerate 生成 10 个新备份码:明文仅本次响应出现,服务端
// 只存 SHA-256;重生成即作废旧码。须已启用 MFA。
func (s *MfaService) MfaBackupCodesRegenerate(ctx context.Context, _ *authenticationV1.MfaBackupCodesRegenerateRequest) (*authenticationV1.MfaBackupCodesResponse, error) {
	userID := operatorIDOf(ctx)
	if userID == 0 {
		return nil, authenticationV1.ErrorUnauthorized("unauthenticated")
	}
	_, enabled, err := s.userRepo.GetMfaState(ctx, userID)
	if err != nil || !enabled {
		return nil, authenticationV1.ErrorBadRequest("mfa not enabled")
	}

	codes := make([]string, 0, 10)
	hashes := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		raw := make([]byte, 4)
		if _, err = rand.Read(raw); err != nil {
			return nil, authenticationV1.ErrorInternalServerError("generate failed")
		}
		code := fmt.Sprintf("%s-%s", hex.EncodeToString(raw[:2]), hex.EncodeToString(raw[2:]))
		sum := sha256.Sum256([]byte(code))
		codes = append(codes, code)
		hashes = append(hashes, hex.EncodeToString(sum[:]))
	}
	if err = s.userRepo.SetMfaBackupCodes(ctx, userID, hashes); err != nil {
		return nil, authenticationV1.ErrorInternalServerError("save failed")
	}
	s.log.Infof("mfa backup codes regenerated for user [%d]", userID)
	return &authenticationV1.MfaBackupCodesResponse{Codes: codes}, nil
}

// consumeBackupCode 备份码消费:哈希命中即从列表移除(一次性),返回是否命中。
func (s *MfaService) consumeBackupCode(ctx context.Context, userID uint32, code string) bool {
	hashes, err := s.userRepo.GetMfaBackupCodes(ctx, userID)
	if err != nil || len(hashes) == 0 {
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	want := hex.EncodeToString(sum[:])
	for i, h := range hashes {
		if h == want {
			remaining := append(append([]string{}, hashes[:i]...), hashes[i+1:]...)
			if err = s.userRepo.SetMfaBackupCodes(ctx, userID, remaining); err != nil {
				s.log.Errorf("consume backup code for user [%d]: %s", userID, err.Error())
				return false
			}
			s.log.Infof("mfa backup code consumed for user [%d] (remaining=%d)", userID, len(remaining))
			return true
		}
	}
	return false
}
