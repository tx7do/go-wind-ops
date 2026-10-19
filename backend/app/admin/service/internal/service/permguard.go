// permguard.go 触发类/敏感入口的显式权限码门。
//
// 全站 authz 引擎为 noop(admin/front data.NewAuthorizer()→noop.State{},
// 角色→API 策略无数据通道),中间件层鉴权空转——BFF 门面内的权限码点查是
// 当前唯一真实防线。码表与前端路由 authority 对齐,且限用已播种的权限码
// (ops:node_manage/project_manage/script_manage/build_manage/distribute +
// sys 受保护码)。
//
// 守卫经 wiring 启动期一次性注入(SetPermissionGuard,先于任何 server 启动),
// 避免十余个门面构造器逐一透传两个客户端。
package service

import (
	"context"
	"sync"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"

	"go-wind-ops/app/admin/service/internal/authzcheck"
	"go-wind-ops/pkg/middleware/auth"
)

type PermissionGuard struct {
	roleClient permissionV1.RoleServiceClient
	permClient permissionV1.PermissionServiceClient
}

func NewPermissionGuard(roleClient permissionV1.RoleServiceClient, permClient permissionV1.PermissionServiceClient) *PermissionGuard {
	return &PermissionGuard{roleClient: roleClient, permClient: permClient}
}

var (
	permGuardMu  sync.RWMutex
	appPermGuard *PermissionGuard
)

// SetPermissionGuard wiring 启动期注入(必须先于 REST server 开始服务)。
func SetPermissionGuard(g *PermissionGuard) {
	permGuardMu.Lock()
	defer permGuardMu.Unlock()
	appPermGuard = g
}

func currentPermissionGuard() *PermissionGuard {
	permGuardMu.RLock()
	defer permGuardMu.RUnlock()
	return appPermGuard
}

// requirePermission 校验当前操作者持有指定权限码;未持有返回 403。
func requirePermission(ctx context.Context, code string) error {
	g := currentPermissionGuard()
	if g == nil {
		// fail-closed:守卫未装配时拒绝敏感操作,而不是放行
		return adminV1.ErrorForbidden("permission guard not configured")
	}
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return err
	}
	allowed, err := authzcheck.HasPermissionCode(ctx, g.roleClient, g.permClient, operator.GetUserId(), code)
	if err != nil {
		return adminV1.ErrorInternalServerError("authorization check failed")
	}
	if !allowed {
		return adminV1.ErrorForbidden("%s", "missing permission "+code)
	}
	return nil
}
