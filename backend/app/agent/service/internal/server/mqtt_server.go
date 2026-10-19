// Package server 组装 agent 的传输层。agent 只有单一 MQTT 出站连接:
// 不注册服务发现、无 REST/gRPC 服务、不连数据库。
package server

import (
	"os"
	"strings"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	kratosLog "github.com/go-kratos/kratos/v2/log"
	mqttBootstrap "github.com/tx7do/kratos-bootstrap/transport/mqtt"
	"github.com/tx7do/kratos-transport/transport/mqtt"

	"go-wind-ops/app/agent/service/internal/agent"
	"go-wind-ops/pkg/filetransfer"
	"go-wind-ops/pkg/tlsconf"
	"go-wind-ops/pkg/topic"
)

// NewAgentMqttServer 构建 agent 的 MQTT 连接:
//   - clientID = agent-{machineID},与 agentId 一致便于 EMQX 侧排查;
//   - MQTT 凭证 = agent 自身身份(username=agentId,password=本机 token):
//     EMQX HTTP 认证回调 core 校验 token 哈希,首连 TOFU 注册;
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

	// broker 级认证要求以自身身份连接:token 在装配连接前确保已生成/加载
	// (LoadOrGenerateAuthToken 幂等,Start 内的再次调用为空操作)。
	if err := agentService.LoadOrGenerateAuthToken(); err != nil {
		panic("load agent auth token failed: " + err.Error())
	}

	// 预签名直传直拉的出站主机白名单(纵深防御:只应指向对象存储)。
	if hosts := os.Getenv("AGENT_DOWNLOAD_ALLOWED_HOSTS"); hosts != "" {
		filetransfer.ConfigureAllowedHosts(strings.Split(hosts, ","))
	}

	clientID := os.Getenv("AGENT_MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = "agent-" + agentService.AgentID()
	}

	opts := []mqtt.ServerOption{
		mqtt.WithEnableKeepAlive(false),
		mqtt.WithClientId(clientID),
		mqtt.WithOnConnect(agentService.OnMqttConnected),
		// 覆盖配置里的静态共享凭证:broker 级认证按 agent 身份(token)校验
		mqtt.WithAuth(agentService.AgentID(), agentService.GetAuthToken()),
	}

	if tlsCfg := tlsconf.ClientTLSConfig(cfg.Server.Mqtt.GetTls()); tlsCfg != nil {
		opts = append(opts, mqtt.WithTLSConfig(tlsCfg))
	}

	if len(filetransferHostsSnapshot()) == 0 {
		kratosLog.Warnf("AGENT_DOWNLOAD_ALLOWED_HOSTS not set: presigned download/upload hosts are unrestricted (set it to the MinIO host in production)")
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

	// Web 终端尺寸(通配所有会话,payload.session_id 路由)
	_ = mqtt.RegisterSubscriber(srv,
		ctx.Context(),
		topic.MakeMqttTopicTerminalResize(agentService.AgentID(), "+"),
		agentService.HandleTerminalResize,
	)

	return srv
}

// filetransferHostsSnapshot 判断白名单是否已配置(告警用)。
func filetransferHostsSnapshot() []string {
	hosts := os.Getenv("AGENT_DOWNLOAD_ALLOWED_HOSTS")
	if hosts == "" {
		return nil
	}
	return strings.Split(hosts, ",")
}
