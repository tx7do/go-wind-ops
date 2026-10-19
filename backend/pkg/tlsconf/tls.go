// Package tlsconf 从 kratos-bootstrap 的 conf.TLS 构造出站 TLS 配置。
// MQTT 信令通道(EMQX 8883/ssl://)等自签场景:dev 用 insecure_skip_verify,
// 生产配置 file.ca_path 指向 CA 证书做校验。
package tlsconf

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
)

// ClientTLSConfig 客户端侧 TLS 配置;cfg 为 nil 返回 nil(不启用 TLS)。
// 仅消费 ca_path 与 insecure_skip_verify——客户端不需要证书链(EMQX 未开
// 双向认证)。
func ClientTLSConfig(cfg *conf.TLS) *tls.Config {
	if cfg == nil {
		return nil
	}
	out := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.GetInsecureSkipVerify(), //nolint:gosec // 自签 dev 场景显式配置
	}
	if caPath := cfg.GetFile().GetCaPath(); caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return out
		}
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM(pem) {
			out.RootCAs = pool
		}
	}
	return out
}
