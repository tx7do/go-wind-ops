// Package rpcsecret 提供 BFF(admin/front) ↔ core 之间 gRPC 内部调用的
// 共享密钥校验。
//
// 背景:core gRPC 长期"零认证信任岛"——直接采信入站 x-md-global-operator
// 元数据构建租户上下文,一旦端口被内网直达,攻击者可冒充任意用户/租户,
// 绕过 BFF 全部权限码与审批闸门直接调用执行面。本包以对称共享密钥把
// "网络可达"收窄为"持有密钥":
//
//   - 服务端 Server(secret):secret 非空时强制校验请求头,缺失/不符一律
//     Unauthorized(fail-closed);secret 为空时跳过并建议仅用于开发。
//   - 客户端 Client(secret):secret 非空时自动附加请求头;为空时空转。
//
// 密钥经 CORE_RPC_SECRET 环境变量下发,core 与两个 BFF 必须一致;compose
// 部署已默认装配,生产务必换掉默认值。
package rpcsecret

import (
	"context"
	"crypto/subtle"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// EnvKey 共享密钥的环境变量名。
const EnvKey = "CORE_RPC_SECRET"

// HeaderKey 密钥随 gRPC 元数据传输的请求头名。
const HeaderKey = "x-rpc-secret"

// SecretFromEnv 读取共享密钥(去除首尾空白);未配置返回空串。
func SecretFromEnv() string {
	return strings.TrimSpace(os.Getenv(EnvKey))
}

// Server 服务端中间件:secret 非空时校验请求头(常时比较),失败返回
// 401;secret 为空时直接放行(未启用)。
func Server(secret string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if secret == "" {
				return handler(ctx, req)
			}
			got := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				got = tr.RequestHeader().Get(HeaderKey)
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
				return nil, errors.Unauthorized("RPC_SECRET", "missing or invalid internal rpc secret")
			}
			return handler(ctx, req)
		}
	}
}

// Client 客户端中间件:secret 非空时为每个请求附加共享密钥头。
func Client(secret string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if secret != "" {
				if tr, ok := transport.FromClientContext(ctx); ok {
					tr.RequestHeader().Set(HeaderKey, secret)
				}
			}
			return handler(ctx, req)
		}
	}
}
