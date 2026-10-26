// Package configenv 配置的环境变量显式覆盖。
//
// 背景(实测勘误):kratos config 的 ${VAR:default} 占位符只解析"配置键",
// 不读环境变量——配置文件里的 ${ENV_NAME:default} 永远落到 default 分支
// (整个配置栈没有挂 env source)。且 bootstrap.Context.GetConfig() 每次返回
// proto 深拷贝,任何"装配前统一覆盖"都会改在克隆体上而丢失——覆盖必须
// 在每个消费点调用:MQTT 口令见 ApplyMqtt(core 的 broker 连接与 :9500
// 认证回调两处)、MinIO 凭据见 ApplyOSS(oss.NewMinIoClient 内部收口)、
// 登录口令密钥见 LoginAESKey(各解密点直调)。
package configenv

import (
	"os"
	"strings"

	gocrypto "github.com/tx7do/go-utils/crypto"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
)

// LoginAESKeyEnv 口令传输加密密钥的环境变量名(前端对应 VITE_AES_KEY)。
const LoginAESKeyEnv = "LOGIN_AES_KEY"

// LoginAESKey 登录/注册/改密口令的传输加密密钥:前端以同一密钥(VITE_AES_KEY,
// AES-128-CBC key=iv)加密 password 字段,后端解密。默认回落上游库的公开
// 默认串(仅限开发);生产必须两端一致配置,否则登录解密全部失败。
func LoginAESKey() []byte {
	if v := strings.TrimSpace(os.Getenv(LoginAESKeyEnv)); v != "" {
		return []byte(v)
	}
	return gocrypto.DefaultAESKey
}

// ApplyMqtt 用环境变量覆盖 Bootstrap 配置中的 MQTT 口令。
//
// ⚠️ 必须在每个消费点调用:bootstrap.Context.GetConfig() 每次返回 proto
// 深拷贝,任何"装配前统一覆盖"都会改在克隆体上而丢失。core 的 MQTT broker
// 连接与 :9500 认证回调两个消费点都要调用(同一环境变量,覆盖后两端一致)。
func ApplyMqtt(cfg *conf.Bootstrap) {
	if cfg == nil {
		return
	}
	// core 自身 MQTT 口令:持有它即可向全部 agent 下发指令,生产必须注入。
	if m := cfg.GetServer().GetMqtt(); m != nil {
		if v := os.Getenv("MQTT_CORE_PASSWORD"); strings.TrimSpace(v) != "" {
			m.Password = v
		}
	}
}

// ApplyOSS 用环境变量覆盖 Bootstrap 配置中的 MinIO 凭据。
// pkg/oss.NewMinIoClient 内部已统一调用,新调用方无需重复。
func ApplyOSS(cfg *conf.Bootstrap) {
	if cfg == nil {
		return
	}
	if o := cfg.GetOss().GetMinio(); o != nil {
		if v := os.Getenv("MINIO_ACCESS_KEY"); strings.TrimSpace(v) != "" {
			o.AccessKey = v
		}
		if v := os.Getenv("MINIO_SECRET_KEY"); strings.TrimSpace(v) != "" {
			o.SecretKey = v
		}
	}
}
