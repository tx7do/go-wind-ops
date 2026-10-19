// terminal_handlers.go 实现 Web 终端会话(terminal_open/terminal_close 指令
// + 通配输入/尺寸主题路由)。真 PTY:Windows=ConPTY,Unix=内核 pty——回显、
// 行编辑、Ctrl+C 与交互式程序均由伪终端原生支持。
package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
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

	pty       pty.Pty
	ptyCmd    *pty.Cmd
	shellName string

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

	// consoleMu 串行化 Windows 控制台状态操作(FreeConsole/AttachConsole/
	// CtrlHandler 是进程级状态,与 open/inject 并发会互踩)。
	consoleMu sync.Mutex
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

// handleTerminalOpen 打开会话:按平台起真 PTY shell 进程,挂输出泵。
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

	sess, err := s.terminal.open(payload.SessionId, payload.Cols, payload.Rows)
	if err != nil {
		return nil, err
	}
	s.log().Infof("terminal session opened (session=%s pid=%d shell=%s)",
		payload.SessionId, sess.ptyCmd.Process.Pid, sess.shellName)
	return map[string]any{
		"session_id": payload.SessionId,
		"pid":        sess.ptyCmd.Process.Pid,
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
	_ = shellexec.KillTree(sess.ptyCmd.Process.Pid)
	sess.cancel()
	s.log().Infof("terminal session closed (session=%s)", payload.SessionId)
	return map[string]any{"closed": payload.SessionId}, nil
}

// HandleTerminalInput 通配主题 ops/agent/{id}/terminal/+/input 的路由入口:
// 按 payload.session_id 找会话写入 PTY;会话不存在则静默丢弃。
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
	// Ctrl+C(0x03): ConPTY 的 VT 输入流不生成控制台事件,须向控制台显式
	// 投递才能中断运行中的前台子进程;0x03 本身仍透传 PTY(conhost 原生
	// 行纪律负责取消行编辑等)。
	if bytes.IndexByte(data, 0x03) >= 0 {
		s.terminal.consoleMu.Lock()
		if ierr := interruptForeground(sess.ptyCmd.Process.Pid); ierr != nil {
			s.log().Warnf("terminal ctrl+c deliver (session=%s pid=%d): %s",
				msg.SessionId, sess.ptyCmd.Process.Pid, ierr.Error())
		}
		s.terminal.consoleMu.Unlock()
	}
	if _, err = sess.pty.Write(data); err != nil {
		s.log().Warnf("terminal stdin write (session=%s): %s", msg.SessionId, err.Error())
	}
	return nil
}

// HandleTerminalResize 通配主题 ops/agent/{id}/terminal/+/resize 的路由入口:
// 调整 PTY 尺寸;会话不存在则静默丢弃。
func (s *Service) HandleTerminalResize(ctx context.Context, _ string, _ broker.Headers, msg *signal.TerminalResizePayload) error {
	if msg == nil || msg.SessionId == "" || msg.Cols == 0 || msg.Rows == 0 {
		return nil
	}
	sess := s.terminal.get(msg.SessionId)
	if sess == nil {
		return nil
	}
	if err := sess.pty.Resize(int(msg.Cols), int(msg.Rows)); err != nil {
		s.log().Warnf("terminal resize (session=%s cols=%d rows=%d): %s",
			msg.SessionId, msg.Cols, msg.Rows, err.Error())
	}
	return nil
}

// open 起真 PTY shell 并注册会话(Windows=ConPTY,Unix=内核 pty)。
func (m *terminalSessionManager) open(sessionId string, cols, rows uint32) (*terminalSession, error) {
	_, cancel := context.WithCancel(context.Background())

	pt, err := pty.New()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("new pty: %w", err)
	}

	var ptyCmd *pty.Cmd
	shellName := "cmd.exe"
	if runtime.GOOS == "windows" {
		ptyCmd = pt.Command("cmd.exe")
	} else {
		shellName = os.Getenv("SHELL")
		if shellName == "" {
			shellName = "/bin/bash"
		}
		ptyCmd = pt.Command(shellName, "-i")
	}
	ptyCmd.Env = os.Environ()

	// 清除本进程(可能继承自服务链)的"忽略 CTRL_C"标志再起子进程——该标志
	// 会遗传给 cmd→子进程整条链,导致 Ctrl+C 全链失效;起完立即给自身设回
	// 免疫(投递事件时不波及自己)。进程级状态,用 consoleMu 串行化。
	m.consoleMu.Lock()
	prepareChildCtrlC()
	startErr := ptyCmd.Start()
	restoreSelfCtrlC()
	m.consoleMu.Unlock()
	if startErr != nil {
		_ = pt.Close()
		cancel()
		return nil, fmt.Errorf("start %s: %w", shellName, startErr)
	}
	if cols > 0 && rows > 0 {
		_ = pt.Resize(int(cols), int(rows))
	}

	sess := &terminalSession{
		id:        sessionId,
		pty:       pt,
		ptyCmd:    ptyCmd,
		shellName: shellName,
		cancel:    cancel,
		lastActive: time.Now(),
	}
	if err = m.put(sess); err != nil {
		_ = shellexec.KillTree(ptyCmd.Process.Pid)
		_ = pt.Close()
		cancel()
		return nil, err
	}

	go m.pump(sess, pt)
	go m.waitLoop(sess)

	return sess, nil
}

// pump 输出泵:PTY 输出 → MQTT output 主题。
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

// waitLoop 进程退出:关 PTY、发 CLOSED 事件帧并清理会话表。
func (m *terminalSessionManager) waitLoop(sess *terminalSession) {
	_ = sess.ptyCmd.Wait()
	_ = sess.pty.Close()
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
			_ = shellexec.KillTree(sess.ptyCmd.Process.Pid)
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
