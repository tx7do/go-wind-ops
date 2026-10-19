// Package cloudprovider 云厂商密钥在线校验:各实现仅用标准库按厂商签名规范
// 直调最便宜的身份 API(通常是 STS GetCallerIdentity),不引入厂商 SDK。
// 校验结果只回填账号 ID/名,不承载计费/资源查询。
package cloudprovider

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// VerifyResult 身份校验结果(厂商未必都提供账号名)。
type VerifyResult struct {
	AccountID   string
	AccountName string
}

// Verify 按 provider(ALIYUN/TENCENT_CLOUD/HUAWEI_CLOUD/AWS)校验 AK/SK 有效性。
func Verify(ctx context.Context, provider, accessKey, secretKey, region string) (*VerifyResult, error) {
	switch provider {
	case "ALIYUN":
		return verifyAliyun(ctx, accessKey, secretKey)
	case "TENCENT_CLOUD":
		return verifyTencentCloud(ctx, accessKey, secretKey)
	case "AWS":
		return verifyAWS(ctx, accessKey, secretKey, region)
	case "HUAWEI_CLOUD":
		return nil, fmt.Errorf("华为云暂不支持在线校验,请通过控制台确认密钥有效性")
	default:
		return nil, fmt.Errorf("未知的云厂商: %s", provider)
	}
}

// hmacSHA256 计算 HMAC-SHA256。
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// sha256Hex 计算十六进制 SHA-256( SigV4/TC3 的 payload 摘要)。
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hmacSHA1Base64 计算 HMAC-SHA1 的 base64(阿里云 RPC 签名)。
func hmacSHA1Base64(key, data []byte) string {
	h := hmac.New(sha1.New, key)
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// nowUTC 统一取 UTC 当前时间,便于测试与签名对齐。
func nowUTC() time.Time {
	return time.Now().UTC()
}
