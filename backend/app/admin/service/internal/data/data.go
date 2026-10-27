package data

import (
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"


	"github.com/go-kratos/kratos/v2/registry"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	redisClient "github.com/tx7do/kratos-bootstrap/cache/redis"
	bRegistry "github.com/tx7do/kratos-bootstrap/registry"
	"github.com/tx7do/kratos-bootstrap/rpc"

	"github.com/tx7do/go-utils/captcha"

	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	dictV1 "go-wind-ops/api/gen/go/dict/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	taskV1 "go-wind-ops/api/gen/go/task/service/v1"

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
	return authenticationV1.ClientType_admin
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(ctx *bootstrap.Context) (*redis.Client, func(), error) {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, func() {}, nil
	}

	l := ctx.NewLoggerHelper("redis/data/admin-service")

	cli := redisClient.NewClient(cfg.Data, l)

	return cli, func() {
		if err := cli.Close(); err != nil {
			l.Error(err)
		}
	}, nil
}

func NewCaptcha(rdb *redis.Client) *captcha.Captcha {
	captchaInstance := captcha.NewCaptcha(rdb,
		captcha.WithDriverType(captcha.DriverString),
		captcha.WithExpire(10*time.Minute),
		captcha.WithKeyPrefix(serviceid.ProjectName+":captcha"),
		captcha.WithStringCount(6),
		captcha.WithStringSource("ABCDEFGHJKLMNPQRSTUVWXYZ23456789"),
	)
	return captchaInstance
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

func NewAuthenticationServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.AuthenticationServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return authenticationV1.NewAuthenticationServiceClient(cli)
}

func NewMaintenanceWindowServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.MaintenanceWindowServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewMaintenanceWindowServiceClient(cli)
}

func NewMfaServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.MfaServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return authenticationV1.NewMfaServiceClient(cli)
}

func NewUserCredentialServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.UserCredentialServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return authenticationV1.NewUserCredentialServiceClient(cli)
}

func NewLoginPolicyServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.LoginPolicyServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return authenticationV1.NewLoginPolicyServiceClient(cli)
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

func NewPermissionGroupServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.PermissionGroupServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return permissionV1.NewPermissionGroupServiceClient(cli)
}

func NewPermissionServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.PermissionServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return permissionV1.NewPermissionServiceClient(cli)
}

func NewApiServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.ApiServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return permissionV1.NewApiServiceClient(cli)
}

func NewMenuServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.MenuServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return permissionV1.NewMenuServiceClient(cli)
}

func NewPermissionAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.PermissionAuditLogServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return auditV1.NewPermissionAuditLogServiceClient(cli)
}

func NewPolicyEvaluationLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.PolicyEvaluationLogServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return permissionV1.NewPolicyEvaluationLogServiceClient(cli)
}

func NewApiAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.ApiAuditLogServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return auditV1.NewApiAuditLogServiceClient(cli)
}

func NewDataAccessAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.DataAccessAuditLogServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return auditV1.NewDataAccessAuditLogServiceClient(cli)
}

func NewLoginAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.LoginAuditLogServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return auditV1.NewLoginAuditLogServiceClient(cli)
}

func NewOperationAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.OperationAuditLogServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return auditV1.NewOperationAuditLogServiceClient(cli)
}

func NewDictEntryServiceClient(ctx *bootstrap.Context, r registry.Discovery) dictV1.DictEntryServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return dictV1.NewDictEntryServiceClient(cli)
}

func NewDictTypeServiceClient(ctx *bootstrap.Context, r registry.Discovery) dictV1.DictTypeServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return dictV1.NewDictTypeServiceClient(cli)
}

func NewLanguageServiceClient(ctx *bootstrap.Context, r registry.Discovery) dictV1.LanguageServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return dictV1.NewLanguageServiceClient(cli)
}

func NewTaskServiceClient(ctx *bootstrap.Context, r registry.Discovery) taskV1.TaskServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return taskV1.NewTaskServiceClient(cli)
}

func NewAgentNodeServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.AgentNodeServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewAgentNodeServiceClient(cli)
}

func NewProjectServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.ProjectServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewProjectServiceClient(cli)
}

func NewCommandRecordServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.CommandRecordServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewCommandRecordServiceClient(cli)
}

func NewTerminalServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.TerminalServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewTerminalServiceClient(cli)
}

func NewPipelineServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.PipelineServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewPipelineServiceClient(cli)
}

func NewApiKeyServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.ApiKeyServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewApiKeyServiceClient(cli)
}

func NewCloudAccountServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.CloudAccountServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewCloudAccountServiceClient(cli)
}

func NewCloudInstanceServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.CloudInstanceServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewCloudInstanceServiceClient(cli)
}

func NewCloudCostServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.CloudCostServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewCloudCostServiceClient(cli)
}

func NewPluginServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.PluginServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewPluginServiceClient(cli)
}

func NewMonitorServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.MonitorServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewMonitorServiceClient(cli)
}

func NewAlertServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.AlertServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewAlertServiceClient(cli)
}

func NewCloudBudgetServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.CloudBudgetServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewCloudBudgetServiceClient(cli)
}

func NewCertServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.CertServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewCertServiceClient(cli)
}

func NewNotificationDeliveryServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.NotificationDeliveryServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewNotificationDeliveryServiceClient(cli)
}

func NewTerminalAuditServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.TerminalServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewTerminalServiceClient(cli)
}

func NewInspectionServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.InspectionServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewInspectionServiceClient(cli)
}

func NewLogSnapshotServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.LogSnapshotServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewLogSnapshotServiceClient(cli)
}

func NewConfigServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.ConfigServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewConfigServiceClient(cli)
}

func NewScriptBatchServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.ScriptBatchServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewScriptBatchServiceClient(cli)
}

func NewBackupServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.BackupServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewBackupServiceClient(cli)
}

func NewApprovalServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.ApprovalServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewApprovalServiceClient(cli)
}

func NewNotificationChannelServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.NotificationChannelServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewNotificationChannelServiceClient(cli)
}

func NewFileServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.FileServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewFileServiceClient(cli)
}

func NewScriptServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.ScriptServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewScriptServiceClient(cli)
}

func NewRepositoryServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.RepositoryServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewRepositoryServiceClient(cli)
}

func NewBuildServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.BuildServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewBuildServiceClient(cli)
}

func NewBuildHistoryServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.BuildHistoryServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewBuildHistoryServiceClient(cli)
}

func NewDistributionServiceClient(ctx *bootstrap.Context, r registry.Discovery) opsV1.DistributionServiceClient {
	cli, err := newCoreGrpcConn(ctx, r)
	if err != nil {
		return nil
	}

	return opsV1.NewDistributionServiceClient(cli)
}
