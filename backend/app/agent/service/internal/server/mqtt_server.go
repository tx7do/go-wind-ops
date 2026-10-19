// Package server 组装 agent 的传输层。agent 只有单一 MQTT 出站连接:
// 不注册服务发现、无 REST/gRPC 服务、不连数据库。
package server

import (
	"os"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	mqttBootstrap "github.com/tx7do/kratos-bootstrap/transport/mqtt"
	"github.com/tx7do/kratos-transport/transport/mqtt"

	"go-wind-ops/app/agent/service/internal/agent"
	"go-wind-ops/pkg/topic"
)

// NewAgentMqttServer 构建 agent 的 MQTT 连接:
//   - clientID = agent-{machineID},与 agentId 一致便于 EMQX 侧排查;
//   - OnConnect 回调触发准入请求(断线重连后自动重新认证并重订阅);
//   - 订阅 auth/response 与 command 主题(kratos mqtt server 在连接建立后
//     统一执行订阅注册,断线重连由 doRegisterSubscriberMap 恢复)。
func NewAgentMqttServer(
	ctx *bootstrap.Context,
	agentService *agent.Service,
) *mqtt.Server {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Mqtt == nil {
		return nil
	}

	clientID := os.Getenv("AGENT_MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = "agent-" + agentService.AgentID()
	}

	opts := []mqtt.ServerOption{
		mqtt.WithEnableKeepAlive(false),
		mqtt.WithClientId(clientID),
		mqtt.WithOnConnect(agentService.OnMqttConnected),
	}

	srv := mqttBootstrap.NewMqttServer(cfg.Server.Mqtt, opts...)

	agentService.RegisterMqttBroker(srv)

	if err := agentService.Start(ctx.Context()); err != nil {
		panic("start agent service failed: " + err.Error())
	}

	// 准入应答
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.MakeMqttTopicAuth(agentService.AgentID(), false),
		agentService.HandleAuthResponse,
	)

	// 下行指令
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.MakeMqttTopicCommand(agentService.AgentID()),
		agentService.HandleCommand,
	)

	// Web 终端输入(通配所有会话,payload.session_id 路由)
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.MakeMqttTopicTerminalInput(agentService.AgentID(), "+"),
		agentService.HandleTerminalInput,
	)

	return srv
}
