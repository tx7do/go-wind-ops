// Package authzcheck 显式权限码校验:绕过 authz 中间件的入口(WebSocket
// 握手等)以"用户→角色→权限点→权限码"链路做点查。全站 authz 引擎当前
// 为 noop,本包是敏感入口的真实防线;权限码与前端路由 authority 对齐。
package authzcheck

import (
	"context"

	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"
)

// HasPermissionCode 判定用户是否持有指定权限码。查询失败返回错误(调用方
// 按 deny 处理),未持有返回 false。
func HasPermissionCode(
	ctx context.Context,
	roleClient permissionV1.RoleServiceClient,
	permClient permissionV1.PermissionServiceClient,
	userID uint32,
	code string,
) (bool, error) {
	permIds, err := roleClient.ListPermissionIds(ctx, &permissionV1.ListPermissionIdsRequest{
		QueryBy: &permissionV1.ListPermissionIdsRequest_UserId{UserId: userID},
	})
	if err != nil {
		return false, err
	}
	if len(permIds.GetPermissionIds()) == 0 {
		return false, nil
	}
	codes, err := permClient.ListPermissionCodesByIds(ctx, &permissionV1.ListPermissionCodesByIdsRequest{
		PermissionIds: permIds.GetPermissionIds(),
	})
	if err != nil {
		return false, err
	}
	for _, c := range codes.GetPermissionCodes() {
		if c == code {
			return true, nil
		}
	}
	return false, nil
}
