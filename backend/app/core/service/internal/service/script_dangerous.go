// script_dangerous.go 脚本/即席命令的危险模式识别:命中即要求调用方显式
// 确认(confirm_dangerous)才放行下发。这是"防手滑"门槛而非安全边界——
// 脚本本就是任意执行能力,拦截清单只覆盖不可逆的灾难性操作;运维平台
// 不做、也做不全命令语义审查。误报的代价只是多一次确认点击。
package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// DangerousCommandReason 拦截错误专用 reason:前端按 reason 识别(错误对象的
// message 会被 i18n 按 reason 覆盖,message 不可作契约)。前端 i18n 提供
// request.reason.DANGEROUS_COMMAND_CONFIRM 文案,onError 判定 reason 后弹
// 二次确认框并携带 confirm_dangerous=true 重发。
// 契约常量,改动需同步前端。
const DangerousCommandReason = "DANGEROUS_COMMAND_CONFIRM"

// DangerousCommandMarker 下发被拦截时的错误文案前缀(日志审计用)。
const DangerousCommandMarker = "dangerous command confirmation required"

// dangerousPattern 危险模式:忽略大小写的正则 + 人类可读名称(展示给确认框)。
type dangerousPattern struct {
	name    string
	matched *regexp.Regexp
}

// dangerousPatterns 灾难性操作清单。原则:只收录"根目录/整机/盘级不可逆
// 破坏",常规 rm/mv/服务操作不入列,控制误报。
var dangerousPatterns = []dangerousPattern{
	// ── Unix:根目录/块设备/整机 ──
	{name: "rm 递归根目录", matched: regexp.MustCompile(`(?i)\brm\s+(?:-\w+\s+)*(?:\S+\s+)*"?/(\s|\*|$)`)},
	{name: "mkfs 格式化文件系统", matched: regexp.MustCompile(`(?i)\bmkfs(?:\.[a-z0-9]+)?\b`)},
	{name: "dd 覆写块设备", matched: regexp.MustCompile(`(?i)\bdd\b[^|;&]*\bof=/dev/(?:sd|vd|nvme|hd|xvd|disk|mapper)`)},
	{name: "重定向覆写块设备", matched: regexp.MustCompile(`(?i)>\s*/dev/(?:sd|vd|nvme|hd|xvd)[a-z]`)},
	{name: "chmod 777 根目录", matched: regexp.MustCompile(`(?i)\bchmod\b[^|;&]*\s+777\s+/?(\s|\*|$)`)},
	{name: "关机 (shutdown/halt/poweroff)", matched: regexp.MustCompile(`(?i)\b(?:shutdown|halt|poweroff)\b`)},
	{name: "重启 (reboot)", matched: regexp.MustCompile(`(?i)\breboot\b`)},

	// ── Windows:盘级 ──
	{name: "format 格式化磁盘", matched: regexp.MustCompile(`(?i)\bformat(\.com)?\s+[a-z]:`)},
	{name: "del/rd 递归删盘根", matched: regexp.MustCompile(`(?i)\b(?:del|erase|rd|rmdir)\s+(?:/\w\s+)*"?[a-z]:\\"?\s*$`)},
	{name: "diskpart 磁盘分区操作", matched: regexp.MustCompile(`(?i)\bdiskpart\b`)},
	{name: "PowerShell 递归删盘根", matched: regexp.MustCompile(`(?i)\b(?:remove-item|ri|rm)\b[^;\n|]*\s-(?:r\b|recursive\b|recurse\b)[^;\n|]*\s"?[a-z]:\\"?\s*$`)},

	// ── 跨平台 ──
	{name: "fork 炸弹", matched: regexp.MustCompile(`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`)},
}

// findDangerousPatterns 返回内容命中的危险模式名称列表(去重、保序)。
func findDangerousPatterns(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	var hits []string
	seen := make(map[string]struct{}, len(dangerousPatterns))
	for _, p := range dangerousPatterns {
		if !p.matched.MatchString(content) {
			continue
		}
		if _, ok := seen[p.name]; ok {
			continue
		}
		seen[p.name] = struct{}{}
		hits = append(hits, p.name)
	}
	return hits
}

// guardDangerousContent 统一确认门槛:命中危险模式且未确认时返回带模式
// 清单的 400 错误(文案前缀见 DangerousCommandMarker);已确认则打审计日志。
// 返回 true 表示已放行(无命中或已确认)。
func guardDangerousContent(log *log.Helper, content string, confirmed bool, operatorID uint32) (bool, []string) {
	patterns := findDangerousPatterns(content)
	if len(patterns) == 0 {
		return true, nil
	}
	if !confirmed {
		return false, patterns
	}
	log.Warnf("DANGEROUS command confirmed and dispatched (operator=%d patterns=[%s])",
		operatorID, strings.Join(patterns, "; "))
	return true, patterns
}

// dangerousCommandError 构造"需要危险命令确认"的 403 错误(reason 专用,
// 供前端识别);message 携带命中模式名,进日志审计。
func dangerousCommandError(patterns []string) error {
	return errors.New(http.StatusForbidden, DangerousCommandReason,
		fmt.Sprintf("%s: %s", DangerousCommandMarker, strings.Join(patterns, "; ")))
}
