package main

import (
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	pkgcrypto "go-wind-ops/pkg/crypto"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/client"
	"go-wind-ops/app/core/service/internal/server"
	"go-wind-ops/app/core/service/internal/service"
)

// initApp 手写装配整个应用,是本服务的依赖注入点(替代原 google/wire 生成)。
// initApp assembles the whole application by hand — the dependency injection point.
//
// 装配严格单向分层,自上而下的阅读顺序即依赖方向:
// The wiring is strictly layered; reading top-down follows the dependency direction:
//
//	基础设施 → 仓储层(data) → 服务层(service) → 传输层(server)
//
// 约定 / Conventions:
//   - 新增仓储/服务:在对应分层小节追加一行构造,并传给下游消费者;漏接由编译器在调用处报错。
//     To add a repo/service: append one constructor line in its layer section, then pass it to
//     downstream consumers; a missing connection is a compile error at the call site.
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

	// 全局存储加密器(AES-256-GCM, pkg/crypto):凭据类字段落库加密的总开关。
	// key 从 ENCRYPTION_KEY 环境变量读取;未配置时降级明文并告警(生产必须配置)。
	if encKey := os.Getenv("ENCRYPTION_KEY"); encKey != "" {
		if err := pkgcrypto.InitGlobalEncryptor(encKey, true); err != nil {
			return nil, nil, err
		}
	} else {
		log.Warnf("ENCRYPTION_KEY not set: credential encryption DISABLED (secrets stored in plaintext)")
	}

	// 认证基建:令牌缓存 → 认证器
	authenticatorOption := data.NewAuthenticatorConfig(ctx)

	redisClient, cleanupRedis, err := client.NewRedisClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupRedis)

	userTokenCache := data.NewUserTokenCache(ctx, redisClient)
	authenticator := data.NewAuthenticator(ctx, authenticatorOption, userTokenCache)

	entClient, cleanupEnt, err := client.NewEntClient(ctx)
	if err != nil {
		rollback()
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupEnt)

	crypto := data.NewPasswordCrypto()
	minIOClient := client.NewMinIoClient(ctx)

	// ═══════════════════════ 二、仓储层(internal/data) ═══════════════════════

	// 身份与账号
	userCredentialRepo := data.NewUserCredentialRepo(ctx, entClient, crypto)
	loginPolicyRepo := data.NewLoginPolicyRepo(ctx, entClient)
	userRoleRepo := data.NewUserRoleRepo(ctx, entClient)
	userOrgUnitRepo := data.NewUserOrgUnitRepo(ctx, entClient)
	userPositionRepo := data.NewUserPositionRepo(ctx, entClient)
	membershipRoleRepo := data.NewMembershipRoleRepo(ctx, entClient)
	membershipPositionRepo := data.NewMembershipPositionRepo(ctx, entClient)
	membershipOrgUnitRepo := data.NewMembershipOrgUnitRepo(ctx, entClient)
	membershipRepo := data.NewMembershipRepo(ctx, entClient, membershipRoleRepo, membershipPositionRepo, membershipOrgUnitRepo)
	userRepo := data.NewUserRepo(ctx, entClient, userRoleRepo, userOrgUnitRepo, userPositionRepo, membershipRepo)

	// 租户
	tenantRepo := data.NewTenantRepo(ctx, entClient)

	// RBAC:权限(聚合 Api/Menu 挂载点) → 角色(聚合权限)
	rolePermissionRepo := data.NewRolePermissionRepo(ctx, entClient)
	permissionApiRepo := data.NewPermissionApiRepo(ctx, entClient)
	permissionMenuRepo := data.NewPermissionMenuRepo(ctx, entClient)
	permissionRepo := data.NewPermissionRepo(ctx, entClient, permissionApiRepo, permissionMenuRepo, rolePermissionRepo)
	roleMetadataRepo := data.NewRoleMetadataRepo(ctx, entClient)
	roleRepo := data.NewRoleRepo(ctx, entClient, rolePermissionRepo, permissionRepo, roleMetadataRepo, userRoleRepo)

	// RBAC 挂载点:菜单 / Api / 权限组
	menuRepo := data.NewMenuRepo(ctx, entClient)
	apiRepo := data.NewApiRepo(ctx, entClient)
	permissionGroupRepo := data.NewPermissionGroupRepo(ctx, entClient)

	// 组织架构
	positionRepo := data.NewPositionRepo(ctx, entClient)
	orgUnitRepo := data.NewOrgUnitRepo(ctx, entClient)

	// 审计日志
	permissionAuditLogRepo := data.NewPermissionAuditLogRepo(ctx, entClient)
	policyEvaluationLogRepo := data.NewPolicyEvaluationLogRepo(ctx, entClient)
	loginAuditLogRepo := data.NewLoginAuditLogRepo(ctx, entClient)
	apiAuditLogRepo := data.NewApiAuditLogRepo(ctx, entClient)
	operationAuditLogRepo := data.NewOperationAuditLogRepo(ctx, entClient)
	dataAccessAuditLogRepo := data.NewDataAccessAuditLogRepo(ctx, entClient)

	// 任务
	taskRepo := data.NewTaskRepo(ctx, entClient)

	// 字典与多语言
	dictEntryI18nRepo := data.NewDictEntryI18nRepo(ctx, entClient)
	dictEntryRepo := data.NewDictEntryRepo(ctx, entClient, dictEntryI18nRepo)
	dictTypeRepo := data.NewDictTypeRepo(ctx, entClient, dictEntryRepo)
	languageRepo := data.NewLanguageRepo(ctx, entClient)

	// Agent 节点(受管机器资产)与项目/指令台账/脚本/构建/分发
	agentNodeRepo := data.NewAgentNodeRepo(ctx, entClient)
	projectRepo := data.NewProjectRepo(ctx, entClient)
	commandRecordRepo := data.NewCommandRecordRepo(ctx, entClient)
	scriptRepo := data.NewScriptRepo(ctx, entClient)
	repositoryRepo := data.NewRepositoryRepo(ctx, entClient)
	buildRepo := data.NewBuildRepo(ctx, entClient)
	buildHistoryRepo := data.NewBuildHistoryRepo(ctx, entClient)
	distributionRepo := data.NewDistributionRepo(ctx, entClient)
	distributionRecordRepo := data.NewDistributionRecordRepo(ctx, entClient)
	monitorRepo := data.NewMonitorRepo(ctx, entClient)
	approvalRepo := data.NewApprovalRepo(ctx, entClient)
	alertRepo := data.NewAlertRepo(ctx, entClient)
	notificationChannelRepo := data.NewNotificationChannelRepo(ctx, entClient)

	// ── register:repo ── 新模块仓储在此行后注册(make register 工具锚点,勿删)

	// ═══════════════════════ 三、服务层(internal/service) ═══════════════════════

	// 认证与登录策略
	authenticationService := service.NewAuthenticationService(ctx, authenticator, userCredentialRepo, userRepo, roleRepo, tenantRepo, permissionRepo, redisClient)
	loginPolicyService := service.NewLoginPolicyService(ctx, loginPolicyRepo)
	userCredentialService := service.NewUserCredentialService(ctx, userCredentialRepo)

	// 任务
	taskService := service.NewTaskService(ctx, taskRepo, userRepo)

	// 字典与多语言
	dictTypeService := service.NewDictTypeService(ctx, dictTypeRepo)
	dictEntryService := service.NewDictEntryService(ctx, dictEntryRepo)
	languageService := service.NewLanguageService(ctx, languageRepo)

	// 租户与身份
	tenantService := service.NewTenantService(ctx, tenantRepo, userRepo, userCredentialRepo, roleRepo)
	userService := service.NewUserService(ctx, userRepo, roleRepo, userCredentialRepo, positionRepo, orgUnitRepo, tenantRepo, membershipRepo)
	roleService := service.NewRoleService(ctx, roleRepo, tenantRepo, userRoleRepo, userRepo)
	positionService := service.NewPositionService(ctx, positionRepo, orgUnitRepo)
	orgUnitService := service.NewOrgUnitService(ctx, orgUnitRepo, userRepo)

	// RBAC
	menuService := service.NewMenuService(ctx, menuRepo)
	apiService := service.NewApiService(ctx, apiRepo)
	permissionService := service.NewPermissionService(ctx, permissionRepo, permissionGroupRepo, menuRepo, apiRepo, roleRepo)
	permissionGroupService := service.NewPermissionGroupService(ctx, permissionGroupRepo, permissionRepo)

	// 审计日志
	permissionAuditLogService := service.NewPermissionAuditLogService(ctx, permissionAuditLogRepo)
	policyEvaluationLogService := service.NewPolicyEvaluationLogService(ctx, policyEvaluationLogRepo)
	loginAuditLogService := service.NewLoginAuditLogService(ctx, loginAuditLogRepo)
	apiAuditLogService := service.NewApiAuditLogService(ctx, apiAuditLogRepo, apiRepo)
	operationAuditLogService := service.NewOperationAuditLogService(ctx, operationAuditLogRepo)
	dataAccessAuditLogService := service.NewDataAccessAuditLogService(ctx, dataAccessAuditLogRepo)

	// Agent 信令(节点准入/心跳/指令下发;MinIO 预签名签发经 pkg/oss)
	metricRepo := data.NewNodeMetricRepo(ctx, entClient)
	agentService := service.NewAgentService(ctx, agentNodeRepo, commandRecordRepo, projectRepo, distributionRecordRepo, metricRepo, minIOClient)
	agentNodeService := service.NewAgentNodeService(ctx, agentNodeRepo, metricRepo, agentService)
	projectService := service.NewProjectService(ctx, projectRepo, agentNodeRepo, agentService)
	commandRecordService := service.NewCommandRecordService(ctx, commandRecordRepo)
	scriptService := service.NewScriptService(ctx, scriptRepo, agentNodeRepo, agentService)
	repositoryService := service.NewRepositoryService(ctx, repositoryRepo)
	buildService := service.NewBuildService(ctx, buildRepo, repositoryRepo, buildHistoryRepo, minIOClient)
	buildHistoryService := service.NewBuildHistoryService(ctx, buildHistoryRepo, buildRepo, repositoryRepo, minIOClient)
	distributionService := service.NewDistributionService(ctx, distributionRepo, distributionRecordRepo, buildRepo, buildHistoryRepo, projectRepo, agentNodeRepo, minIOClient, agentService)
	monitorService := service.NewMonitorService(ctx, monitorRepo)
	alertService := service.NewAlertService(ctx, alertRepo)
	notificationChannelService := service.NewNotificationChannelService(ctx, notificationChannelRepo)
	approvalService := service.NewApprovalService(ctx, approvalRepo, agentService)
	fileService := service.NewFileService(ctx, agentService, agentNodeRepo)

	// 定时调度(构建/脚本 cron + 监控检查)
	cronScheduler := service.NewCronScheduler(ctx, buildRepo, scriptRepo, monitorRepo, alertRepo, projectRepo, agentNodeRepo, notificationChannelRepo, buildHistoryRepo, minIOClient, buildService, scriptService, agentService)
	cronScheduler.Start(ctx.Context())

	// EMQX broker 认证 webhook(最小 HTTP listener, :9500)
	mqttAuthServer := server.NewMqttAuthServer(ctx, agentNodeRepo)

	// ── register:service ── 新模块服务在此行后注册(make register 工具锚点,勿删)

	// ═══════════════════════ 四、传输层(internal/server) ═══════════════════════

	grpcMiddlewares := server.NewGrpcMiddleware(ctx)

	grpcServer, err := server.NewGrpcServer(ctx, grpcMiddlewares,
		authenticationService, loginPolicyService, userCredentialService,
		taskService,
		dictTypeService, dictEntryService, languageService,
		tenantService, userService, roleService, positionService, orgUnitService,
		menuService, apiService, permissionService, permissionGroupService, permissionAuditLogService, policyEvaluationLogService,
		loginAuditLogService, apiAuditLogService, operationAuditLogService, dataAccessAuditLogService,
		agentNodeService,
		projectService, commandRecordService,
		scriptService,
		repositoryService, buildService, buildHistoryService,
		distributionService,
		monitorService, alertService,
		notificationChannelService,
		approvalService,
		fileService,
		// register:grpc-arg ── 新模块服务实参在此行后追加(make register 工具锚点,勿删)
	)
	if err != nil {
		rollback()
		return nil, nil, err
	}

	asynqServer := server.NewAsynqServer(ctx, taskService)

	// MQTT(agent 信令通道):未配置 server.mqtt 段时为 nil,core 仍可运行
	var servers []kratosTransport.Server
	servers = append(servers, grpcServer, asynqServer)
	if mqttServer := server.NewCoreMqttServer(ctx, agentService); mqttServer != nil {
		servers = append(servers, mqttServer)
	}
	// EMQX broker 认证 webhook(最小 HTTP :9500)
	if mqttAuthServer != nil {
		servers = append(servers, mqttAuthServer)
	}

	return newApp(ctx, servers...), rollback, nil
}
