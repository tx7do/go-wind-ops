// terminal_service.go Web 终端 gRPC 双向通道:
// admin BFF(WS) ↔ 本服务(gRPC bidi) ↔ MQTT ↔ agent。
// 首帧 OPEN 开会话(terminal_open 经 command 主题同步等待回执),此后
// data 帧双向对传;流断开即 terminal_close 清理。输出经通配订阅 +
// 内存路由表分发到对应流。
package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/broker"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/signal"
	"go-wind-ops/pkg/topic"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const (
	// terminalOpenTimeout terminal_open 回执等待上限(进程启动通常亚秒)。
	terminalOpenTimeout = 20 * time.Second

	// terminalChunkMax 单帧字节上限(回显不宜过大,输出泵已按 4KB 切)。
	terminalChunkMax = 64 << 10
)

type TerminalService struct {
	opsV1.UnimplementedTerminalServiceServer

	log      *log.Helper
	agentSvc *AgentService
	nodeRepo *data.AgentNodeRepo

	sessionRepo *data.TerminalSessionRepo
	chunkRepo   *data.TerminalChunkRepo
}

func NewTerminalService(ctx *bootstrap.Context, agentSvc *AgentService, nodeRepo *data.AgentNodeRepo, sessionRepo *data.TerminalSessionRepo, chunkRepo *data.TerminalChunkRepo) *TerminalService {
	return &TerminalService{
		log:         ctx.NewLoggerHelper("terminal/service/core-service"),
		agentSvc:    agentSvc,
		nodeRepo:    nodeRepo,
		sessionRepo: sessionRepo,
		chunkRepo:   chunkRepo,
	}
}

// strPtr 指针简写。
func strPtr(v string) *string { return &v }

// Channel 终端双向流。
func (s *TerminalService) Channel(stream opsV1.TerminalService_ChannelServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.GetEvent() != "OPEN" || first.GetAgentId() == "" || first.GetSessionId() == "" {
		return opsV1.ErrorBadRequest("first chunk must be OPEN with agentId/sessionId")
	}
	agentId, sessionId := first.GetAgentId(), first.GetSessionId()
	if len(sessionId) > 64 {
		return opsV1.ErrorBadRequest("sessionId too long")
	}
	ctx := stream.Context()

	nodeID := uint32(0)
	if s.nodeRepo != nil {
		node, nerr := s.nodeRepo.GetByAgentID(ctx, agentId)
		if nerr != nil {
			return opsV1.ErrorNotFound("agent not enrolled: %s", agentId)
		}
		if node != nil {
			nodeID = node.ID
		}
	}

	// ── 先挂输出路由再开会话:shell 启动横幅早于 open 回执到达,后注册
	// 会把首批输出当"无路由"丢帧,前端将一直停在"正在连接"。──
	ch := s.agentSvc.RegisterTerminalRoute(agentId, sessionId)
	defer s.agentSvc.UnregisterTerminalRoute(agentId, sessionId)

	// ── 开会话:terminal_open 同步等待回执 ──
	_, err = s.agentSvc.PublishCommandSync(ctx, 0, agentId,
		signal.CommandActionTerminalOpen,
		&signal.TerminalOpenPayload{SessionId: sessionId}, operatorIDOf(ctx), terminalOpenTimeout)
	if err != nil {
		s.log.Warnf("terminal open failed (agent=%s session=%s): %s", agentId, sessionId, err.Error())
		_ = stream.Send(&opsV1.TerminalChunk{Event: strPtr("CLOSED"), Error: strPtr("open failed: " + err.Error())})
		return nil
	}
	// 流结束(含 WS 断开)时关闭会话。
	defer func() {
		_, _ = s.agentSvc.PublishCommand(ctx, 0, agentId,
			signal.CommandActionTerminalClose,
			&signal.TerminalClosePayload{SessionId: sessionId}, operatorIDOf(ctx))
	}()
	if err = stream.Send(&opsV1.TerminalChunk{Event: strPtr("OPENED")}); err != nil {
		return nil
	}
	s.log.Infof("terminal session opened (agent=%s session=%s operator=%d)", agentId, sessionId, operatorIDOf(ctx))

	// ── 会话录制:异步落库,失败不影响终端 ──
	var rec *TerminalRecorder
	if s.sessionRepo != nil && s.chunkRepo != nil {
		rec = NewTerminalRecorder(ctx, s.sessionRepo, s.chunkRepo, s.log, sessionId, agentId, nodeID, operatorIDOf(ctx))
	}
	defer rec.Close()

	var sendMu sync.Mutex
	send := func(chunk *opsV1.TerminalChunk) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(chunk)
	}

	// MQTT 输出 → gRPC
	go func() {
		for {
			select {
			case out, ok := <-ch:
				if !ok {
					return
				}
				chunk := &opsV1.TerminalChunk{}
				if out.Closed {
					chunk.Event = strPtr("CLOSED")
				} else {
					data, derr := base64.StdEncoding.DecodeString(out.Data)
					if derr != nil {
						continue
					}
					rec.Output(data)
					chunk.Data = data
				}
				if serr := send(chunk); serr != nil {
					return
				}
				if out.Closed {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// gRPC 输入 → MQTT
	for {
		in, rerr := stream.Recv()
		if rerr != nil {
			return nil
		}
		if in.GetEvent() == "CLOSE" {
			return nil
		}
		if in.GetEvent() == "RESIZE" {
			if perr := s.agentSvc.PublishTerminalResize(ctx, agentId, sessionId, in.GetCols(), in.GetRows()); perr != nil {
				s.log.Warnf("terminal resize publish (agent=%s session=%s): %s", agentId, sessionId, perr.Error())
			}
			continue
		}
		data := in.GetData()
		if len(data) == 0 {
			continue
		}
		if len(data) > terminalChunkMax {
			data = data[:terminalChunkMax]
		}
		rec.Input(data)
		if perr := s.agentSvc.PublishTerminalInput(ctx, agentId, sessionId, data); perr != nil {
			s.log.Warnf("terminal input publish (agent=%s session=%s): %s", agentId, sessionId, perr.Error())
		}
	}
}

// ── 路由表(AgentService 扩展):终端输出通配订阅 → 每流一个 chan ──

func terminalRouteKey(agentId, sessionId string) string {
	return agentId + "/" + sessionId
}

// RegisterTerminalRoute 注册一条流的输出通道。
func (s *AgentService) RegisterTerminalRoute(agentId, sessionId string) <-chan *signal.TerminalIO {
	ch := make(chan *signal.TerminalIO, 128)
	s.terminalRoutesMu.Lock()
	s.terminalRoutes[terminalRouteKey(agentId, sessionId)] = ch
	s.terminalRoutesMu.Unlock()
	return ch
}

// UnregisterTerminalRoute 注销流的输出通道(不 close,接收方经 ctx 退出)。
func (s *AgentService) UnregisterTerminalRoute(agentId, sessionId string) {
	s.terminalRoutesMu.Lock()
	delete(s.terminalRoutes, terminalRouteKey(agentId, sessionId))
	s.terminalRoutesMu.Unlock()
}

// HandleTerminalOutput 通配主题 ops/agent/+/terminal/+/output 的路由入口:
// 从主题段解析 agentId,按路由表分发;无路由(会话不在此副本)则丢弃。
func (s *AgentService) HandleTerminalOutput(ctx context.Context, topicStr string, _ broker.Headers, msg *signal.TerminalIO) error {
	if msg == nil || msg.SessionId == "" {
		return nil
	}
	agentId := segmentOf(topicStr, 2)
	if agentId == "" {
		return nil
	}
	s.terminalRoutesMu.Lock()
	ch, ok := s.terminalRoutes[terminalRouteKey(agentId, msg.SessionId)]
	s.terminalRoutesMu.Unlock()
	if !ok {
		return nil
	}
	select {
	case ch <- msg:
	default: // 背压满即丢帧,终端回显可容忍
	}
	return nil
}

// PublishTerminalInput 发布键盘输入到 input 主题。
func (s *AgentService) PublishTerminalInput(ctx context.Context, agentId, sessionId string, data []byte) error {
	b, ok := s.mqttBroker.Load().(broker.Broker)
	if !ok || b == nil {
		return fmt.Errorf("mqtt broker not ready")
	}
	return b.Publish(ctx, topic.MakeMqttTopicTerminalInput(agentId, sessionId),
		broker.NewMessage(&signal.TerminalIO{
			SessionId: sessionId,
			Data:      base64.StdEncoding.EncodeToString(data),
		}))
}

// PublishTerminalResize 发布 PTY 尺寸变更到 resize 主题。
func (s *AgentService) PublishTerminalResize(ctx context.Context, agentId, sessionId string, cols, rows uint32) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	b, ok := s.mqttBroker.Load().(broker.Broker)
	if !ok || b == nil {
		return fmt.Errorf("mqtt broker not ready")
	}
	return b.Publish(ctx, topic.MakeMqttTopicTerminalResize(agentId, sessionId),
		broker.NewMessage(&signal.TerminalResizePayload{
			SessionId: sessionId,
			Cols:      cols,
			Rows:      rows,
		}))
}

// segmentOf 取 "/" 分隔主题的第 idx 段(ops/agent/{2}=agentId/...)。
func segmentOf(topicStr string, idx int) string {
	parts := strings.Split(topicStr, "/")
	if idx < 0 || idx >= len(parts) {
		return ""
	}
	return parts[idx]
}
