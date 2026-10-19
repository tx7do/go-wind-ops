package data

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	casbinEngine "github.com/tx7do/kratos-authz/engine/casbin"
	authzEngine "github.com/tx7do/kratos-authz/engine"

	"go-wind-ops/pkg/authorizer"
)

func newTestEngine(t *testing.T, mode string) *AuthzEngine {
	t.Helper()
	inner, err := casbinEngine.NewEngine(context.Background())
	if err != nil {
		t.Fatalf("casbin engine: %v", err)
	}
	return &AuthzEngine{inner: inner, mode: mode, log: log.NewHelper(log.DefaultLogger)}
}

func TestCasbinRulesOf(t *testing.T) {
	data := authorizer.PermissionDataMap{
		"admin": {authorizer.PermissionData{Path: "/admin/v1/scripts/{id}", Method: "PUT"}},
		"ops":   {authorizer.PermissionData{Path: "/admin/v1/scripts/{id}/executions:send", Method: "POST"}},
	}
	rules := casbinRulesOf(data)
	if len(rules) != 2 {
		t.Fatalf("expect 2 rules, got %d", len(rules))
	}
	for _, r := range rules {
		if r.PType != "p" || r.V3 != "*" {
			t.Fatalf("bad rule %+v", r)
		}
	}
}

func TestShadowAllowsDespiteNoPolicy(t *testing.T) {
	e := newTestEngine(t, authzModeShadow)
	allowed, err := e.IsAuthorized(context.Background(), "nobody", "GET", "/admin/v1/scripts", "")
	if err != nil || !allowed {
		t.Fatalf("shadow must allow: allowed=%v err=%v", allowed, err)
	}
}

func TestEnforceDeniesWithoutPolicy(t *testing.T) {
	e := newTestEngine(t, authzModeEnforce)
	allowed, err := e.IsAuthorized(context.Background(), "nobody", "GET", "/admin/v1/scripts", "")
	if allowed {
		t.Fatalf("enforce with empty policy must deny")
	}
	_ = err
}

func TestNoopAlwaysAllows(t *testing.T) {
	var e authzEngine.Engine = noopEngine{}
	allowed, err := e.IsAuthorized(context.Background(), "x", "DELETE", "/anything", "")
	if err != nil || !allowed {
		t.Fatalf("noop must allow: allowed=%v err=%v", allowed, err)
	}
}
