// 审计签名密钥持久化:ECDSA 私钥此前每次进程启动临时生成、随进程消亡,
// 历史审计签名的可验证性随之丢失(不可抵赖性不成立)。现在 load-or-generate
// 到 AUDIT_SIGN_KEY_FILE(默认 data/audit-signing-key.pem,SEC1 PEM,0600),
// 重启后沿用同一把密钥,审计签名可跨重启验证。
package logging

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"
)

// AuditSignKeyFileEnv 审计签名私钥落盘路径的环境变量名。
const AuditSignKeyFileEnv = "AUDIT_SIGN_KEY_FILE"

// defaultAuditSignKeyFile 默认落盘路径(相对进程工作目录)。
const defaultAuditSignKeyFile = "data/audit-signing-key.pem"

// loadOrGenerateECDSAKeyPair 加载持久化的审计签名私钥;不存在/损坏则生成
// 新密钥并落盘。落盘失败(只读 FS 等)返回临时密钥并告警——签发能力不中断,
// 但日志会明确提示签名不可跨重启验证。
func loadOrGenerateECDSAKeyPair(logger *log.Helper) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	path := os.Getenv(AuditSignKeyFileEnv)
	if path == "" {
		path = defaultAuditSignKeyFile
	}

	if key, err := readECPrivateKeyFile(path); err == nil {
		return key, &key.PublicKey, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Warnf("audit signing key file unreadable (%s), regenerating: %v", path, err)
	}

	priv, pub, err := generateECDSAKeyPair()
	if err != nil {
		return nil, nil, err
	}

	if err := writeECPrivateKeyFile(path, priv); err != nil {
		logger.Warnf("audit signing key persistence FAILED (%s): %v — signatures will NOT be verifiable across restarts", path, err)
		return priv, pub, nil
	}
	logger.Infof("audit signing key generated at %s", path)
	return priv, pub, nil
}

func readECPrivateKeyFile(path string) (*ecdsa.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("not an EC PRIVATE KEY pem: %s", path)
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func writeECPrivateKeyFile(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal ec key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	buf := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	return os.WriteFile(path, buf, 0o600)
}
