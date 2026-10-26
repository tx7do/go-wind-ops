package signal

import (
	"testing"
	"time"
)

func testEnvelope() *CommandEnvelope {
	return &CommandEnvelope{
		RequestId: "req-1",
		AgentId:   "agent-a",
		Action:    CommandActionScriptExec,
		Payload:   []byte(`{"content":"echo hi"}`),
		IssuedAt:  time.Now(),
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	env := testEnvelope()
	SignEnvelope(env, "k1")
	if env.Signature == "" {
		t.Fatal("signature not set")
	}
	if err := VerifyEnvelope(env, "k1", time.Now()); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyUnsignedMode(t *testing.T) {
	// key 未配置(空)时跳过校验——未签名模式的兼容语义。
	env := testEnvelope()
	if err := VerifyEnvelope(env, "", time.Now()); err != nil {
		t.Fatalf("unsigned mode should pass: %v", err)
	}
}

func TestVerifyRejects(t *testing.T) {
	cases := map[string]func(env *CommandEnvelope){
		"missing signature":  func(env *CommandEnvelope) { env.Signature = "" },
		"wrong key":          func(env *CommandEnvelope) { SignEnvelope(env, "other-key") },
		"tampered payload":   func(env *CommandEnvelope) { SignEnvelope(env, "k1"); env.Payload = []byte(`{"content":"rm -rf /"}`) },
		"tampered action":    func(env *CommandEnvelope) { SignEnvelope(env, "k1"); env.Action = CommandActionAgentUpgrade },
		"cross agent replay": func(env *CommandEnvelope) { SignEnvelope(env, "k1"); env.AgentId = "agent-b" },
		"stale": func(env *CommandEnvelope) {
			SignEnvelope(env, "k1")
			env.IssuedAt = time.Now().Add(-CommandStaleMaxAge - time.Minute)
		},
		"future beyond skew": func(env *CommandEnvelope) {
			SignEnvelope(env, "k1")
			env.IssuedAt = time.Now().Add(CommandFutureSkew + time.Minute)
		},
		"missing issued_at":   func(env *CommandEnvelope) { SignEnvelope(env, "k1"); env.IssuedAt = time.Time{} },
		"operator not signed": func(env *CommandEnvelope) { SignEnvelope(env, "k1"); env.OperatorId = 42 },
	}
	for name, mutate := range cases {
		env := testEnvelope()
		mutate(env)
		if err := VerifyEnvelope(env, "k1", time.Now()); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}

func TestVerifyWithinSkew(t *testing.T) {
	env := testEnvelope()
	env.IssuedAt = time.Now().Add(CommandFutureSkew / 2)
	SignEnvelope(env, "k1")
	if err := VerifyEnvelope(env, "k1", time.Now()); err != nil {
		t.Fatalf("skew within allowance should pass: %v", err)
	}
}
