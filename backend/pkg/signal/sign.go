// 指令信封 HMAC 签名:把 agent 侧的信任锚从"MQTT broker 完好"升级为
// "持有 COMMAND_SIGN_KEY"。签名覆盖 request_id/agent_id/action/issued_at/
// payload 五元组,配合时效校验与 request_id 去重,封死三类攻击:
//   - 伪造:持有 core MQTT 口令(或 broker 被攻破)者可向任意 agent 主题
//     注入指令——无有效签名即被 agent 拒绝;
//   - 跨 agent 重放:签名含目标 agent_id,截获的指令无法改投其他 agent;
//   - 同 agent 重放:签名的时效窗口 + agent 侧 request_id 去重兜底。
//
// 密钥经 COMMAND_SIGN_KEY 环境变量下发,core 与全部 agent 必须一致;
// 任一端未配置则按"未签名模式"运行并告警(仅限开发环境)。
package signal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// SignKeyEnv 指令签名密钥的环境变量名。
const SignKeyEnv = "COMMAND_SIGN_KEY"

// 命名空间隔离:信封签名与未来其他用途(如心跳签名)共用密钥时避免串签。
const envelopeSignContext = "gwo-command-v1"

// CommandStaleMaxAge 指令时效:签发后超过该时长的指令视为过期(EMQX 离线
// 队列里滞留超过时效的指令会被拒收,由台账超时链路定案 FAILED)。
const CommandStaleMaxAge = 15 * time.Minute

// CommandFutureSkew 允许的时钟前拨余量(core 与 agent 机器钟差)。
const CommandFutureSkew = 5 * time.Minute

// SignKeyFromEnv 读取签名密钥(去除首尾空白);未配置返回空串。
func SignKeyFromEnv() string {
	return strings.TrimSpace(os.Getenv(SignKeyEnv))
}

// SignEnvelope 计算并回填信封签名(覆盖已有值)。key 为空时不动信封。
func SignEnvelope(env *CommandEnvelope, key string) {
	if env == nil || key == "" {
		return
	}
	env.Signature = computeSignature(env, key)
}

// VerifyEnvelope 校验信封签名与时效。key 为空时跳过校验(未签名模式)。
// 校验失败返回具体原因:缺失签名、签名不符、过期、时钟前拨超限。
func VerifyEnvelope(env *CommandEnvelope, key string, now time.Time) error {
	if env == nil {
		return errors.New("nil envelope")
	}
	if key == "" {
		return nil
	}
	if env.Signature == "" {
		return errors.New("missing signature")
	}
	if env.IssuedAt.IsZero() {
		return errors.New("missing issued_at")
	}
	if d := now.Sub(env.IssuedAt); d > CommandStaleMaxAge {
		return fmt.Errorf("stale command (age %s > %s)", d.Truncate(time.Second), CommandStaleMaxAge)
	}
	if d := env.IssuedAt.Sub(now); d > CommandFutureSkew {
		return fmt.Errorf("issued_at in future beyond skew (%s > %s)", d.Truncate(time.Second), CommandFutureSkew)
	}
	if !hmac.Equal([]byte(computeSignature(env, key)), []byte(env.Signature)) {
		return errors.New("signature mismatch")
	}
	return nil
}

// computeSignature HMAC-SHA256,覆盖信封全部业务字段;agent_id 参与签名使
// 截获的指令无法改投其他 agent(各 agent 校验时以自身 ID 入串),
// issued_at 入签防止被截获的签名经时间前拨无限续命。
func computeSignature(env *CommandEnvelope, key string) string {
	mac := hmac.New(sha256.New, []byte(envelopeSignContext+":"+key))
	_, _ = fmt.Fprintf(mac, "%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%x",
		envelopeSignContext, env.RequestId, env.AgentId, string(env.Action),
		env.IssuedAt.UTC().Format(time.RFC3339Nano), env.OperatorId, env.Payload)
	return hex.EncodeToString(mac.Sum(nil))
}
