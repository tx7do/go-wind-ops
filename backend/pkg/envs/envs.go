// Package envs 运行环境判定:生产态守卫用。APP_ENV=production 时,各服务
// 对"公开默认密钥/缺省加密密钥"之类的开发便利配置拒绝启动(fail-closed),
// 防止默认配置直接上线。未设置或任何其他值均视为开发环境。
package envs

import (
	"os"
	"strings"
)

// EnvKey 环境标识变量名。
const EnvKey = "APP_ENV"

// EnrollmentAllowlistEnv agent 准入白名单的环境变量名(逗号分隔 agentId)。
// 非空时关闭 TOFU 开放注册:仅白名单内的 agentId 允许首连注册。
const EnrollmentAllowlistEnv = "AGENT_ENROLL_ALLOWLIST"

// IsProduction 报告当前是否生产环境(APP_ENV=production)。
func IsProduction() bool {
	return os.Getenv(EnvKey) == "production"
}

// EnrollmentAllowed 报告 agentId 是否允许 TOFU 首连注册:
// 白名单环境变量未配置 = 开放注册(历史行为);配置后仅白名单内放行。
func EnrollmentAllowed(agentID string) bool {
	v := strings.TrimSpace(os.Getenv(EnrollmentAllowlistEnv))
	if v == "" {
		return true
	}
	for _, id := range strings.Split(v, ",") {
		if strings.TrimSpace(id) == agentID {
			return true
		}
	}
	return false
}
