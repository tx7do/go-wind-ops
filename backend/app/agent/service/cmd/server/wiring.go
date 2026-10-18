package main

import (
	"errors"

	"github.com/go-kratos/kratos/v2"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/agent/service/internal/agent"
	"go-wind-ops/app/agent/service/internal/server"
)

// initApp 手写装配整个应用,是本服务的依赖注入点(替代 google/wire 生成)。
//
// agent 是边缘进程:无数据库、无服务发现注册、无 REST/gRPC 入口,
// 仅一条到 EMQX 的 MQTT 出站连接。
func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	agentSvc := agent.NewService(ctx)

	mqttServer := server.NewAgentMqttServer(ctx, agentSvc)
	if mqttServer == nil {
		return nil, nil, errors.New("mqtt config missing: agent service requires server.mqtt section in server.yaml")
	}

	return newApp(ctx, mqttServer), agentSvc.Stop, nil
}
