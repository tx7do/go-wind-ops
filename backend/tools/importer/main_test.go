package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"
)

// TestEncryptPassword AES-128-CBC(key=iv,PKCS7,base64)形态:
// 输出可解密回原文(与前端 utils/crypto.ts 同规)。
func TestEncryptPassword(t *testing.T) {
	const key = "0123456789abcdef" // 16 字节
	enc, err := encryptPassword("admin123", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "" || enc == "admin123" {
		t.Fatalf("unexpected ciphertext %q", enc)
	}

	// 解密回原文(独立实现,防"加密即返回输入"的假通过)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if len(decoded) == 0 || len(decoded)%16 != 0 {
		t.Fatalf("ciphertext len %d not block aligned", len(decoded))
	}
	plain := make([]byte, len(decoded))
	cipher.NewCBCDecrypter(block, []byte(key)).CryptBlocks(plain, decoded)
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > 16 {
		t.Fatalf("bad padding %d", pad)
	}
	if string(plain[:len(plain)-pad]) != "admin123" {
		t.Fatalf("roundtrip = %q", plain[:len(plain)-pad])
	}
}

// TestManifestParse 清单解析:字段映射与可选项。
func TestManifestParse(t *testing.T) {
	raw := []byte(`{
		"scripts": [{"name":"s1","content":"echo hi","timeoutSeconds":30}],
		"projects": [{"name":"p1","nodeId":2,"projectDir":"/opt/a","env":"prod","labels":["w"]}]
	}`)
	mf, err := parseManifest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(mf.Scripts) != 1 || mf.Scripts[0].Name != "s1" || mf.Scripts[0].TimeoutSeconds != 30 {
		t.Errorf("scripts = %+v", mf.Scripts)
	}
	if len(mf.Projects) != 1 || mf.Projects[0].NodeID != 2 || mf.Projects[0].Env != "prod" {
		t.Errorf("projects = %+v", mf.Projects)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate([]byte("short")); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
}
