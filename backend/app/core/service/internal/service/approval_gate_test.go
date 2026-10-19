package service

import "testing"

func TestParseApprovalActions(t *testing.T) {
	got := parseApprovalActions(" agent_upgrade, Script_Exec ,,plugin_exec,")
	if len(got) != 3 {
		t.Fatalf("expect 3 actions, got %v", got)
	}
	for _, a := range []string{"agent_upgrade", "script_exec", "plugin_exec"} {
		if _, ok := got[a]; !ok {
			t.Fatalf("missing %q in %v", a, got)
		}
	}
	if len(parseApprovalActions("")) != 0 {
		t.Fatalf("empty env must yield empty set")
	}
}

func TestApprovalRequiredActionsDisabledByDefault(t *testing.T) {
	// 默认(APPROVAL_REQUIRED_ACTIONS 未设置)闸门不启用:直接放行
	if err := approvalGate(nil, nil, nil, "agent_upgrade", 0, "a", "n", "{}", 0); err != nil {
		t.Fatalf("gate must be no-op when env unset, got %v", err)
	}
}
