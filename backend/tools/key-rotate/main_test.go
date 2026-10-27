package main

import (
	"strings"
	"testing"
)

// TestGenerateAllConstraints 生成规格:AES-128 配对恰 16 字节且两键同值,HMAC 类 64 hex。
func TestGenerateAllConstraints(t *testing.T) {
	secrets := generateAll()
	byName := map[string]string{}
	for _, s := range secrets {
		byName[s.name] = s.value
	}

	login, vite := byName["LOGIN_AES_KEY"], byName["VITE_AES_KEY"]
	if len(login) != 32 { // AES-128 → 16 字节 → 32 hex
		t.Fatalf("LOGIN_AES_KEY len=%d want 32", len(login))
	}
	if login != vite {
		t.Fatalf("LOGIN_AES_KEY(%q) != VITE_AES_KEY(%q)", login, vite)
	}

	for _, name := range []string{"JWT_SIGNING_KEY", "JWT_SIGNING_KEY_APP", "COMMAND_SIGN_KEY",
		"MQTT_AUTH_WEBHOOK_SECRET", "CORE_RPC_SECRET", "OPENAPI_GIT_WEBHOOK_SECRET"} {
		if len(byName[name]) != 64 {
			t.Errorf("%s len=%d want 64 hex", name, len(byName[name]))
		}
	}

	if len(byName["MQTT_CORE_PASSWORD"]) != 32 {
		t.Errorf("MQTT_CORE_PASSWORD len=%d want 32", len(byName["MQTT_CORE_PASSWORD"]))
	}
}

// TestRenderContainsAll 输出覆盖全部键与非生成项说明。
func TestRenderContainsAll(t *testing.T) {
	text := render(generateAll())
	for _, name := range []string{"JWT_SIGNING_KEY", "JWT_SIGNING_KEY_APP", "COMMAND_SIGN_KEY",
		"MQTT_AUTH_WEBHOOK_SECRET", "CORE_RPC_SECRET", "MQTT_CORE_PASSWORD",
		"OPENAPI_GIT_WEBHOOK_SECRET", "LOGIN_AES_KEY", "VITE_AES_KEY",
		"AUDIT_SIGN_KEY_FILE", "OIDC_CLIENT_SECRET"} {
		if !strings.Contains(text, name+"=") && !strings.Contains(text, name+":") {
			t.Errorf("render missing %s", name)
		}
	}
}
