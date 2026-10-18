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

	"go-wind-ops/app/admin/service/cmd/server/assets"
	"go-wind-ops/app/admin/service/internal/service"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"

	"go-wind-ops/pkg/metadata"
	"go-wind-ops/pkg/middleware/auth"
	applogging "go-wind-ops/pkg/middleware/logging"
	entmiddleware "go-wind-ops/pkg/middleware/ent"
)

// NewRestMiddleware 创建中间件
func NewRestMiddleware(
	ctx *bootstrap.Context,
	accessTokenChecker auth.AccessTokenChecker,
	authorizer authzEngine.Engine,
	apiAuditLogServiceClient auditV1.ApiAuditLogServiceClient,
	loginAuditLogServiceClient auditV1.LoginAuditLogServiceClient,
	operationAuditLogServiceClient auditV1.OperationAuditLogServiceClient,
) []middleware.Middleware {
	var ms []middleware.Middleware
	ms = append(ms, logging.Server(ctx.GetLogger()))

	// add white list for authentication.
	// GenerateCaptcha 和 VerifyCaptcha 必须在白名单中，因为登录需要验证码，
	// 而获取验证码时用户尚无 token（鸡生蛋问题）。
	rpc.AddWhiteList(
		adminV1.OperationAuthenticationServiceLogin,
		adminV1.OperationAuthenticationServiceGenerateCaptcha,
		adminV1.OperationAuthenticationServiceVerifyCaptcha,
		// AuthenticationService.GetOidcAuthorizeUrl：OIDC 登录的前置步骤（登录页
		// 尚无 token），与 Login 同列免鉴权；state 防 CSRF 由 core 生成校验。
		adminV1.OperationAuthenticationServiceGetOidcAuthorizeUrl,
		// AuthenticationService.RefreshToken：access token 过期后凭 refresh_token 换新，
		// 调用时 bearer 往往已过期，若要求有效 token 刷新将永远 401。
		adminV1.OperationAuthenticationServiceRefreshToken,
	)

	ms = append(ms, applogging.Server(
		applogging.WithWriteApiLogFunc(func(ctx context.Context, data *auditV1.ApiAuditLog) error {
			ctx, _ = metadata.NewContext(ctx, metadata.NewUserOperator(0, 0, 0, identityV1.DataScope_ALL))
			// TODO 如果系统的负载比较小，可以同步写入数据库，否则，建议使用异步方式，即投递进队列。
			_, err := apiAuditLogServiceClient.Create(ctx, &auditV1.CreateApiAuditLogRequest{Data: data})
			return err
		}),
		applogging.WithWriteLoginLogFunc(func(ctx context.Context, data *auditV1.LoginAuditLog) error {
			// TODO 如果系统的负载比较小，可以同步写入数据库，否则，建议使用异步方式，即投递进队列。
			ctx, _ = metadata.NewContext(ctx, metadata.NewUserOperator(0, 0, 0, identityV1.DataScope_ALL))
			_, err := loginAuditLogServiceClient.Create(ctx, &auditV1.CreateLoginAuditLogRequest{Data: data})
			return err
		}),
		applogging.WithWriteOperationLogFunc(func(ctx context.Context, data *auditV1.OperationAuditLog) error {
			ctx, _ = metadata.NewContext(ctx, metadata.NewUserOperator(0, 0, 0, identityV1.DataScope_ALL))
			_, err := operationAuditLogServiceClient.Create(ctx, &auditV1.CreateOperationAuditLogRequest{Data: data})
			return err
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
	ms = append(ms, entmiddleware.Server())

	return ms
}

// NewRestServer new an REST server.
func NewRestServer(
	ctx *bootstrap.Context,

	middlewares []middleware.Middleware,

	userService *service.UserService,
	userProfileService *service.UserProfileService,
	roleService *service.RoleService,
	tenantService *service.TenantService,
	orgUnitService *service.OrgUnitService,
	positionService *service.PositionService,

	menuSvc *service.MenuService,
	apiService *service.ApiService,
	permissionGroupService *service.PermissionGroupService,
	permissionService *service.PermissionService,

	adminPortalService *service.AdminPortalService,
	taskService *service.TaskService,

	agentNodeService *service.AgentNodeService,
	projectService *service.ProjectService,
	commandRecordService *service.CommandRecordService,
	scriptService *service.ScriptService,
	repositoryService *service.RepositoryService,
	buildService *service.BuildService,
	buildHistoryService *service.BuildHistoryService,
	distributionService *service.DistributionService,
	monitorService *service.MonitorService,
	alertService *service.AlertService,
	approvalService *service.ApprovalService,
	notificationChannelService *service.NotificationChannelService,
	fileService *service.FileService,
	pipelineService *service.PipelineService,
	apiKeyService *service.ApiKeyService,
	pluginService *service.PluginService,
	terminalWsHandler *TerminalWsHandler,
	openApiGateway *OpenApiGateway,

	authenticationService *service.AuthenticationService,
	loginPolicyService *service.LoginPolicyService,

	dictTypeService *service.DictTypeService,
	dictEntryService *service.DictEntryService,
	languageService *service.LanguageService,


	apiAuditLogService *service.ApiAuditLogService,
	dataAccessAuditLogService *service.DataAccessAuditLogService,
	loginAuditLogService *service.LoginAuditLogService,
	policyEvaluationLogService *service.PolicyEvaluationLogService,
	operationAuditLogService *service.OperationAuditLogService,
	permissionAuditLogService *service.PermissionAuditLogService,

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

	// Web 终端 WS 端点:直接注册在 mux 上,不经过鉴权中间件
	// (浏览器 WS 无法带 Authorization 头,handler 内校验查询参数 token)。
	if terminalWsHandler != nil {
		srv.Handle(terminalWsHandler.Path(), terminalWsHandler)
	}

	// OpenAPI 网关:API Key 认证,不经过 JWT 中间件
	if openApiGateway != nil {
		srv.HandlePrefix(openApiGateway.Prefix(), openApiGateway)
	}

	adminV1.RegisterAuthenticationServiceHTTPServer(srv, authenticationService)
	adminV1.RegisterLoginPolicyServiceHTTPServer(srv, loginPolicyService)

	adminV1.RegisterUserProfileServiceHTTPServer(srv, userProfileService)
	adminV1.RegisterUserServiceHTTPServer(srv, userService)
	adminV1.RegisterRoleServiceHTTPServer(srv, roleService)
	adminV1.RegisterTenantServiceHTTPServer(srv, tenantService)
	adminV1.RegisterOrgUnitServiceHTTPServer(srv, orgUnitService)
	adminV1.RegisterPositionServiceHTTPServer(srv, positionService)

	adminV1.RegisterAdminPortalServiceHTTPServer(srv, adminPortalService)
	adminV1.RegisterTaskServiceHTTPServer(srv, taskService)

	adminV1.RegisterAgentNodeServiceHTTPServer(srv, agentNodeService)
	adminV1.RegisterProjectServiceHTTPServer(srv, projectService)
	adminV1.RegisterCommandRecordServiceHTTPServer(srv, commandRecordService)
	adminV1.RegisterScriptServiceHTTPServer(srv, scriptService)
	adminV1.RegisterRepositoryServiceHTTPServer(srv, repositoryService)
	adminV1.RegisterBuildServiceHTTPServer(srv, buildService)
	adminV1.RegisterBuildHistoryServiceHTTPServer(srv, buildHistoryService)
	adminV1.RegisterDistributionServiceHTTPServer(srv, distributionService)
	adminV1.RegisterMonitorServiceHTTPServer(srv, monitorService)
	adminV1.RegisterAlertServiceHTTPServer(srv, alertService)
	adminV1.RegisterApprovalServiceHTTPServer(srv, approvalService)
	adminV1.RegisterNotificationChannelServiceHTTPServer(srv, notificationChannelService)
	adminV1.RegisterFileServiceHTTPServer(srv, fileService)
	adminV1.RegisterPipelineServiceHTTPServer(srv, pipelineService)
	adminV1.RegisterApiKeyServiceHTTPServer(srv, apiKeyService)
	adminV1.RegisterPluginServiceHTTPServer(srv, pluginService)

	adminV1.RegisterDictTypeServiceHTTPServer(srv, dictTypeService)
	adminV1.RegisterDictEntryServiceHTTPServer(srv, dictEntryService)
	adminV1.RegisterLanguageServiceHTTPServer(srv, languageService)

	adminV1.RegisterApiServiceHTTPServer(srv, apiService)
	adminV1.RegisterMenuServiceHTTPServer(srv, menuSvc)
	adminV1.RegisterPermissionGroupServiceHTTPServer(srv, permissionGroupService)
	adminV1.RegisterPermissionServiceHTTPServer(srv, permissionService)

	adminV1.RegisterApiAuditLogServiceHTTPServer(srv, apiAuditLogService)
	adminV1.RegisterDataAccessAuditLogServiceHTTPServer(srv, dataAccessAuditLogService)
	adminV1.RegisterLoginAuditLogServiceHTTPServer(srv, loginAuditLogService)
	adminV1.RegisterOperationAuditLogServiceHTTPServer(srv, operationAuditLogService)
	adminV1.RegisterPermissionAuditLogServiceHTTPServer(srv, permissionAuditLogService)
	adminV1.RegisterPolicyEvaluationLogServiceHTTPServer(srv, policyEvaluationLogService)


	// register:route ── 新模块路由在此行后注册(make register 工具锚点,勿删)

	if cfg.GetServer().GetRest().GetEnableSwagger() {
		swaggerUI.RegisterSwaggerUIServerWithOption(
			srv,
			swaggerUI.WithTitle("GoWind Ops Admin API"),
			swaggerUI.WithMemoryData(assets.OpenApiData, "yaml"),
		)
	}

	return srv
}
