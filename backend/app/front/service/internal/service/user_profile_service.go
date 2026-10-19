package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	googleProtoFieldmask "google.golang.org/protobuf/types/known/fieldmaskpb"

	appV1 "go-wind-ops/api/gen/go/app/service/v1"
	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"

	"go-wind-ops/pkg/middleware/auth"
)

type UserProfileService struct {
	appV1.UserProfileServiceHTTPServer

	log *log.Helper

	userServiceClient     identityV1.UserServiceClient
	tenantServiceClient   identityV1.TenantServiceClient
	orgUnitServiceClient  identityV1.OrgUnitServiceClient
	positionServiceClient identityV1.PositionServiceClient

	roleServiceClient permissionV1.RoleServiceClient

	userCredentialServiceClient authenticationV1.UserCredentialServiceClient
}

func NewUserProfileService(
	ctx *bootstrap.Context,
	userServiceClient identityV1.UserServiceClient,
	tenantServiceClient identityV1.TenantServiceClient,
	orgUnitServiceClient identityV1.OrgUnitServiceClient,
	positionServiceClient identityV1.PositionServiceClient,
	roleServiceClient permissionV1.RoleServiceClient,
	userCredentialServiceClient authenticationV1.UserCredentialServiceClient,
) *UserProfileService {
	return &UserProfileService{
		log:                         ctx.NewLoggerHelper("user-profile/service/front-service"),
		userServiceClient:           userServiceClient,
		tenantServiceClient:         tenantServiceClient,
		orgUnitServiceClient:        orgUnitServiceClient,
		positionServiceClient:       positionServiceClient,
		roleServiceClient:           roleServiceClient,
		userCredentialServiceClient: userCredentialServiceClient,
	}
}

func (s *UserProfileService) GetUser(ctx context.Context, _ *emptypb.Empty) (*identityV1.User, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.userServiceClient.Get(ctx, &identityV1.GetUserRequest{
		QueryBy: &identityV1.GetUserRequest_Id{
			Id: operator.UserId,
		},
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// profileFieldWhitelist C 端自助资料更新允许修改的字段(显式白名单):
// 角色等关系型字段绝不可经此入口设置,否则任何注册用户可自授角色提权。
var profileFieldWhitelist = []string{
	"nickname", "realname", "avatar", "email", "mobile",
	"telephone", "region", "address", "description", "remark", "gender",
}

func (s *UserProfileService) UpdateUser(ctx context.Context, req *identityV1.UpdateUserRequest) (*emptypb.Empty, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, appV1.ErrorBadRequest("invalid parameter")
	}

	// 以白名单重建请求:只透传资料字段,roleIds/orgUnitIds/positionIds 等
	// 一律丢弃(即使调用方手动构造 update_mask 也无效)。
	clean := &identityV1.UpdateUserRequest{
		Id:   operator.UserId,
		Data: &identityV1.User{Id: trans.Ptr(operator.UserId)},
		UpdateMask: &googleProtoFieldmask.FieldMask{
			Paths: append([]string(nil), profileFieldWhitelist...),
		},
	}
	if req.UpdateMask != nil {
		// 只在调用方掩码内的白名单字段才真正更新,其余置空跳过
		asked := make(map[string]struct{}, len(req.UpdateMask.Paths))
		for _, p := range req.UpdateMask.Paths {
			asked[p] = struct{}{}
		}
		paths := make([]string, 0, len(profileFieldWhitelist))
		for _, f := range profileFieldWhitelist {
			if _, ok := asked[f]; ok {
				paths = append(paths, f)
			}
		}
		clean.UpdateMask.Paths = paths
	}
	for _, f := range clean.UpdateMask.Paths {
		switch f {
		case "nickname":
			clean.Data.Nickname = req.Data.Nickname
		case "realname":
			clean.Data.Realname = req.Data.Realname
		case "avatar":
			clean.Data.Avatar = req.Data.Avatar
		case "email":
			clean.Data.Email = req.Data.Email
		case "mobile":
			clean.Data.Mobile = req.Data.Mobile
		case "telephone":
			clean.Data.Telephone = req.Data.Telephone
		case "region":
			clean.Data.Region = req.Data.Region
		case "address":
			clean.Data.Address = req.Data.Address
		case "description":
			clean.Data.Description = req.Data.Description
		case "remark":
			clean.Data.Remark = req.Data.Remark
		case "gender":
			clean.Data.Gender = req.Data.Gender
		}
	}

	return s.userServiceClient.Update(ctx, clean)
}

func (s *UserProfileService) ChangePassword(ctx context.Context, req *identityV1.ChangePasswordRequest) (*emptypb.Empty, error) {
	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	// NeedDecrypt:true → core 会先把 AES(base64) 密文解密成明文再做 bcrypt
	// 校验/入库,与登录/注册的密码传输口径一致(前端提交 AES 密文)。
	return s.userCredentialServiceClient.ChangeCredential(ctx, &authenticationV1.ChangeCredentialRequest{
		IdentityType:  authenticationV1.UserCredential_USERNAME,
		Identifier:    operator.GetUsername(),
		OldCredential: req.GetOldPassword(),
		NewCredential: req.GetNewPassword(),
		NeedDecrypt:   true,
	})
}

// BindContact 绑定手机号码/邮箱 — 尚未实现
func (s *UserProfileService) BindContact(_ context.Context, _ *identityV1.BindContactRequest) (*emptypb.Empty, error) {
	return nil, appV1.ErrorNotImplemented("contact binding is not implemented")
}

// VerifyContact 验证手机号码/邮箱 — 尚未实现
func (s *UserProfileService) VerifyContact(_ context.Context, _ *identityV1.VerifyContactRequest) (*emptypb.Empty, error) {
	return nil, appV1.ErrorNotImplemented("contact verification is not implemented")
}
