// audit_write.go 审计写入的操作者元数据选择。
//
// 此前三个审计写闭包一律以零值系统操作者(metadata.NewUserOperator(0,0,0,ALL))
// 调 core Create:租户隐私策略按 viewer 租户填 tenant_id,导致操作审计恒落
// tenant 0,租户管理员按租户过滤看不到自己租户的操作日志。
// 现优先复用 auth 中间件注入 transport 请求头的真实操作者(含租户)——审计
// 随操作者租户落库;白名单/未认证路径(登录失败等)无操作者,回落零值系统
// 操作者,保证审计写入永不因缺身份而失败。
package server

import (
	"context"

	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"

	"go-wind-ops/pkg/metadata"
)

func auditWriteOperator(ctx context.Context) *authenticationV1.OperatorMetadata {
	if op, err := metadata.OperatorFromTransportHeader(ctx); err == nil && op != nil {
		return op
	}
	return metadata.NewUserOperator(0, 0, 0, identityV1.DataScope_ALL)
}
