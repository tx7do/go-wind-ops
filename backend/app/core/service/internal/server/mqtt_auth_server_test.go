package server

import "testing"

func TestAuthorizeMqttTopic(t *testing.T) {
	cases := []struct {
		name     string
		coreUser string
		username string
		topic    string
		want     bool
	}{
		// core 账号
		{"core 下发指令", "core", "core", "ops/agent/agent-1/command", true},
		{"core 订阅终端输出", "core", "core", "ops/agent/+/terminal/+/output", true},
		{"core 共享订阅回执", "core", "core", "$share/gwo/ops/agent/+/command/result", true},
		{"core 共享订阅心跳", "core", "core", "$share/gwo/ops/agent/+/heartbeat", true},
		{"core 订 $SYS 拒绝", "core", "core", "$SYS/brokers/1/version", false},
		{"core 订非 ops 拒绝", "core", "core", "other/topic", false},

		// agent 身份(用户名=agentId)
		{"agent 订自己指令", "core", "agent-1", "ops/agent/agent-1/command", true},
		{"agent 发自己回执", "core", "agent-1", "ops/agent/agent-1/command/result", true},
		{"agent 终端输入通配", "core", "agent-1", "ops/agent/agent-1/terminal/+/input", true},
		{"agent 发自己心跳", "core", "agent-1", "ops/agent/agent-1/heartbeat", true},
		{"agent 订别台机器拒绝", "core", "agent-1", "ops/agent/agent-2/command", false},
		{"agent 终端通配跨机拒绝", "core", "agent-1", "ops/agent/+/terminal/+/output", false},
		{"agent 订全部拒绝", "core", "agent-1", "ops/#", false},
		{"agent 订 $SYS 拒绝", "core", "agent-1", "$SYS/#", false},
		{"agent 共享订阅剥壳后仍限自己", "core", "agent-1", "$share/gwo/ops/agent/agent-1/command", true},
		{"agent 共享订阅别机拒绝", "core", "agent-1", "$share/gwo/ops/agent/agent-2/command", false},
		{"agent 主题段不足拒绝", "core", "agent-1", "ops/agent", false},
		{"伪 core 用户名不匹配拒绝", "core", "corex", "ops/#", false},

		// 防御性
		{"空用户拒绝", "core", "", "ops/agent/agent-1/command", false},
		{"空主题拒绝", "core", "agent-1", "", false},
		{"未配置 core 账号时按 agent 规则", "", "agent-1", "ops/agent/agent-1/command", true},
		{"未配置 core 账号时 core 名按 agent 规则", "", "core", "ops/agent/+/command/result", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := authorizeMqttTopic(tc.coreUser, tc.username, tc.topic); got != tc.want {
				t.Fatalf("authorizeMqttTopic(core=%q user=%q topic=%q) = %v, want %v",
					tc.coreUser, tc.username, tc.topic, got, tc.want)
			}
		})
	}
}
