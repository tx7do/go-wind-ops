// tenant_ctx.go 系统后台路径的租户归因工具。
//
// 背景:ops 业务表已挂 TenantID mixin(go-crud TenantPrivacy)——用户请求
// 路径的 Create 由请求元数据自动落租户;但后台任务(sweep/cron/pipeline
// 引擎/终端录制)运行在无操作者上下文,Create 会落到 tenant 0(平台),
// 租户用户看不到自己项目的告警/终端回放等数据。本工具以父实体的租户
// 构建 UserViewer 上下文,让这些写入归属正确租户;uid=0 保持系统归因。
package service

import (
	"context"

	"github.com/tx7do/go-crud/viewer"

	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	appViewer "go-wind-ops/pkg/entgo/viewer"
)

// tenantScopedCtx 返回带指定租户 viewer 的上下文;tid 为空/0(平台级父实体,
// 如 agent_node)时原样返回,行为与现状一致。
func tenantScopedCtx(ctx context.Context, tid *uint32) context.Context {
	if tid == nil || *tid == 0 {
		return ctx
	}
	return viewer.WithContext(ctx, appViewer.NewUserViewer(0, uint64(*tid), 0, "", identityV1.DataScope_ALL))
}

// viewerTenantOf 读取请求上下文里的租户(无 viewer 或平台视图返回 0)。
func viewerTenantOf(ctx context.Context) uint32 {
	if vc, exist := viewer.FromContext(ctx); exist && vc != nil {
		return uint32(vc.TenantID())
	}
	return 0
}

// uint32Val 可空租户字段取值(nil→0)。
func uint32Val(p *uint32) uint32 {
	if p == nil {
		return 0
	}
	return *p
}
