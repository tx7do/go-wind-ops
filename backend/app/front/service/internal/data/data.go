package data

import (
	"github.com/redis/go-redis/v9"

	authnEngine "github.com/tx7do/kratos-authn/engine"
	"github.com/tx7do/kratos-authn/engine/jwt"

	authzEngine "github.com/tx7do/kratos-authz/engine"
	"github.com/tx7do/kratos-authz/engine/noop"

	"github.com/go-kratos/kratos/v2/registry"
	"google.golang.org/grpc"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	redisClient "github.com/tx7do/kratos-bootstrap/cache/redis"
	bRegistry "github.com/tx7do/kratos-bootstrap/registry"
	"github.com/tx7do/kratos-bootstrap/rpc"

	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"

	"go-wind-ops/pkg/middleware/rpcsecret"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/serviceid"
)

// newCoreGrpcConn 拨连 core gRPC 的统一入口:配置了 CORE_RPC_SECRET 时
// 自动附加内部共享密钥头(与 core 侧 rpcsecret.Server 校验配对)。
func newCoreGrpcConn(ctx *bootstrap.Context, r registry.Discovery) (grpc.ClientConnInterface, error) {
	return rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig(), rpcsecret.Client(rpcsecret.SecretFromEnv()))
}

func NewClientType() authenticationV1.ClientType {
	return authenticationV1.ClientType_app
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(ctx *bootstrap.Context) (*redis.Client, func(), error) {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, func() {}, nil
	}

	l := ctx.NewLoggerHelper("redis/data/front-service")

	cli := redisClient.NewClient(cfg.Data, l)

	return cli, func() {
		if err := cli.Close(); err != nil {
			l.Error(err)
		}
	}, nil
}

// NewDiscovery 创建服务发现客户端
func NewDiscovery(ctx *bootstrap.Context) registry.Discovery {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil
	}

	discovery, err := bRegistry.NewDiscovery(cfg.Registry)
	if err != nil {
		return nil
	}


	return discovery
}

func NewMinIoClient(ctx *bootstrap.Context) *oss.MinIOClient {
	return oss.NewMinIoClient(ctx.GetConfig(), ctx.GetLogger())
}

// NewAuthenticator 创建认证器
func NewAuthenticator(cfg *conf.Bootstrap) authnEngine.Authenticator {
	// 拒绝已知的开发默认密钥，避免生产环境因未设置 jwt_signing_key 而使用
	// 公开默认值导致令牌可被离线伪造。
	signingKey := cfg.Server.Rest.Middleware.Auth.Key
	if signingKey == "" || signingKey == "dev_only_change_me_in_prod" {
		panic("jwt signing key must be set to a non-default secret in production")
	}

	authenticator, _ := jwt.NewAuthenticator(
		jwt.WithKey([]byte(signingKey)),
		jwt.WithSigningMethod(cfg.Server.Rest.Middleware.Auth.Method),
	)
	return authenticator
}

// NewAuthorizer 创建权鉴器
func NewAuthorizer() authzEngine.Engine {
	return noop.State{}
}

func NewAuthenticationServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.AuthenticationServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return authenticationV1.NewAuthenticationServiceClient(cli)
}

func NewUserCredentialServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.UserCredentialServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return authenticationV1.NewUserCredentialServiceClient(cli)
}

func NewUserServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.UserServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return identityV1.NewUserServiceClient(cli)
}

func NewTenantServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.TenantServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return identityV1.NewTenantServiceClient(cli)
}

func NewRoleServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.RoleServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return permissionV1.NewRoleServiceClient(cli)
}

func NewOrgUnitServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.OrgUnitServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return identityV1.NewOrgUnitServiceClient(cli)
}

func NewPositionServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.PositionServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return identityV1.NewPositionServiceClient(cli)
}
