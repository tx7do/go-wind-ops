package server

import (
	"os"

	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	mqttBootstrap "github.com/tx7do/kratos-bootstrap/transport/mqtt"
	"github.com/tx7do/kratos-transport/transport/mqtt"

	"go-wind-ops/app/core/service/internal/service"
	"go-wind-ops/pkg/tlsconf"
	"go-wind-ops/pkg/topic"
)

// NewCoreMqttServer 构建 core 服务对 agent 的单一 MQTT 客户端连接(全局
// core 超级用户),完成 broker 注入与 $share 共享订阅注册:
// 上行(auth/heartbeat/command_result)全部走共享订阅,多副本部署时组内
// 负载均衡;下行(publish)用原始主题。
//
// 未配置 server.mqtt 段时返回 nil(core 仍可无 MQTT 运行,仅无 agent 能力)。
func NewCoreMqttServer(
	ctx *bootstrap.Context,
	agentService *service.AgentService,
) *mqtt.Server {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Mqtt == nil {
		return nil
	}

	machineID, _ := id.UnifyMachineID()
	// clientID 默认按机器派生;同机多平台(或同机多套环境)会与其它服务撞车
	// 导致 EMQX 会话互踢,可用 MQTT_CLIENT_ID 显式错开。
	clientID := os.Getenv("MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = "core-" + machineID
	}

	opts := []mqtt.ServerOption{
		mqtt.WithEnableKeepAlive(false),
		mqtt.WithClientId(clientID),
	}
	if tlsCfg := tlsconf.ClientTLSConfig(cfg.Server.Mqtt.GetTls()); tlsCfg != nil {
		opts = append(opts, mqtt.WithTLSConfig(tlsCfg))
	}

	srv := mqttBootstrap.NewMqttServer(cfg.Server.Mqtt, opts...)

	agentService.RegisterMqttBroker(srv)
	agentService.Start(ctx.Context())

	// agent 准入请求
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.SharedSubscription(topic.MqttShareGroupCore, topic.MakeMqttTopicAuth("+", true)),
		agentService.HandleAgentAuth,
	)

	// 心跳(内含指标快照)
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.SharedSubscription(topic.MqttShareGroupCore, topic.MakeMqttTopicHeartbeat("+")),
		agentService.HandleHeartbeat,
	)

	// 指令执行结果回执
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.SharedSubscription(topic.MqttShareGroupCore, topic.MakeMqttTopicCommandResult("+")),
		agentService.HandleCommandResult,
	)

	// Web 终端输出:连接级点对点流,不经 $share(否则多副本组内分流会丢流)。
	// 每副本都收全量输出,仅持有对应 gRPC 流的副本消费,其余按路由表丢弃。
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.MakeMqttTopicTerminalOutput("+", "+"),
		agentService.HandleTerminalOutput,
	)

	return srv
}
