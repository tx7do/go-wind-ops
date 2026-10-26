// Package logalock 登录失败锁定(防爆破):同 用户名+来源IP 连续失败
// MaxFailures 次后锁定 LockMinutes 分钟(Redis 计数)。
//
// 复合键而非纯用户名:防止任意攻击者对已知用户名故意失败几次即可把该
// 账号全平台锁死(账号 DoS);按 IP 隔离后,攻击者只能锁死自己来源 IP 的
// 登录,跨 IP 爆破另受验证码等前置约束。admin/front 两个 BFF 共用。
package logalock

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	ktransport "github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/redis/go-redis/v9"
)

const (
	// MaxFailures 锁定阈值(连续失败次数)。
	MaxFailures = 5
	// LockMinutes 锁定时长(分钟)。
	LockMinutes = 15
)

const keyPrefix = "go-wind-ops:login:fail:"

// TooManyAttempts 锁定命中的标准错误(reason 契约,前端可据此提示)。
func TooManyAttempts() *errors.Error {
	return errors.New(429, "TOO_MANY_REQUESTS", "too many failed logins, retry later")
}

// Locker 登录失败锁定器(依赖 Redis 计数;nil client 时全部放行)。
type Locker struct {
	redis *redis.Client
	log   *log.Helper
}

// New 构造锁定器。
func New(redisClient *redis.Client, logger *log.Helper) *Locker {
	return &Locker{redis: redisClient, log: logger}
}

func failKey(username, ip string) string {
	return keyPrefix + strings.ToLower(username) + ":" + ip
}

// ClientIP 提取登录来源 IP:优先 X-Forwarded-For 首跳(反代部署),
// 其次 X-Real-IP,兜底 RemoteAddr。
func ClientIP(ctx context.Context) string {
	tr, ok := ktransport.FromServerContext(ctx)
	if !ok {
		return "unknown"
	}
	htr, hok := tr.(*khttp.Transport)
	if !hok || htr.Request() == nil {
		return "unknown"
	}
	r := htr.Request()
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if xff = strings.TrimSpace(xff); xff != "" {
			return xff
		}
	}
	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		return rip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// Check 登录前检查:命中锁定返回 429 TOO_MANY_REQUESTS。
func (l *Locker) Check(ctx context.Context, username string) error {
	if l == nil || l.redis == nil {
		return nil
	}
	key := failKey(username, ClientIP(ctx))
	if n, err := l.redis.Get(ctx, key).Int(); err == nil && n >= MaxFailures {
		return TooManyAttempts()
	}
	return nil
}

// RecordFailure 登录失败计数(到达阈值即进入锁定窗口)。
func (l *Locker) RecordFailure(ctx context.Context, username string) {
	if l == nil || l.redis == nil {
		return
	}
	key := failKey(username, ClientIP(ctx))
	pipe := l.redis.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, LockMinutes*time.Minute)
	if _, err := pipe.Exec(ctx); err != nil && l.log != nil {
		l.log.Warnf("record login failure: %v", err)
	}
}

// Clear 登录成功清零。
func (l *Locker) Clear(ctx context.Context, username string) {
	if l == nil || l.redis == nil {
		return
	}
	if err := l.redis.Del(ctx, failKey(username, ClientIP(ctx))).Err(); err != nil && l.log != nil {
		l.log.Warnf("clear login failures: %v", err)
	}
}
