package logging

import (
	"context"
	"crypto/ecdsa"

	"github.com/go-kratos/kratos/v2/log"

	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
)

type WriteApiLogFunc func(ctx context.Context, data *auditV1.ApiAuditLog) error
type WriteLoginLogFunc func(ctx context.Context, data *auditV1.LoginAuditLog) error
type WriteOperationLogFunc func(ctx context.Context, data *auditV1.OperationAuditLog) error

type options struct {
	writeApiLogFunc   WriteApiLogFunc   // 写入API审计日志函数
	writeLoginLogFunc WriteLoginLogFunc // 写入登录审计日志函数

	writeOperationLogFunc WriteOperationLogFunc // 写入操作审计日志函数

	loginOperation  string // 登录操作名称
	logoutOperation string // 登出操作名称

	ecPrivateKey *ecdsa.PrivateKey // 私钥(审计签名;未注入时按 AUDIT_SIGN_KEY_FILE 持久化加载/生成)
	ecPublicKey  *ecdsa.PublicKey  // 公钥(可公开)

	logger *log.Helper // 审计写入失败等内部告警(nil=静默)
}

type Option func(*options)

func WithWriteApiLogFunc(fnc WriteApiLogFunc) Option {
	return func(opts *options) {
		opts.writeApiLogFunc = fnc
	}
}

func WithWriteLoginLogFunc(fnc WriteLoginLogFunc) Option {
	return func(opts *options) {
		opts.writeLoginLogFunc = fnc
	}
}

func WithWriteOperationLogFunc(fnc WriteOperationLogFunc) Option {
	return func(opts *options) {
		opts.writeOperationLogFunc = fnc
	}
}

func WithLoginOperation(operation string) Option {
	return func(opts *options) {
		opts.loginOperation = operation
	}
}

func WithLogoutOperation(operation string) Option {
	return func(opts *options) {
		opts.logoutOperation = operation
	}
}

func WithECPrivateKey(key *ecdsa.PrivateKey) Option {
	return func(opts *options) {
		opts.ecPrivateKey = key
	}
}

func WithECPublicKey(key *ecdsa.PublicKey) Option {
	return func(opts *options) {
		opts.ecPublicKey = key
	}
}

// WithLogger 注入内部告警日志器(审计写入失败时使用,不注入则静默)。
func WithLogger(l *log.Helper) Option {
	return func(opts *options) {
		opts.logger = l
	}
}
