// audit_chain.go 审计日志哈希链写入器。
//
// 链模型:每条新记录的 prev_log_hash = 当前链尾的 log_hash(按 id 递增),
// 空串 = 链头。prev_log_hash 上的唯一索引使"两条记录指向同一前驱"(分叉)
// 在数据库层结构性不可能;并发写入撞链尾时后到者唯一键冲突,重读链尾重试。
// 效果:链中任何记录的篡改/删除/重排都会让 prev 链条断裂,可被线性核验发现。
//
// 已知边界:链尾(最新一条)被删除后新记录接在新链尾上,尾部删除本身不可检
// (需外部锚定才能覆盖);AUDIT_RETENTION_DAYS 的到期清理删除的是历史区段,
// 核验方从现存最早记录起走链即可(其 prev 指向已清理行属预期)。
//
// log_hash 本身(内容哈希)与 ECDSA 签名(BFF 中间件以 (tenant,user,created_at,
// log_hash) 签发)语义不变——链式化只追加 prev 指针,既有验证不破坏。
package data

import (
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-ops/app/core/service/internal/data/ent"
)

// chainInsertMaxAttempts 链尾竞争重试上限(唯一键冲突后重读链尾再插)。
const chainInsertMaxAttempts = 5

// auditChain 表级链式写入器(每个审计 repo 持有一个;链尾读取由 repo 以
// 各自生成代码实现——six tables 各自的 Query/Select 不经反射)。
type auditChain struct {
	tail func(ctx context.Context) string // 当前链尾的 log_hash(无链尾返回空串)
	log  *log.Helper
	name string // 表语义名,报错定位用
}

// execChained 带链指针插入:每次尝试先读链尾,经 setPrev 注入 prev_log_hash 后
// 执行 exec;唯一键冲突(并发撞同一链尾)则重读重试。全部尝试耗尽返回最后错误。
func (a auditChain) execChained(ctx context.Context, setPrev func(prev string), exec func() error) error {
	var lastErr error
	for attempt := 0; attempt < chainInsertMaxAttempts; attempt++ {
		setPrev(a.tail(ctx))
		if err := exec(); err == nil {
			return nil
		} else if isUniqueViolation(err) {
			// 并发对手拿到了同一链尾:让位重读,重新接链
			a.log.Warnf("audit chain [%s] tail race (attempt %d/%d), re-linking",
				a.name, attempt+1, chainInsertMaxAttempts)
			lastErr = err
			continue
		} else {
			return err
		}
	}
	return lastErr
}

// isUniqueViolation 识别数据库唯一键冲突(ent 统一包装为 ConstraintError)。
// 包级 var 以便单测桩替换(ConstraintError 字段不导出,无法构造真实实例)。
var isUniqueViolation = func(err error) bool {
	if err == nil {
		return false
	}
	var ce *ent.ConstraintError
	return errors.As(err, &ce)
}
