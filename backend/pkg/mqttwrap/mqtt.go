// Package mqttwrap 为 MQTT 传输服务提供带重试的启动包装。
//
// 背景:tx7do transport/mqtt 的 Server.Start 对初始连接失败零重试,且失败
// 状态缓存在私有字段 s.err 里、只能在成功启动后的 Stop 里复位——失败的
// 实例无法原地重试。kratos App 把 Start 错误视为致命并整体中止,于是:
// 冷启动时 core 先于 EMQX 认证回调资源(指向 core :9500,自带健康检查周期)
// 恢复的首连会被拒,core 随即整体退出;agent 在 EMQX 未就绪时同样直接死。
//
// 因此本包装按"工厂"模式工作:每次尝试用工厂重建内部实例再 Start,失败
// 则丢弃换新实例重试,直到成功或超时。两端共用。
package mqttwrap

import (
	"context"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

// Options 重试参数。
type Options struct {
	// RetryInterval 首连失败后的重试间隔(默认 3s)。
	RetryInterval time.Duration
	// MaxWait 放弃前的最长等待(默认 2m;超时后返回最后一次错误)。
	MaxWait time.Duration
}

func (o *Options) applyDefaults() {
	if o.RetryInterval <= 0 {
		o.RetryInterval = 3 * time.Second
	}
	if o.MaxWait <= 0 {
		o.MaxWait = 2 * time.Minute
	}
}

// Factory 每次尝试构造一个全新的内部服务器实例。
type Factory func() (transport.Server, error)

type resilientServer struct {
	factory Factory
	opts    Options
	log     *log.Helper

	mu    sync.Mutex
	inner transport.Server
}

// WrapFactory 返回带首连重试的服务器包装。
func WrapFactory(factory Factory, logger *log.Helper, opts Options) transport.Server {
	opts.applyDefaults()
	return &resilientServer{factory: factory, opts: opts, log: logger}
}

func (s *resilientServer) Start(ctx context.Context) error {
	deadline := time.Now().Add(s.opts.MaxWait)
	var lastErr error
	for attempt := 1; ; attempt++ {
		inner, err := s.factory()
		if err == nil {
			err = inner.Start(ctx)
		}
		if err == nil {
			s.mu.Lock()
			s.inner = inner
			s.mu.Unlock()
			return nil
		}
		lastErr = err
		if inner != nil {
			// 丢弃失败实例(其内部 s.err 已固化,不可复用)
			_ = inner.Stop(ctx)
		}
		if time.Now().After(deadline) {
			s.log.Errorf("mqtt server start failed after %d attempt(s): %v", attempt, lastErr)
			return lastErr
		}
		s.log.Warnf("mqtt server start failed (attempt %d): %v — retry in %s", attempt, lastErr, s.opts.RetryInterval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.opts.RetryInterval):
		}
	}
}

func (s *resilientServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	inner := s.inner
	s.inner = nil
	s.mu.Unlock()
	if inner == nil {
		return nil
	}
	return inner.Stop(ctx)
}
