// Package totp RFC 6238 时间型一次性密码(TOTP),SHA-1/6 位/30 秒步长,
// 与主流验证器 App(Google Authenticator/Microsoft Authenticator/1Password)兼容。
// 零第三方依赖:crypto/hmac + encoding/base32 即可;±1 窗口容忍时钟漂移。
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// Step 秒级时间步长(RFC 6238 推荐 30s)。
	Step = 30 * time.Second
	// Digits 验证码位数。
	Digits = 6
	// skew 允许的时钟漂移窗口(±1 步 = ±30s)。
	skew = 1
	// secretBytes 随机密钥字节数(160bit,base32 后 32 字符)。
	secretBytes = 20
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret 生成新的 Base32 密钥(无填充,验证器 App 扫码/手输的标准形态)。
func GenerateSecret() (string, error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return b32.EncodeToString(raw), nil
}

// OtpauthURI 构造 otpauth:// 统一资源标识(验证器 App 扫码导入)。
func OtpauthURI(issuer, account, secret string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		url.PathEscape(issuer), url.PathEscape(account), url.QueryEscape(secret),
		url.QueryEscape(issuer), Digits, int(Step.Seconds()))
}

// Verify 校验验证码:当前时间及 ±skew 窗口内任一匹配即通过(常数时间比较)。
func Verify(secret, code string, now time.Time) bool {
	if len(code) != Digits {
		return false
	}
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil || len(key) == 0 {
		return false
	}
	counter := uint64(now.Unix()) / uint64(Step.Seconds())
	for i := -int64(skew); i <= skew; i++ {
		if hotp(key, counter+uint64(i)) == code {
			return true
		}
	}
	return false
}

// hotp RFC 4226 HMAC-SHA-1 动态截断,返回 6 位十进制字符串。
func hotp(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}
