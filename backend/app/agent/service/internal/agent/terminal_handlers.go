// terminal_handlers.go 实现 Web 终端会话(terminal_open/terminal_close 指令
// + 通配输入主题路由)。管道伪终端:进程 stdin/stdout/stderr 直连,不经
// shell 解释器包装;不支持交互式全屏程序(vim/htop)与 Ctrl+C 控制台事件。
package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/tx7do/kratos-transport/broker"

	"go-wind-ops/pkg/shellexec"
	"go-wind-ops/pkg/signal"
	"go-wind-ops/pkg/topic"
)

// terminalIdleTimeout 会话空闲上限(无输入即视为闲置)。
const terminalIdleTimeout = 15 * time.Minute

// terminalSession 一个存活中的终端会话。
type terminalSession struct {
	id string

	cmd       *exec.Cmd
	shellName string
	stdin     interface{ Write([]byte) (int, error) }

	cancel context.CancelFunc

	mu         sync.Mutex
	lastActive time.Time
}

func (t *terminalSession) touch() {
	t.mu.Lock()
	t.lastActive = time.Now()
	t.mu.Unlock()
}

func (t *terminalSession) idleFor() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return time.Since(t.lastActive)
}

// terminalSessionManager 会话表与输出发布。
type terminalSessionManager struct {
	svc *Service

	mu       sync.Mutex
	sessions map[string]*terminalSession
}

func newTerminalSessionManager(svc *Service) *terminalSessionManager {
	return &terminalSessionManager{svc: svc, sessions: make(map[string]*terminalSession)}
}

func (m *terminalSessionManager) get(sessionId string) *terminalSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[sessionId]
}

func (m *terminalSessionManager) put(sess *terminalSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sessions) >= signal.TerminalSessionMax {
		return fmt.Errorf("too many terminal sessions (max %d)", signal.TerminalSessionMax)
	}
	m.sessions[sess.id] = sess
	return nil
}

func (m *terminalSessionManager) remove(sessionId string) {
	m.mu.Lock()
	delete(m.sessions, sessionId)
	m.mu.Unlock()
}

func (m *terminalSessionManager) listIdle() []*terminalSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	var idle []*terminalSession
	for _, s := range m.sessions {
		if s.idleFor() > terminalIdleTimeout {
			idle = append(idle, s)
		}
	}
	return idle
}

// registerTerminalHandlers 注册终端开关指令。
func (s *Service) registerTerminalHandlers() {
	if s.terminal == nil {
		s.terminal = newTerminalSessionManager(s)
	}
	s.RegisterHandler(signal.CommandActionTerminalOpen, s.handleTerminalOpen)
	s.RegisterHandler(signal.CommandActionTerminalClose, s.handleTerminalClose)

	go s.terminal.gcLoop()
}

// handleTerminalOpen 打开会话:按平台起 shell 进程,接管道,挂输出泵。
func (s *Service) handleTerminalOpen(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.TerminalOpenPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.SessionId == "" {
		return nil, fmt.Errorf("payload.session_id required")
	}
	if s.terminal.get(payload.SessionId) != nil {
		return nil, fmt.Errorf("session %s already exists", payload.SessionId)
	}

	sess, err := s.terminal.open(payload.SessionId)
	if err != nil {
		return nil, err
	}
	s.log().Infof("terminal session opened (session=%s pid=%d shell=%s)",
		payload.SessionId, sess.cmd.Process.Pid, sess.shellName)
	return map[string]any{
		"session_id": payload.SessionId,
		"pid":        sess.cmd.Process.Pid,
		"shell":      sess.shellName,
	}, nil
}

// handleTerminalClose 关闭会话(杀整棵进程树,幂等)。
func (s *Service) handleTerminalClose(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.TerminalClosePayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.SessionId == "" {
		return nil, fmt.Errorf("payload.session_id required")
	}

	sess := s.terminal.get(payload.SessionId)
	if sess == nil {
		return map[string]any{"closed": payload.SessionId}, nil
	}
	_ = shellexec.KillTree(sess.cmd.Process.Pid)
	sess.cancel()
	s.log().Infof("terminal session closed (session=%s)", payload.SessionId)
	return map[string]any{"closed": payload.SessionId}, nil
}

// HandleTerminalInput 通配主题 ops/agent/{id}/terminal/+/input 的路由入口:
// 按 payload.session_id 找会话写入 stdin;会话不存在则静默丢弃。
func (s *Service) HandleTerminalInput(ctx context.Context, _ string, _ broker.Headers, msg *signal.TerminalIO) error {
	if msg == nil || msg.SessionId == "" || msg.Data == "" {
		return nil
	}
	sess := s.terminal.get(msg.SessionId)
	if sess == nil {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return nil
	}
	sess.touch()
	if _, err = sess.stdin.Write(data); err != nil {
		s.log().Warnf("terminal stdin write (session=%s): %s", msg.SessionId, err.Error())
	}
	return nil
}

// open 起进程并注册会话。
func (m *terminalSessionManager) open(sessionId string) (*terminalSession, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var cmd *exec.Cmd
	shellName := "cmd.exe"
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe")
	} else {
		shellName = os.Getenv("SHELL")
		if shellName == "" {
			shellName = "/bin/bash"
		}
		cmd = exec.CommandContext(ctx, shellName, "-i")
	}
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err = cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", shellName, err)
	}

	sess := &terminalSession{
		id:         sessionId,
		cmd:        cmd,
		shellName:  shellName,
		stdin:      stdin,
		cancel:     cancel,
		lastActive: time.Now(),
	}
	if err = m.put(sess); err != nil {
		_ = shellexec.KillTree(cmd.Process.Pid)
		cancel()
		return nil, err
	}

	go m.pump(sess, stdout)
	go m.pump(sess, stderr)
	go m.waitLoop(sess)

	return sess, nil
}

// pump 输出泵:进程输出 → MQTT output 主题。
func (m *terminalSessionManager) pump(sess *terminalSession, r interface{ Read([]byte) (int, error) }) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			m.svc.publishTerminalIO(sess.id, buf[:n], false)
		}
		if err != nil {
			return
		}
	}
}

// waitLoop 进程退出:发 CLOSED 事件帧并清理会话表。
func (m *terminalSessionManager) waitLoop(sess *terminalSession) {
	_ = sess.cmd.Wait()
	m.svc.publishTerminalIO(sess.id, nil, true)
	m.remove(sess.id)
}

// gcLoop 空闲会话回收。
func (m *terminalSessionManager) gcLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		for _, sess := range m.listIdle() {
			m.svc.log().Infof("terminal session idle timeout, closing (session=%s)", sess.id)
			_ = shellexec.KillTree(sess.cmd.Process.Pid)
			sess.cancel()
		}
	}
}

// publishTerminalIO 发布终端字节帧到 output 主题。
func (s *Service) publishTerminalIO(sessionId string, data []byte, closed bool) {
	b, ok := s.mqttBroker.Load().(broker.Broker)
	if !ok || b == nil {
		return
	}
	_ = b.Publish(context.Background(),
		topic.MakeMqttTopicTerminalOutput(s.AgentID(), sessionId),
		broker.NewMessage(&signal.TerminalIO{
			SessionId: sessionId,
			Data:      base64.StdEncoding.EncodeToString(data),
			Closed:    closed,
		}))
}
