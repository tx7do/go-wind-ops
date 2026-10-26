package data

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

)

// errUnique 模拟唯一键冲突的哨兵错误(经桩 isUniqueViolation 识别)。
var errUnique = errors.New("unique violation (fake)")

// nopLogger 测试用静默 logger。
type nopLogger struct{}

func (nopLogger) Log(_ log.Level, _ ...any) error { return nil }

// stubUniqueViolation 把唯一键冲突判定桩到哨兵错误,返回恢复函数。
func stubUniqueViolation(t *testing.T) {
	t.Helper()
	orig := isUniqueViolation
	isUniqueViolation = func(err error) bool { return errors.Is(err, errUnique) }
	t.Cleanup(func() { isUniqueViolation = orig })
}

// TestAuditChainExecChained 链式写入器语义:链尾注入/冲突重试/非冲突直返。
func TestAuditChainExecChained(t *testing.T) {
	logger := log.NewHelper(nopLogger{})
	ctx := context.Background()

	stubUniqueViolation(t)
	t.Run("无冲突:注入链尾并恰好执行一次", func(t *testing.T) {
		tail := "tail-hash-1"
		chain := auditChain{tail: func(context.Context) string { return tail }, log: logger, name: "t"}
		var injected []string
		execCount := 0
		err := chain.execChained(ctx,
			func(prev string) { injected = append(injected, prev) },
			func() error { execCount++; return nil })
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if execCount != 1 || len(injected) != 1 || injected[0] != "tail-hash-1" {
			t.Fatalf("exec=%d injected=%v", execCount, injected)
		}
	})

	stubUniqueViolation(t)
	t.Run("链尾竞争:冲突后重读链尾重试并重新接链", func(t *testing.T) {
		tails := []string{"tail-a", "tail-b"} // 第一次读 tail-a,冲突后读到 tail-b
		calls := 0
		chain := auditChain{tail: func(context.Context) string {
			calls++
			return tails[min(calls-1, len(tails)-1)]
		}, log: logger, name: "t"}
		var injected []string
		execCount := 0
		err := chain.execChained(ctx,
			func(prev string) { injected = append(injected, prev) },
			func() error {
				execCount++
				if execCount == 1 {
					return errUnique // 第一插撞链
				}
				return nil
			})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if execCount != 2 || len(injected) != 2 || injected[0] != "tail-a" || injected[1] != "tail-b" {
			t.Fatalf("exec=%d injected=%v", execCount, injected)
		}
	})

	stubUniqueViolation(t)
	t.Run("持续竞争:耗尽重试返回最后冲突错误", func(t *testing.T) {
		chain := auditChain{tail: func(context.Context) string { return "x" }, log: logger, name: "t"}
		execCount := 0
		err := chain.execChained(ctx,
			func(string) {},
			func() error { execCount++; return errUnique })
		if err == nil {
			t.Fatal("expected error after exhausting attempts")
		}
		if execCount != chainInsertMaxAttempts {
			t.Fatalf("exec=%d want %d", execCount, chainInsertMaxAttempts)
		}
	})

	t.Run("非冲突错误:立即返回不重试", func(t *testing.T) {
		chain := auditChain{tail: func(context.Context) string { return "" }, log: logger, name: "t"}
		boom := errors.New("db down")
		execCount := 0
		err := chain.execChained(ctx, func(string) {}, func() error { execCount++; return boom })
		if !errors.Is(err, boom) {
			t.Fatalf("want original error, got %v", err)
		}
		if execCount != 1 {
			t.Fatalf("exec=%d want 1", execCount)
		}
	})

	t.Run("空链尾即链头语义", func(t *testing.T) {
		chain := auditChain{tail: func(context.Context) string { return "" }, log: logger, name: "t"}
		var head string
		_ = chain.execChained(ctx, func(prev string) { head = prev }, func() error { return nil })
		if head != "" || !strings.Contains("", head) {
			t.Fatalf("head prev = %q, want empty string", head)
		}
	})
}
