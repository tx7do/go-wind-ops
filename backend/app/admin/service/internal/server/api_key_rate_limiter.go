// api_key_rate_limiter.go OpenAPI 网关的按 key 限流(滑动窗口计数)。
// 背景:API key 此前无任何限流,持有有效 key 可无节制刷触发端点。
// 进程内实现(单 key 计数不跨副本共享;多副本部署时每副本独立限额,
// 总限额=副本数×限额,可接受——限流目标是防刷而非精确配额)。
package server

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// apiKeyRateLimitEnv 每分钟每 key 触发上限的环境变量名;<=0 = 不限流。
const apiKeyRateLimitEnv = "OPENAPI_RATE_LIMIT"

type apiKeyWindow struct {
	start time.Time
	count int
}

type apiKeyRateLimiter struct {
	limit  int           // 窗口内允许次数;<=0 = 不限流
	window time.Duration // 窗口长度

	mu      sync.Mutex
	windows map[string]*apiKeyWindow
	lastGC  time.Time
}

// newAPIKeyRateLimiter 从 OPENAPI_RATE_LIMIT 读取限额(次/分钟);未配置
// 默认 60,显式配置为 0 或负值则不限流。
func newAPIKeyRateLimiter() *apiKeyRateLimiter {
	limit := 60
	if v := strings.TrimSpace(os.Getenv(apiKeyRateLimitEnv)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return &apiKeyRateLimiter{
		limit:   limit,
		window:  time.Minute,
		windows: make(map[string]*apiKeyWindow),
	}
}

// Allow 报告该 key 在当前窗口内是否仍可触发。
func (l *apiKeyRateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 || key == "" {
		return true
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// 惰性清理过期窗口,防长尾 key 撑大映射
	if now.Sub(l.lastGC) >= l.window {
		for k, w := range l.windows {
			if now.Sub(w.start) >= l.window {
				delete(l.windows, k)
			}
		}
		l.lastGC = now
	}

	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.windows[key] = &apiKeyWindow{start: now, count: 1}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}
