package constants

const (
	// SystemPermissionCodePrefix 系统权限代码前缀
	SystemPermissionCodePrefix = "sys:"

	// SystemAccessBackendPermissionCode 系统访问后台权限代码
	SystemAccessBackendPermissionCode = SystemPermissionCodePrefix + "access_backend"

	// SystemAccessAppPermissionCode 系统访问应用端(C 端)权限代码
	SystemAccessAppPermissionCode = SystemPermissionCodePrefix + "access_app"

	// SystemManageTenantsPermissionCode 系统管理租户权限代码
	SystemManageTenantsPermissionCode = SystemPermissionCodePrefix + "manage_tenants"

	// SystemAuditLogsPermissionCode 系统审计日志权限代码
	SystemAuditLogsPermissionCode = SystemPermissionCodePrefix + "audit_logs"

	// SystemPlatformAdminPermissionCode 系统平台管理员权限代码
	SystemPlatformAdminPermissionCode = SystemPermissionCodePrefix + "platform_admin"
	// SystemTenantManagerPermissionCode 系统租户管理员权限代码
	SystemTenantManagerPermissionCode = SystemPermissionCodePrefix + "tenant_manager"

	// OpsPermissionCodePrefix 运维域权限代码前缀
	OpsPermissionCodePrefix = "ops:"

	// OpsNodeManagePermissionCode 节点管理权限
	OpsNodeManagePermissionCode = OpsPermissionCodePrefix + "node_manage"
	// OpsProjectManagePermissionCode 项目管理权限
	OpsProjectManagePermissionCode = OpsPermissionCodePrefix + "project_manage"
	// OpsProjectDeployPermissionCode 项目分发部署权限(含启动/停止/重启)
	OpsProjectDeployPermissionCode = OpsPermissionCodePrefix + "project_deploy"
	// OpsScriptManagePermissionCode 脚本管理权限
	OpsScriptManagePermissionCode = OpsPermissionCodePrefix + "script_manage"
	// OpsScriptExecutePermissionCode 脚本执行权限
	OpsScriptExecutePermissionCode = OpsPermissionCodePrefix + "script_execute"
	// OpsBuildManagePermissionCode 构建管理权限
	OpsBuildManagePermissionCode = OpsPermissionCodePrefix + "build_manage"
	// OpsBuildTriggerPermissionCode 构建触发权限
	OpsBuildTriggerPermissionCode = OpsPermissionCodePrefix + "build_trigger"
	// OpsDistributePermissionCode 分发权限
	OpsDistributePermissionCode = OpsPermissionCodePrefix + "distribute"

	// SystemPermissionModule 系统权限模块标识
	SystemPermissionModule = "sys"

	// OpsPermissionModule 运维权限模块标识
	OpsPermissionModule = "ops"

	// DefaultBizPermissionModule 业务权限模块标识
	DefaultBizPermissionModule = "biz"

	// UncategorizedPermissionGroup 未分类权限组标识
	UncategorizedPermissionGroup = "uncategorized"
)

// ProtectedPermissionCodes 受保护的权限代码列表，禁止删除
var ProtectedPermissionCodes = []string{
	SystemAccessBackendPermissionCode,
	SystemAccessAppPermissionCode,
	SystemManageTenantsPermissionCode,
	SystemAuditLogsPermissionCode,
	SystemPlatformAdminPermissionCode,
	SystemTenantManagerPermissionCode,
}
