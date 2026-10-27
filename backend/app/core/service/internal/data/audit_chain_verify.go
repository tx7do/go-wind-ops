// audit_chain_verify.go 审计哈希链完整性核验(数据层)。
//
// 与 tools/audit-verify CLI 同一套链语义:按 id 序,prev_log_hash 必须等于
// 前一行 log_hash(空串=链头,NULL prev 为链启用前历史跳过)。此处为 cron
// 自检用的轻量 SQL 版——只判链,不验 ECDSA 签名(验签需要私钥公钥与两代
// 签名内容复现,属离线深检,仍在 CLI);断链即安全事件,告警不可被维护
// 窗口静默。
package data

import (
	"context"
	"database/sql"
	"fmt"
)

// auditChainTables 核验目标(与 tools/audit-verify 保持一致)。
var auditChainTables = []string{
	"sys_api_audit_logs",
	"sys_login_audit_logs",
	"sys_operation_audit_logs",
	"sys_data_access_audit_logs",
	"sys_permission_audit_logs",
	"sys_policy_evaluation_logs",
}

// ChainIssue 单表核验结果。
type ChainIssue struct {
	Table      string
	Total      int
	Legacy     int    // prev 为 NULL:链启用前历史
	BrokenIDs  []int64
	BrokenHead string // 首个断点期望值(排查线索)
}

// Broken 有断链。
func (c ChainIssue) Broken() bool { return len(c.BrokenIDs) > 0 }

// VerifyAuditChains 走全部审计表哈希链;缺表跳过(部分部署未挂全部审计面)。
func VerifyAuditChains(ctx context.Context, db *sql.DB) ([]ChainIssue, error) {
	var out []ChainIssue
	for _, table := range auditChainTables {
		issue, err := verifyAuditChainTable(ctx, db, table)
		if err != nil {
			if isMissingTable(err) {
				continue
			}
			return nil, fmt.Errorf("verify %s: %w", table, err)
		}
		out = append(out, issue)
	}
	return out, nil
}

func verifyAuditChainTable(ctx context.Context, db *sql.DB, table string) (ChainIssue, error) {
	issue := ChainIssue{Table: table}

	rows, err := db.QueryContext(ctx, `SELECT id, log_hash, prev_log_hash FROM ` + table + ` ORDER BY id`)
	if err != nil {
		return issue, err
	}
	defer func() { _ = rows.Close() }()

	var prevHash *string
	for rows.Next() {
		var id int64
		var logHash, prevLogHash *string
		if err = rows.Scan(&id, &logHash, &prevLogHash); err != nil {
			return issue, err
		}
		issue.Total++

		if prevLogHash == nil {
			issue.Legacy++
		} else {
			expected := ""
			if prevHash != nil {
				expected = *prevHash
			}
			if *prevLogHash != expected && len(issue.BrokenIDs) == 0 {
				issue.BrokenHead = expected
			}
			if *prevLogHash != expected {
				issue.BrokenIDs = append(issue.BrokenIDs, id)
			}
		}
		prevHash = logHash
	}
	return issue, rows.Err()
}

// isMissingTable PG 42P01。
func isMissingTable(err error) bool {
	if err == nil {
		return false
	}
	type coder interface{ SQLState() string }
	if c, ok := err.(coder); ok {
		return c.SQLState() == "42P01"
	}
	return false
}
