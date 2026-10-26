package server

import "testing"

import opsV1 "go-wind-ops/api/gen/go/ops/service/v1"

// TestScopeAllowed OpenAPI 网关作用域闸门语义:
// 未限定(空)= 全部(存量密钥兼容);ALL 通配;其余须精确命中目标类别。
func TestScopeAllowed(t *testing.T) {
	cases := []struct {
		name     string
		scopes   []opsV1.ApiKey_Scope
		required opsV1.ApiKey_Scope
		want     bool
	}{
		{"无作用域放行", nil, opsV1.ApiKey_BUILDS, true},
		{"ALL 通配放行", []opsV1.ApiKey_Scope{opsV1.ApiKey_ALL}, opsV1.ApiKey_PIPELINES, true},
		{"精确命中放行", []opsV1.ApiKey_Scope{opsV1.ApiKey_BUILDS, opsV1.ApiKey_WEBHOOKS}, opsV1.ApiKey_BUILDS, true},
		{"类别不符拒绝", []opsV1.ApiKey_Scope{opsV1.ApiKey_BUILDS}, opsV1.ApiKey_DISTRIBUTIONS, false},
		{"仅 webhook 密钥不可触发流水线", []opsV1.ApiKey_Scope{opsV1.ApiKey_WEBHOOKS}, opsV1.ApiKey_PIPELINES, false},
	}
	for _, c := range cases {
		if got := scopeAllowed(c.scopes, c.required); got != c.want {
			t.Errorf("%s: scopeAllowed=%v want %v", c.name, got, c.want)
		}
	}
}
