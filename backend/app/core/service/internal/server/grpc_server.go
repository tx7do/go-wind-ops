package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/rpc"

	"go-wind-ops/app/core/service/internal/service"

	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	dictV1 "go-wind-ops/api/gen/go/dict/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"
	taskV1 "go-wind-ops/api/gen/go/task/service/v1"

	"go-wind-ops/pkg/middleware/ent"
)

func NewGrpcMiddleware(ctx *bootstrap.Context) []middleware.Middleware {
	var ms []middleware.Middleware
	ms = append(ms, logging.Server(ctx.GetLogger()))
	ms = append(ms, ent.Server())
	return ms
}

// NewGrpcServer new a gRPC server.
func NewGrpcServer(
	ctx *bootstrap.Context,
	middlewares []middleware.Middleware,

	authenticationService *service.AuthenticationService,
	loginPolicyService *service.LoginPolicyService,
	userCredentialService *service.UserCredentialService,

	taskService *service.TaskService,

	dictTypeService *service.DictTypeService,
	dictEntryService *service.DictEntryService,
	languageService *service.LanguageService,

	tenantService *service.TenantService,
	userService *service.UserService,
	roleService *service.RoleService,
	positionService *service.PositionService,
	orgUnitService *service.OrgUnitService,

	menuService *service.MenuService,
	apiService *service.ApiService,
	permissionService *service.PermissionService,
	permissionGroupService *service.PermissionGroupService,
	permissionAuditLogService *service.PermissionAuditLogService,
	policyEvaluationLogService *service.PolicyEvaluationLogService,

	loginAuditLogService *service.LoginAuditLogService,
	apiAuditLogService *service.ApiAuditLogService,
	operationAuditLogService *service.OperationAuditLogService,
	dataAccessAuditLogService *service.DataAccessAuditLogService,

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
	notificationChannelService *service.NotificationChannelService,
	approvalService *service.ApprovalService,
	fileService *service.FileService,
	terminalService *service.TerminalService,
	pipelineService *service.PipelineService,
	apiKeyService *service.ApiKeyService,
	pluginService *service.PluginService,

	// register:param ── 新模块服务形参在此行后注册(make register 工具锚点,勿删)
) (*grpc.Server, error) {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Grpc == nil {
		return nil, nil
	}

	srv, err := rpc.CreateGrpcServer(cfg, middlewares...)
	if err != nil {
		return nil, err
	}

	taskV1.RegisterTaskServiceServer(srv, taskService)

	authenticationV1.RegisterLoginPolicyServiceServer(srv, loginPolicyService)
	authenticationV1.RegisterAuthenticationServiceServer(srv, authenticationService)
	authenticationV1.RegisterUserCredentialServiceServer(srv, userCredentialService)

	dictV1.RegisterDictTypeServiceServer(srv, dictTypeService)
	dictV1.RegisterDictEntryServiceServer(srv, dictEntryService)
	dictV1.RegisterLanguageServiceServer(srv, languageService)

	permissionV1.RegisterApiServiceServer(srv, apiService)
	permissionV1.RegisterMenuServiceServer(srv, menuService)

	permissionV1.RegisterPermissionServiceServer(srv, permissionService)
	permissionV1.RegisterPermissionGroupServiceServer(srv, permissionGroupService)
	permissionV1.RegisterPolicyEvaluationLogServiceServer(srv, policyEvaluationLogService)
	permissionV1.RegisterRoleServiceServer(srv, roleService)

	identityV1.RegisterUserServiceServer(srv, userService)
	identityV1.RegisterOrgUnitServiceServer(srv, orgUnitService)
	identityV1.RegisterPositionServiceServer(srv, positionService)
	identityV1.RegisterTenantServiceServer(srv, tenantService)

	auditV1.RegisterLoginAuditLogServiceServer(srv, loginAuditLogService)
	auditV1.RegisterApiAuditLogServiceServer(srv, apiAuditLogService)
	auditV1.RegisterOperationAuditLogServiceServer(srv, operationAuditLogService)
	auditV1.RegisterDataAccessAuditLogServiceServer(srv, dataAccessAuditLogService)
	auditV1.RegisterPermissionAuditLogServiceServer(srv, permissionAuditLogService)

	opsV1.RegisterAgentNodeServiceServer(srv, agentNodeService)
	opsV1.RegisterProjectServiceServer(srv, projectService)
	opsV1.RegisterCommandRecordServiceServer(srv, commandRecordService)
	opsV1.RegisterScriptServiceServer(srv, scriptService)
	opsV1.RegisterRepositoryServiceServer(srv, repositoryService)
	opsV1.RegisterBuildServiceServer(srv, buildService)
	opsV1.RegisterBuildHistoryServiceServer(srv, buildHistoryService)
	opsV1.RegisterDistributionServiceServer(srv, distributionService)
	opsV1.RegisterMonitorServiceServer(srv, monitorService)
	opsV1.RegisterAlertServiceServer(srv, alertService)
	opsV1.RegisterNotificationChannelServiceServer(srv, notificationChannelService)
	opsV1.RegisterApprovalServiceServer(srv, approvalService)
	opsV1.RegisterFileServiceServer(srv, fileService)
	opsV1.RegisterTerminalServiceServer(srv, terminalService)
	opsV1.RegisterPipelineServiceServer(srv, pipelineService)
	opsV1.RegisterApiKeyServiceServer(srv, apiKeyService)
	opsV1.RegisterPluginServiceServer(srv, pluginService)

	// register:route ── 新模块路由在此行后注册(make register 工具锚点,勿删)

	return srv, nil
}
