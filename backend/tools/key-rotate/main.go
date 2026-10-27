// key-rotate 密钥轮换辅助:为平台全部共享密钥生成密码学安全随机值,
// 输出 .env 形态(可直接落 env 文件/密管系统),并附消费方与轮换次序说明。
//
// 覆盖密钥与格式约束:
//   JWT_SIGNING_KEY / JWT_SIGNING_KEY_APP   HMAC 签名,64 hex(32B)
//   COMMAND_SIGN_KEY                        指令信封 HMAC(core 与全部 agent 共享)
//   MQTT_AUTH_WEBHOOK_SECRET                EMQX→core 认证回调口令
//   CORE_RPC_SECRET                         BFF→core gRPC 共享密钥
//   MQTT_CORE_PASSWORD                      core 的 MQTT 账号口令(强口令必设)
//   OPENAPI_GIT_WEBHOOK_SECRET              git webhook HMAC(与 CI 侧共享)
//   LOGIN_AES_KEY / VITE_AES_KEY            口令传输 AES-128-CBC,必须恰 16B 且
//                                           两值相同;VITE_ 侧为构建期注入,
//                                           轮换须重建前端并同步发布,否则登录全挂
//   AUDIT_SIGN_KEY_FILE / OIDC_CLIENT_SECRET 非生成项,见输出尾部说明
//
// 轮换次序(双写窗口思想——先加新不删旧,全网换完再撤):
//   1) 生成新值 → 各服务 env 更新为"接受新值"(当前版本为单值配置,实际做法:
//      低峰一次性滚动重启 core→admin/front→agent;重启间隙指令信封/口令传输
//      会短暂失败,agent 有重试,可接受)
//   2) 前端重建(VITE_AES_KEY)与新 LOGIN_AES_KEY 同批发布
//   3) 审计签名密钥轮换:停 core → 备份并删除 AUDIT_SIGN_KEY_FILE → 启动自动
//      生成新密钥对;旧签名的可验证性随旧公钥灭失(链式 prev_log_hash 不受影响)
//
// 用法:
//   go run ./tools/key-rotate              # 输出到 stdout
//   go run ./tools/key-rotate -out .env.secrets   # 写文件(0600,勿入库)
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	out := flag.String("out", "", "写入文件(缺省 stdout;文件权限 0600)")
	flag.Parse()

	secrets := generateAll()
	text := render(secrets)

	if *out == "" {
		fmt.Print(text)
		return
	}
	if err := os.WriteFile(*out, []byte(text), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Printf("已写入 %s(0600)。该文件含明文密钥,严禁入库/入日志;分发范围=运维负责人。\n", *out)
}

// secretDef 一个密钥的生成规格与说明。
type secretDef struct {
	name    string
	comment []string
	value   string
}

// hex n 字节 → 2n 位 hex。
func hexOf(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败属系统级故障,直接终止
	}
	return strings.ToLower(fmt.Sprintf("%x", b))
}

// password 生成 n 字符 base62 口令(取模剔除偏差可忽略:62 < 2^6 余量充足场景不用于密码学值)。
// MQTT_CORE_PASSWORD 走 env,不要求密码学均匀,安全长度即可。
func password(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	out := make([]byte, n)
	for i, b := range raw {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out)
}

func generateAll() []secretDef {
	loginKey := hexOf(16) // AES-128 恰 16 字节;两键同值配对
	return []secretDef{
		{
			name: "JWT_SIGNING_KEY",
			comment: []string{
				"admin 后台 access/refresh token 签名(core 签发,admin 校验)。",
				"轮换=滚动重启 core+admin;已发 token 全部失效,用户需重新登录。",
			},
			value: hexOf(32),
		},
		{
			name: "JWT_SIGNING_KEY_APP",
			comment: []string{
				"C 端(front)token 签名,与后台分离。轮换同上,作用于 front。",
			},
			value: hexOf(32),
		},
		{
			name: "COMMAND_SIGN_KEY",
			comment: []string{
				"指令信封 HMAC(core 签,agent 强校验+时效+去重)。",
				"core 与全部 agent 必须同步换;先 agent 后 core 会导致 core 下发被拒。",
			},
			value: hexOf(32),
		},
		{
			name: "MQTT_AUTH_WEBHOOK_SECRET",
			comment: []string{
				"EMQX→core 认证/授权回调口令(compose env 与 core 配置两处)。",
				"轮换=改 compose 后重启 emqx+core。",
			},
			value: hexOf(32),
		},
		{
			name: "CORE_RPC_SECRET",
			comment: []string{
				"BFF(admin/front)→core gRPC 共享密钥。",
				"轮换=重启 core 与两个 BFF;错峰窗口内 BFF 调用会 401/拒,秒级自愈。",
			},
			value: hexOf(32),
		},
		{
			name: "MQTT_CORE_PASSWORD",
			comment: []string{
				"core 的 MQTT 账号口令(持有即可向全部 agent 下发指令,必须强口令)。",
				"轮换=重启 core;EMQX 认证回调即时生效。",
			},
			value: password(32),
		},
		{
			name: "OPENAPI_GIT_WEBHOOK_SECRET",
			comment: []string{
				"git webhook 签名(core 与 CI 侧(GitLab/Gitea webhook 配置)共享)。",
				"轮换=先改 CI 侧 webhook secret 再改 core,顺序颠倒会丢触发。",
			},
			value: hexOf(32),
		},
		{
			name: "LOGIN_AES_KEY",
			comment: []string{
				"口令传输 AES-128-CBC(后端解密)。与 VITE_AES_KEY 同值配对,恰 16 字节。",
				"轮换=重建前端(构建期注入)+同步换本键;错批发布=登录全挂。",
			},
			value: loginKey,
		},
		{
			name: "VITE_AES_KEY",
			comment: []string{
				"口令传输加密(前端构建期注入,值须与 LOGIN_AES_KEY 相同)。",
				"仅前端构建环境使用,不进服务端 env。",
			},
			value: loginKey,
		},
	}
}

func render(secrets []secretDef) string {
	var b strings.Builder
	b.WriteString("# go-wind-ops 共享密钥(自动生成,勿入库/勿入日志)\n")
	b.WriteString("# 生成时间戳见文件 mtime;分发范围=运维负责人。\n\n")
	for _, s := range secrets {
		for _, c := range s.comment {
			b.WriteString("# " + c + "\n")
		}
		b.WriteString(s.name + "=" + s.value + "\n\n")
	}
	b.WriteString("# ── 非生成项 ──\n")
	b.WriteString("# AUDIT_SIGN_KEY_FILE: 审计 ECDSA 私钥路径(默认 data/audit-signing-key.pem)。\n")
	b.WriteString("#   轮换=停 core→备份删除该文件→启动自动生成;旧签名验证能力随旧密钥灭失,\n")
	b.WriteString("#   哈希链(prev_log_hash)不受影响。核验:go run ./tools/audit-verify\n")
	b.WriteString("# OIDC_CLIENT_SECRET: 由 IdP 侧签发,在 IdP 控制台轮换后同步到 core env。\n")
	return b.String()
}
