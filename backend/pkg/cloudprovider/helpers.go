package cloudprovider

import (
	"crypto/rand"
	"encoding/hex"
)

// randomNonce 签名随机串(16 字节 hex)。
func randomNonce() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return hex.EncodeToString([]byte(nowUTC().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(raw)
}
