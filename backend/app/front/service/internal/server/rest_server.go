package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"

	authzEngine "github.com/tx7do/kratos-authz/engine"
	authz "github.com/tx7do/kratos-authz/middleware"

	swaggerUI "github.com/tx7do/kratos-swagger-ui"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/rpc"

	"go-wind-ops/app/front/service/cmd/server/assets"
	"go-wind-ops/app/front/service/internal/service"

	appV1 "go-wind-ops/api/gen/go/app/service/v1"
	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"

	"go-wind-ops/pkg/middleware/auth"
	entmiddleware "go-wind-ops/pkg/middleware/ent"
	applogging "go-wind-ops/pkg/middleware/logging"
)

// OperationAuthenticationServiceRegister 手工挂载的 C 端注册路由操作名。
// 手工路由无法从 proto 再生成,路由按生成代码的等价方式手工挂载;
// 须与 AddWhiteList 保持一致。
const OperationAuthenticationServiceRegister = "/app.v1.AuthenticationService/Register"

// NewRestMiddleware 创建中间件
func NewRestMiddleware(
	ctx *bootstrap.Context,
	accessTokenChecker auth.AccessTokenChecker,
	authorizer authzEngine.Engine,
	tenantResolver entmiddleware.TenantResolver,
) []middleware.Middleware {
	var ms []middleware.Middleware
	ms = append(ms, logging.Server(ctx.GetLogger()))

	// add white list for authentication.
	// GenerateCaptcha 和 VerifyCaptcha 必须在白名单中，因为登录需要验证码，
	// 而获取验证码时用户尚无 token（鸡生蛋问题）。
	rpc.AddWhiteList(
		appV1.OperationAuthenticationServiceLogin,
		// AuthenticationService.GetOidcAuthorizeUrl：OIDC 登录的前置步骤（登录页
		// 尚无 token），与 Login 同列免鉴权；state 防 CSRF 由 core 生成校验。
		appV1.OperationAuthenticationServiceGetOidcAuthorizeUrl,
		// AuthenticationService.RefreshToken：access token 过期后凭 refresh_token 换新，
		// 调用时 bearer 往往已过期，若要求有效 token 刷新将永远 401。
		appV1.OperationAuthenticationServiceRefreshToken,
		// MfaService.VerifyMFAChallenge：MFA 二步验证第二步（登录流程内），
		// 与 Login 同列免鉴权；挑战令牌由 core 一次性签发消费。
		appV1.OperationMfaServiceVerifyMFAChallenge,

		OperationAuthenticationServiceRegister,
	)

	ms = append(ms, applogging.Server(
		applogging.WithLogger(ctx.NewLoggerHelper("audit-logging/front-service")),
		applogging.WithWriteApiLogFunc(func(ctx context.Context, data *auditV1.ApiAuditLog) error {
			return nil
		}),
		applogging.WithWriteLoginLogFunc(func(ctx context.Context, data *auditV1.LoginAuditLog) error {
			return nil
		}),
	))

	// 鉴权必须在 ent.Server() 之前执行：auth.Server 对非白名单请求注入
	// OperatorMetadata，随后 ent.Server() 才能据此构建带租户作用域的 UserViewer。
	// 若顺序颠倒，ent.Server() 总以 md==nil 兜底为 SystemViewer，导致租户隔离失效。
	ms = append(ms, selector.Server(
		auth.Server(
			auth.WithAccessTokenChecker(accessTokenChecker),
			auth.WithInjectMetadata(true),
			auth.WithInjectEnt(true),
		),
		authz.Server(authorizer),
	).Match(rpc.NewRestWhiteListMatcher()).Build())

	// ent.Server() 必须在 auth.Server 之后：此时非白名单请求已注入 OperatorMetadata，
	// 可构建 UserViewer；白名单请求（登录/验证码）md==nil 但在白名单内，兜底 SystemViewer。
	ms = append(ms, entmiddleware.Server(entmiddleware.WithTenantResolver(tenantResolver)))

	return ms
}

// NewRestServer new an REST server.
func NewRestServer(
	ctx *bootstrap.Context,

	middlewares []middleware.Middleware,

	authenticationService *service.AuthenticationService,
	mfaService *service.MfaService,
	userProfileService *service.UserProfileService,

	// register:param ── 新模块服务形参在此行后注册(make register 工具锚点,勿删)
) *http.Server {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Rest == nil {
		return nil
	}

	srv, err := rpc.CreateRestServer(cfg, middlewares...)
	if err != nil {
		panic(err)
	}

	appV1.RegisterAuthenticationServiceHTTPServer(srv, authenticationService)
	appV1.RegisterMfaServiceHTTPServer(srv, mfaService)

	// ── 手工挂载:C 端注册 POST /app/v1/register ──
	// 与生成代码等价的注册方式;操作名须与 AddWhiteList 一致。
	{
		registerRoute := srv.Route("/")
		registerRoute.POST("/app/v1/register", func(ctx http.Context) error {
			http.SetOperation(ctx, OperationAuthenticationServiceRegister)
			var in authenticationV1.RegisterUserRequest
			if err := ctx.Bind(&in); err != nil {
				return err
			}
			out, err := authenticationService.Register(ctx, &in)
			if err != nil {
				return err
			}
			return ctx.Result(200, out)
		})
	}

	appV1.RegisterUserProfileServiceHTTPServer(srv, userProfileService)

	// register:route ── 新模块路由在此行后注册(make register 工具锚点,勿删)

	if cfg.GetServer().GetRest().GetEnableSwagger() {
		swaggerUI.RegisterSwaggerUIServerWithOption(
			srv,
			swaggerUI.WithTitle("GoWind Ops Front API"),
			swaggerUI.WithMemoryData(assets.OpenApiData, "yaml"),
		)
	}

	return srv
}
