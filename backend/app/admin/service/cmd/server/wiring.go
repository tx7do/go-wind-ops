package main

import (
	"os"

	"github.com/go-kratos/kratos/v2"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	pkgcrypto "go-wind-ops/pkg/crypto"

	"go-wind-ops/app/admin/service/internal/data"
	"go-wind-ops/app/admin/service/internal/server"
	"go-wind-ops/app/admin/service/internal/service"
	"go-wind-ops/pkg/middleware/auth"
)

// initApp 手写装配整个应用,是本服务的依赖注入点(替代原 google/wire 生成)。
// initApp assembles the whole application by hand — the dependency injection point.
//
// 装配严格单向分层,自上而下的阅读顺序即依赖方向:
// The wiring is strictly layered; reading top-down follows the dependency direction:
//
//	基础设施 → 服务客户端(data) → 服务层(service) → 传输层(server)
//
// 本服务是 BFF 网关:不直连数据库,仓储层由对 core-service 的 gRPC 服务客户端替代。
// 约定 / Conventions:
//   - 新增模块:先加服务客户端行,再加服务构造行,并传给 NewRestServer;漏接由编译器在调用处报错。
//     To add a module: append a service-client line, then a service constructor, then pass it to
//     NewRestServer; a missing connection is a compile error at the call site.
//   - 持有 cleanup 的资源创建成功后立即注册进 cleanups;任何一步失败,rollback 逆序执行已注册
//     的清理(shutdown 与中途失败共用同一条 LIFO 路径)。
//     Resources owning a cleanup register it immediately; rollback runs them LIFO both on
//     mid-way failure and on shutdown.
//   - 本文件只做构造与传参,不写业务逻辑。
//     Construction and parameter passing only; no business logic in this file.
func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	// cleanup 注册表:rollback 时逆序执行。
	// Cleanup registry; rollback runs entries in reverse order.
	var cleanups []func()
	rollback := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// ═══════════════════════ 一、基础设施 ═══════════════════════

	// 全局存储加密器(AES-256-GCM, pkg/crypto):与 core 服务必须配置同一个
	// ENCRYPTION_KEY,否则一边加密一边解不开。未配置时降级明文并告警(生产必须配置)。
	if encKey := os.Getenv("ENCRYPTION_KEY"); encKey != "" {
		if err := pkgcrypto.InitGlobalEncryptor(encKey, true); err != nil {
			return nil, nil, err
		}
	} else {
		log.Warnf("ENCRYPTION_KEY not set: credential encryption DISABLED (secrets stored in plaintext)")
	}

	// 服务发现与鉴权基建
	discovery := data.NewDiscovery(ctx)
	clientType := data.NewClientType()


	redisClient, cleanupRedis, err := data.NewRedisClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupRedis)

	captcha := data.NewCaptcha(redisClient)

	// 认证:认证服务客户端 → 访问令牌校验器
	authenticationServiceClient := data.NewAuthenticationServiceClient(ctx, discovery)
	accessTokenChecker := auth.NewTokenChecker(ctx, authenticationServiceClient, clientType)

	// ═══════════════════════ 二、服务客户端(internal/data) ═══════════════════════

	// 身份与组织
	userServiceClient := data.NewUserServiceClient(ctx, discovery)
	roleServiceClient := data.NewRoleServiceClient(ctx, discovery)
	tenantServiceClient := data.NewTenantServiceClient(ctx, discovery)
	orgUnitServiceClient := data.NewOrgUnitServiceClient(ctx, discovery)
	positionServiceClient := data.NewPositionServiceClient(ctx, discovery)
	userCredentialServiceClient := data.NewUserCredentialServiceClient(ctx, discovery)
	loginPolicyServiceClient := data.NewLoginPolicyServiceClient(ctx, discovery)

	// RBAC
	menuServiceClient := data.NewMenuServiceClient(ctx, discovery)
	apiServiceClient := data.NewApiServiceClient(ctx, discovery)
	permissionServiceClient := data.NewPermissionServiceClient(ctx, discovery)

	// 触发类/敏感入口的权限码守卫(noop authz 下的真实防线,详见 service/permguard.go)
	service.SetPermissionGuard(service.NewPermissionGuard(roleServiceClient, permissionServiceClient))

	// 真 authz 引擎(AUTHZ_MODE=shadow 默认影子模式/enforce/noop):角色→API 策略
	// 数据通道见 data/authz_engine.go
	engine := data.NewAuthzEngine(ctx, roleServiceClient, permissionServiceClient, apiServiceClient)
	permissionGroupServiceClient := data.NewPermissionGroupServiceClient(ctx, discovery)

	// 审计日志
	apiAuditLogServiceClient := data.NewApiAuditLogServiceClient(ctx, discovery)
	loginAuditLogServiceClient := data.NewLoginAuditLogServiceClient(ctx, discovery)
	operationAuditLogServiceClient := data.NewOperationAuditLogServiceClient(ctx, discovery)
	permissionAuditLogServiceClient := data.NewPermissionAuditLogServiceClient(ctx, discovery)
	dataAccessAuditLogServiceClient := data.NewDataAccessAuditLogServiceClient(ctx, discovery)
	policyEvaluationLogServiceClient := data.NewPolicyEvaluationLogServiceClient(ctx, discovery)

	// 字典与多语言
	dictTypeServiceClient := data.NewDictTypeServiceClient(ctx, discovery)
	dictEntryServiceClient := data.NewDictEntryServiceClient(ctx, discovery)
	languageServiceClient := data.NewLanguageServiceClient(ctx, discovery)

	// 任务
	taskServiceClient := data.NewTaskServiceClient(ctx, discovery)

	// Agent 节点
	agentNodeServiceClient := data.NewAgentNodeServiceClient(ctx, discovery)

	// 项目 / 指令台账 / 脚本 / 构建域 / 分发
	projectServiceClient := data.NewProjectServiceClient(ctx, discovery)
	commandRecordServiceClient := data.NewCommandRecordServiceClient(ctx, discovery)
	scriptServiceClient := data.NewScriptServiceClient(ctx, discovery)
	monitorServiceClient := data.NewMonitorServiceClient(ctx, discovery)
	alertServiceClient := data.NewAlertServiceClient(ctx, discovery)
	approvalServiceClient := data.NewApprovalServiceClient(ctx, discovery)
	notificationChannelServiceClient := data.NewNotificationChannelServiceClient(ctx, discovery)
	fileServiceClient := data.NewFileServiceClient(ctx, discovery)
	terminalServiceClient := data.NewTerminalServiceClient(ctx, discovery)
	pipelineServiceClient := data.NewPipelineServiceClient(ctx, discovery)
	apiKeyServiceClient := data.NewApiKeyServiceClient(ctx, discovery)
	pluginServiceClient := data.NewPluginServiceClient(ctx, discovery)
	repositoryServiceClient := data.NewRepositoryServiceClient(ctx, discovery)
	buildServiceClient := data.NewBuildServiceClient(ctx, discovery)
	buildHistoryServiceClient := data.NewBuildHistoryServiceClient(ctx, discovery)
	distributionServiceClient := data.NewDistributionServiceClient(ctx, discovery)

	// ── register:client ── 新模块服务客户端在此行后注册(make register 工具锚点,勿删)
	cloudCostServiceClient := data.NewCloudCostServiceClient(ctx, discovery)
	cloudInstanceServiceClient := data.NewCloudInstanceServiceClient(ctx, discovery)
	cloudAccountServiceClient := data.NewCloudAccountServiceClient(ctx, discovery)
	scriptBatchServiceClient := data.NewScriptBatchServiceClient(ctx, discovery)
	backupServiceClient := data.NewBackupServiceClient(ctx, discovery)
	cloudBudgetServiceClient := data.NewCloudBudgetServiceClient(ctx, discovery)
	certServiceClient := data.NewCertServiceClient(ctx, discovery)
	notificationDeliveryServiceClient := data.NewNotificationDeliveryServiceClient(ctx, discovery)
	terminalAuditServiceClient := data.NewTerminalAuditServiceClient(ctx, discovery)
	inspectionServiceClient := data.NewInspectionServiceClient(ctx, discovery)
	logSnapshotServiceClient := data.NewLogSnapshotServiceClient(ctx, discovery)
	configServiceClient := data.NewConfigServiceClient(ctx, discovery)

	// ═══════════════════════ 三、服务层(internal/service) ═══════════════════════

	// 认证与登录策略
	authenticationService := service.NewAuthenticationService(ctx, authenticationServiceClient, captcha)
	loginPolicyService := service.NewLoginPolicyService(ctx, loginPolicyServiceClient)

	// 身份与组织
	userService := service.NewUserService(ctx, userServiceClient, tenantServiceClient, orgUnitServiceClient, positionServiceClient, roleServiceClient, userCredentialServiceClient)
	userProfileService := service.NewUserProfileService(ctx, userServiceClient, tenantServiceClient, orgUnitServiceClient, positionServiceClient, roleServiceClient, userCredentialServiceClient)
	roleService := service.NewRoleService(ctx, roleServiceClient, tenantServiceClient)
	tenantService := service.NewTenantService(ctx, userServiceClient, userCredentialServiceClient, tenantServiceClient, roleServiceClient)
	orgUnitService := service.NewOrgUnitService(ctx, orgUnitServiceClient, userServiceClient)
	positionService := service.NewPositionService(ctx, positionServiceClient, orgUnitServiceClient)

	// RBAC 与管理门户
	menuService := service.NewMenuService(ctx, menuServiceClient)
	apiService := service.NewApiService(ctx, apiServiceClient)
	permissionGroupService := service.NewPermissionGroupService(ctx, permissionServiceClient, permissionGroupServiceClient)
	permissionService := service.NewPermissionService(ctx, permissionServiceClient, permissionGroupServiceClient, roleServiceClient, apiServiceClient, menuServiceClient)
	adminPortalService := service.NewRouterService(ctx, menuServiceClient, permissionServiceClient, roleServiceClient, userServiceClient)

	// 任务 / 翻译
	taskService := service.NewTaskService(ctx, taskServiceClient)

	// Agent 节点
	agentNodeService := service.NewAgentNodeService(ctx, agentNodeServiceClient)

	// 项目 / 指令台账 / 脚本 / 构建域 / 分发
	projectService := service.NewProjectService(ctx, projectServiceClient)
	commandRecordService := service.NewCommandRecordService(ctx, commandRecordServiceClient)
	scriptService := service.NewScriptService(ctx, scriptServiceClient)
	repositoryService := service.NewRepositoryService(ctx, repositoryServiceClient)
	buildService := service.NewBuildService(ctx, buildServiceClient)
	buildHistoryService := service.NewBuildHistoryService(ctx, buildHistoryServiceClient)
	distributionService := service.NewDistributionService(ctx, distributionServiceClient)
	monitorService := service.NewMonitorService(ctx, monitorServiceClient)
	alertService := service.NewAlertService(ctx, alertServiceClient)
	approvalService := service.NewApprovalService(ctx, approvalServiceClient)
	notificationChannelService := service.NewNotificationChannelService(ctx, notificationChannelServiceClient)
	fileService := service.NewFileService(ctx, fileServiceClient)
	pipelineService := service.NewPipelineService(ctx, pipelineServiceClient)
	apiKeyService := service.NewApiKeyService(ctx, apiKeyServiceClient)
	pluginService := service.NewPluginService(ctx, pluginServiceClient)

	// 字典与多语言
	dictTypeService := service.NewDictTypeService(ctx, dictTypeServiceClient)
	dictEntryService := service.NewDictEntryService(ctx, dictEntryServiceClient)
	languageService := service.NewLanguageService(ctx, languageServiceClient)

	// 审计日志
	apiAuditLogService := service.NewApiAuditLogService(ctx, apiAuditLogServiceClient, apiServiceClient)
	dataAccessAuditLogService := service.NewDataAccessAuditLogService(ctx, dataAccessAuditLogServiceClient)
	loginAuditLogService := service.NewLoginAuditLogService(ctx, loginAuditLogServiceClient)
	policyEvaluationLogService := service.NewPolicyEvaluationLogService(ctx, policyEvaluationLogServiceClient)
	operationAuditLogService := service.NewOperationAuditLogService(ctx, operationAuditLogServiceClient)
	permissionAuditLogService := service.NewPermissionAuditLogService(ctx, permissionAuditLogServiceClient)

	// ── register:service ── 新模块服务在此行后注册(make register 工具锚点,勿删)
	cloudCostService := service.NewCloudCostService(ctx, cloudCostServiceClient)
	cloudInstanceService := service.NewCloudInstanceService(ctx, cloudInstanceServiceClient)
	cloudAccountService := service.NewCloudAccountService(ctx, cloudAccountServiceClient)
	scriptBatchService := service.NewScriptBatchService(ctx, scriptBatchServiceClient)
	backupService := service.NewBackupService(ctx, backupServiceClient)
	cloudBudgetService := service.NewCloudBudgetService(ctx, cloudBudgetServiceClient)
	certService := service.NewCertService(ctx, certServiceClient)
	notificationDeliveryService := service.NewNotificationDeliveryService(ctx, notificationDeliveryServiceClient)
	terminalAuditService := service.NewTerminalAuditService(ctx, terminalAuditServiceClient, roleServiceClient, permissionServiceClient)
	inspectionService := service.NewInspectionService(ctx, inspectionServiceClient)
	logSnapshotService := service.NewLogSnapshotService(ctx, logSnapshotServiceClient)
	configService := service.NewConfigService(ctx, configServiceClient)

	// ═══════════════════════ 四、传输层(internal/server) ═══════════════════════

	restMiddlewares := server.NewRestMiddleware(ctx, accessTokenChecker, engine, apiAuditLogServiceClient, loginAuditLogServiceClient, operationAuditLogServiceClient)

	terminalWsHandler := server.NewTerminalWsHandler(ctx, redisClient, terminalServiceClient)
	terminalTicketHandler := server.NewTerminalTicketHandler(ctx, accessTokenChecker, roleServiceClient, permissionServiceClient, redisClient)
	projectLogWsHandler := server.NewProjectLogWsHandler(ctx, redisClient, projectServiceClient)
	openApiGateway := server.NewOpenApiGateway(apiKeyServiceClient, buildServiceClient, distributionServiceClient, pipelineServiceClient)

	httpServer := server.NewRestServer(ctx, restMiddlewares,
		userService, userProfileService, roleService, tenantService, orgUnitService, positionService,
		menuService, apiService, permissionGroupService, permissionService,
		adminPortalService, taskService,
		agentNodeService,
		projectService, commandRecordService,
		scriptService,
		repositoryService, buildService, buildHistoryService,
		distributionService,
		monitorService, alertService,
		approvalService, notificationChannelService,
		fileService,
		pipelineService,
		apiKeyService,
		pluginService,
		terminalWsHandler, terminalTicketHandler,
		projectLogWsHandler,
		openApiGateway,
		authenticationService, loginPolicyService,
		dictTypeService, dictEntryService, languageService,
		apiAuditLogService, dataAccessAuditLogService, loginAuditLogService, policyEvaluationLogService, operationAuditLogService, permissionAuditLogService,
		// register:rest-arg ── 新模块服务实参在此行后追加(make register 工具锚点,勿删)
		cloudCostService,
		cloudInstanceService,
		cloudAccountService,
		scriptBatchService,
		backupService,
		cloudBudgetService,
		certService,
		notificationDeliveryService,
		terminalAuditService,
		inspectionService,
		logSnapshotService,
		configService,
	)

	grpcMiddlewares := server.NewGrpcMiddleware(ctx)
	grpcServer, err := server.NewGrpcServer(ctx, grpcMiddlewares)
	if err != nil {
		rollback()
		return nil, nil, err
	}

	servers := []kratosTransport.Server{httpServer, grpcServer}

	return newApp(ctx, servers...), rollback, nil
}
