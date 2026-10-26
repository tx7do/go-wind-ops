// Package envs 运行环境判定:生产态守卫用。APP_ENV=production 时,各服务
// 对"公开默认密钥/缺省加密密钥"之类的开发便利配置拒绝启动(fail-closed),
// 防止默认配置直接上线。未设置或任何其他值均视为开发环境。
package envs

import "os"

// EnvKey 环境标识变量名。
const EnvKey = "APP_ENV"

// IsProduction 报告当前是否生产环境(APP_ENV=production)。
func IsProduction() bool {
	return os.Getenv(EnvKey) == "production"
}
