// Package server 组装 agent 的传输层。agent 只有单一 MQTT 出站连接:
// 不注册服务发现、无 REST/gRPC 服务、不连数据库。
package server

import (
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	kratosLog "github.com/go-kratos/kratos/v2/log"
	mqttBootstrap "github.com/tx7do/kratos-bootstrap/transport/mqtt"
	"github.com/tx7do/kratos-transport/transport/mqtt"

	"go-wind-ops/app/agent/service/internal/agent"
	"go-wind-ops/pkg/envs"
	"go-wind-ops/pkg/filetransfer"
	"go-wind-ops/pkg/mqttwrap"
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
) transport.Server {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Mqtt == nil {
		return nil
	}

	// broker 级认证要求以自身身份连接:token 在装配连接前确保已生成/加载
	// (LoadOrGenerateAuthToken 幂等,Start 内的再次调用为空操作)。
	if err := agentService.LoadOrGenerateAuthToken(); err != nil {
		panic("load agent auth token failed: " + err.Error())
	}

	// 生产态守卫:出站白名单缺省等于 file_pull/push 是无约束 SSRF/文件外带
	// 通道,生产直接拒绝启动;授权目录缺省时文件/项目操作本就全部拒绝
	// (fail-closed),仅强提示——存在"只用终端"的合法部署形态。
	if envs.IsProduction() && os.Getenv("AGENT_DOWNLOAD_ALLOWED_HOSTS") == "" {
		panic("AGENT_DOWNLOAD_ALLOWED_HOSTS must be set in production (presigned egress would be unrestricted)")
	}
	if os.Getenv("AGENT_DOWNLOAD_ALLOWED_HOSTS") == "" {
		kratosLog.Warnf("AGENT_DOWNLOAD_ALLOWED_HOSTS not set: presigned download/upload hosts are unrestricted (set it to the MinIO host in production)")
	}
	if envs.IsProduction() && os.Getenv("AGENT_ALLOWED_DIRS") == "" {
		kratosLog.Errorf("AGENT_ALLOWED_DIRS not set in production: all file/project operations will be rejected")
	}

	// 预签名直传直拉的出站主机白名单(纵深防御:只应指向对象存储)。
	if hosts := os.Getenv("AGENT_DOWNLOAD_ALLOWED_HOSTS"); hosts != "" {
		filetransfer.ConfigureAllowedHosts(strings.Split(hosts, ","))
	}

	if err := agentService.Start(ctx.Context()); err != nil {
		panic("start agent service failed: " + err.Error())
	}

	// EMQX 未就绪时传输库首连失败即 fatal(agent 曾直接 panic),且失败状态
	// 固化在实例内不可原地重试——按工厂模式重建实例重试。
	return mqttwrap.WrapFactory(func() (transport.Server, error) {
		srv := buildAgentMqttBroker(cfg, agentService)
		agentService.RegisterMqttBroker(srv)

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

		return srv, nil
	}, kratosLog.NewHelper(kratosLog.DefaultLogger), mqttwrap.Options{})
}

// buildAgentMqttBroker 构造一个全新的 MQTT 连接实例(失败重试时每次新实例:
// 传输库把失败状态固化在实例内,复用旧实例的重试会被缓存错误拒绝)。
func buildAgentMqttBroker(cfg *conf.Bootstrap, agentService *agent.Service) *mqtt.Server {
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

	return mqttBootstrap.NewMqttServer(cfg.Server.Mqtt, opts...)
}
