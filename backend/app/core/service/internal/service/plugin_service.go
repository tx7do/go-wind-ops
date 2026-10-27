// plugin_service.go 插件化指令:清单查询与远程执行(经 MQTT 同步指令代理)。
package service

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const (
	pluginSyncTimeout   = 15 * time.Second
	pluginExecTimeoutMax = 600 * time.Second
)

// pluginActionPattern 插件 action 白名单(与 agent 侧文件名主干约束一致)。
var pluginActionPattern = regexp.MustCompile(`^plugin_[a-z][a-z0-9_]{0,39}$`)

type PluginService struct {
	opsV1.UnimplementedPluginServiceServer

	log      *log.Helper
	agentSvc *AgentService
	nodeRepo *data.AgentNodeRepo

	notifChannelRepo *data.NotificationChannelRepo
	deliveryRepo     *data.NotificationDeliveryRepo
	maintenanceRepo  *data.MaintenanceWindowRepo
	approvalRepo *data.ApprovalRepo
}

func NewPluginService(ctx *bootstrap.Context, agentSvc *AgentService, nodeRepo *data.AgentNodeRepo, approvalRepo *data.ApprovalRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, maintenanceRepo *data.MaintenanceWindowRepo) *PluginService {
	return &PluginService{
		log:          ctx.NewLoggerHelper("plugin/service/core-service"),
		agentSvc:     agentSvc,
		nodeRepo:     nodeRepo,
		approvalRepo: approvalRepo,
		notifChannelRepo: notifChannelRepo,
		deliveryRepo:     deliveryRepo,
		maintenanceRepo:  maintenanceRepo,
	}
}

// resolveAgent 校验节点已注册。
func (s *PluginService) resolveAgent(ctx context.Context, agentId string) (string, error) {
	if agentId == "" {
		return "", opsV1.ErrorBadRequest("agentId required")
	}
	if s.nodeRepo != nil {
		if _, err := s.nodeRepo.GetByAgentID(ctx, agentId); err != nil {
			return "", opsV1.ErrorNotFound("agent not enrolled: %s", agentId)
		}
	}
	return agentId, nil
}

// ListAgentPlugins 查询插件清单。
func (s *PluginService) ListAgentPlugins(ctx context.Context, req *opsV1.ListAgentPluginsRequest) (*opsV1.ListAgentPluginsResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	agentId, err := s.resolveAgent(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.agentSvc.PublishCommandSync(ctx, 0, agentId, signal.CommandActionPluginList, nil, operatorIDOf(ctx), pluginSyncTimeout)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("plugin list failed: %s", err.Error())
	}
	if res == nil || !res.Success {
		msg := "unknown error"
		if res != nil && res.Error != "" {
			msg = res.Error
		}
		return nil, opsV1.ErrorInternalServerError("agent rejected plugin_list: %s", msg)
	}

	var out signal.PluginListResult
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode plugin list: %s", err.Error())
	}

	resp := &opsV1.ListAgentPluginsResponse{Dir: &out.Dir}
	for _, p := range out.Plugins {
		action, file, kind, modified := p.Action, p.File, p.Kind, p.Modified
		size := p.Size
		resp.Plugins = append(resp.Plugins, &opsV1.AgentPlugin{
			Action:   &action,
			File:     &file,
			Kind:     &kind,
			Size:     &size,
			Modified: &modified,
		})
	}
	return resp, nil
}

// ExecAgentPlugin 远程执行插件(带白名单校验,超时=请求值+15s 余量)。
func (s *PluginService) ExecAgentPlugin(ctx context.Context, req *opsV1.ExecAgentPluginRequest) (*opsV1.ExecAgentPluginResponse, error) {
	if req == nil || req.GetAction() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: action required")
	}
	if !pluginActionPattern.MatchString(req.GetAction()) {
		return nil, opsV1.ErrorBadRequest("invalid action name")
	}
	agentId, err := s.resolveAgent(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	timeoutSec := req.GetTimeoutSeconds()
	if timeoutSec == 0 {
		timeoutSec = 60
	}
	if timeoutSec > uint32(pluginExecTimeoutMax/time.Second) {
		timeoutSec = uint32(pluginExecTimeoutMax / time.Second)
	}

	// 审批闸门:APPROVAL_REQUIRED_ACTIONS 含该插件动作时先建审批单待批
	if plJSON, merr := json.Marshal(&signal.PluginExecPayload{Args: req.GetArgs(), TimeoutSec: timeoutSec}); merr == nil {
		if aerr := approvalGate(ctx, s.log, s.approvalRepo, s.notifChannelRepo, s.deliveryRepo, s.maintenanceRepo, string(req.GetAction()),
			0, agentId, req.GetAction(), string(plJSON), operatorIDOf(ctx)); aerr != nil {
			return nil, aerr
		}
	}

	res, err := s.agentSvc.PublishCommandSync(ctx, 0, agentId,
		signal.CommandAction(req.GetAction()),
		&signal.PluginExecPayload{Args: req.GetArgs(), TimeoutSec: timeoutSec},
		operatorIDOf(ctx), time.Duration(timeoutSec)*time.Second+15*time.Second)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("plugin exec failed: %s", err.Error())
	}
	if res == nil || !res.Success {
		msg := "unknown error"
		if res != nil && res.Error != "" {
			msg = res.Error
		}
		return nil, opsV1.ErrorInternalServerError("agent rejected %s: %s", req.GetAction(), msg)
	}

	var out signal.PluginExecResult
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode plugin result: %s", err.Error())
	}

	exitCode := int32(out.ExitCode)
	return &opsV1.ExecAgentPluginResponse{
		Action:   &out.Action,
		ExitCode: &exitCode,
		Output:   &out.Output,
		Duration: &out.Duration,
	}, nil
}
