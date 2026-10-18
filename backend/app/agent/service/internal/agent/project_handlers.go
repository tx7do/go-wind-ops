// project_handlers.go 实现项目生命周期指令(project_start/stop/restart/status):
// 在项目目录内以 shell 执行启动/停止/状态命令。安全基线:项目目录必须落在
// AGENT_ALLOWED_DIRS 授权目录列表内(环境变量,路径分隔符为平台列表分隔符),
// 否则拒绝执行——对应 jpom 的 whitelistDirectory。
//
// 进程管理 v1 语义:
//   - start:以分离进程执行启动命令,存活超过宽限期视为启动成功,记 pid 文件
//     (<项目目录>/.ops-project.pid);宽限期内退出视为失败并回传输出。
//   - stop:优先执行停止命令(同步等待);未配置时按 pid 文件杀进程树。
//   - status:优先执行状态命令(退出码 0 视为运行中);未配置时按 pid 存活判断。
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"go-wind-ops/pkg/signal"
)

const (
	pidFileName       = ".ops-project.pid"
	outputLimit       = 64 << 10 // 回传输出上限 64KB
	startGracePeriod  = 800 * time.Millisecond
	shellWaitTimeout  = 30 * time.Second
)

// LoadAllowedDirs 读取授权目录列表(AGENT_ALLOWED_DIRS,平台列表分隔符分隔)。
func LoadAllowedDirs() []string {
	raw := os.Getenv("AGENT_ALLOWED_DIRS")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, string(os.PathListSeparator))
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, filepath.Clean(p))
		}
	}
	return out
}

// registerProjectHandlers 注册项目生命周期指令处理器。
func (s *Service) registerProjectHandlers() {
	s.RegisterHandler(signal.CommandActionProjectStart, s.handleProjectStart)
	s.RegisterHandler(signal.CommandActionProjectStop, s.handleProjectStop)
	s.RegisterHandler(signal.CommandActionProjectRestart, s.handleProjectRestart)
	s.RegisterHandler(signal.CommandActionProjectStatus, s.handleProjectStatus)
}

// checkDir 校验项目目录在授权列表内(授权列表未配置则全部拒绝)。
func (s *Service) checkDir(payload *signal.ProjectOperationPayload) error {
	dir := filepath.Clean(payload.Dir)
	allowed := LoadAllowedDirs()
	for _, base := range allowed {
		if isWithinBase(dir, base) {
			return nil
		}
	}
	if len(allowed) == 0 {
		return fmt.Errorf("project operations denied: AGENT_ALLOWED_DIRS not configured")
	}
	return fmt.Errorf("project dir %q is outside allowed directories", payload.Dir)
}

// isWithinBase 判断 dir 是否等于 base 或位于 base 之下(大小写不敏感,Windows)。
func isWithinBase(dir, base string) bool {
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir)
		base = strings.ToLower(base)
	}
	rel, err := filepath.Rel(base, dir)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// decodeProjectPayload 解析并校验项目操作载荷。
func decodeProjectPayload(env *signal.CommandEnvelope) (*signal.ProjectOperationPayload, error) {
	payload := &signal.ProjectOperationPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.Dir == "" {
		return nil, fmt.Errorf("payload.dir is required")
	}
	if payload.ProjectId == 0 {
		return nil, fmt.Errorf("payload.project_id is required")
	}
	return payload, nil
}

func (s *Service) handleProjectStart(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	payload, err := decodeProjectPayload(env)
	if err != nil {
		return nil, err
	}
	if err := s.checkDir(payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.StartCommand) == "" {
		return nil, fmt.Errorf("start_command is not configured for project %q", payload.Name)
	}

	pid, alive, output, err := startDetached(ctx, payload.Dir, payload.StartCommand)
	if err != nil {
		return nil, fmt.Errorf("start: %s: %s", err.Error(), truncateOutput(output))
	}
	if !alive {
		return nil, fmt.Errorf("start command exited immediately (see output): %s", truncateOutput(output))
	}

	_ = os.WriteFile(filepath.Join(payload.Dir, pidFileName), []byte(strconv.Itoa(pid)), 0o600)

	return signal.ProjectOperationResult{
		ProjectId: payload.ProjectId,
		Operation: "start",
		Pid:       uint64(pid),
		Alive:     true,
		Status:    "RUNNING",
		Output:    truncateOutput(output),
	}, nil
}

func (s *Service) handleProjectStop(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	payload, err := decodeProjectPayload(env)
	if err != nil {
		return nil, err
	}
	if err := s.checkDir(payload); err != nil {
		return nil, err
	}

	var output string
	var exitCode int

	// 优先执行配置的停止命令(同步等待)
	if strings.TrimSpace(payload.StopCommand) != "" {
		exitCode, output, err = runShellAwait(ctx, payload.Dir, payload.StopCommand)
		if err != nil {
			return nil, fmt.Errorf("stop command failed: %s: %s", err.Error(), truncateOutput(output))
		}
	}

	// pid 文件兜底:确认死透/补杀
	pid := readPidFile(payload.Dir)
	if pid > 0 {
		if alive, _ := process.PidExists(int32(pid)); alive {
			_ = killTree(pid)
			time.Sleep(300 * time.Millisecond)
		}
		_ = os.Remove(filepath.Join(payload.Dir, pidFileName))
	}

	return signal.ProjectOperationResult{
		ProjectId: payload.ProjectId,
		Operation: "stop",
		ExitCode:  exitCode,
		Alive:     false,
		Status:    "STOPPED",
		Output:    truncateOutput(output),
	}, nil
}

func (s *Service) handleProjectRestart(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	// 先尽力停止(不因"本就未运行"而失败)
	if _, err := s.handleProjectStop(ctx, env); err != nil {
		s.log().Warnf("project restart: stop phase: %v", err)
	}
	return s.handleProjectStart(ctx, env)
}

func (s *Service) handleProjectStatus(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	payload, err := decodeProjectPayload(env)
	if err != nil {
		return nil, err
	}
	if err := s.checkDir(payload); err != nil {
		return nil, err
	}

	result := signal.ProjectOperationResult{
		ProjectId: payload.ProjectId,
		Operation: "status",
	}

	pid := readPidFile(payload.Dir)
	result.Pid = uint64(pid)

	if strings.TrimSpace(payload.StatusCommand) != "" {
		exitCode, output, err := runShellAwait(ctx, payload.Dir, payload.StatusCommand)
		if err != nil && exitCode == 0 {
			return nil, fmt.Errorf("status command failed: %s", err.Error())
		}
		result.ExitCode = exitCode
		result.Output = truncateOutput(output)
		result.Alive = err == nil && exitCode == 0
	} else {
		alive := false
		if pid > 0 {
			alive, _ = process.PidExists(int32(pid))
		}
		result.Alive = alive
	}

	if result.Alive {
		result.Status = "RUNNING"
	} else {
		result.Status = "STOPPED"
	}

	return result, nil
}

// ============================================================================
// 进程/Shell 工具
// ============================================================================

// shellCommand 平台 shell 包装(带超时上下文)。
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/c", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

// startDetached 以分离进程执行启动命令:存活超过宽限期视为成功。
// 返回 pid、是否存活、宽限期内捕获的输出。
func startDetached(ctx context.Context, dir, command string) (int, bool, string, error) {
	cmd := shellCommand(context.WithoutCancel(ctx), command)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return 0, false, buf.String(), err
	}
	pid := cmd.Process.Pid

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	select {
	case err := <-exited:
		return pid, false, buf.String(), err
	case <-ctx.Done():
		_ = killTree(pid)
		return pid, false, buf.String(), ctx.Err()
	case <-time.After(startGracePeriod):
		// 存活超过宽限期:启动成功;Wait 交给进程自行结束,避免僵尸
		go func() { <-exited }()
		return pid, true, buf.String(), nil
	}
}

// runShellAwait 同步执行 shell 命令(默认超时)。
func runShellAwait(ctx context.Context, dir, command string) (int, string, error) {
	exitCode, output, _, err := runShell(ctx, dir, command, shellWaitTimeout)
	return exitCode, output, err
}

// runShell 同步执行 shell 命令并限时,返回退出码、合并输出(截断)、是否超时
// 与错误。超时用 killTree 杀整棵进程树:Windows 上 exec.CommandContext 只杀
// cmd.exe 外壳,孙进程(如 ping)会继承管道继续运行,导致输出管道永远等不到
// EOF。
func runShell(ctx context.Context, dir, command string, timeout time.Duration) (exitCode int, output string, timedOut bool, err error) {
	cmd := shellCommand(context.WithoutCancel(ctx), command)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err = cmd.Start(); err != nil {
		return 0, buf.String(), false, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timedOutCh := make(chan struct{}, 1)
	timer := time.AfterFunc(timeout, func() {
		_ = killTree(cmd.Process.Pid)
		select {
		case timedOutCh <- struct{}{}:
		default:
		}
	})
	defer timer.Stop()

	select {
	case err = <-done:
	case <-ctx.Done():
		_ = killTree(cmd.Process.Pid)
		<-done
		return cmd.ProcessState.ExitCode(), buf.String(), false, ctx.Err()
	}

	exitCode = 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	select {
	case <-timedOutCh:
		timedOut = true
	default:
	}
	return exitCode, buf.String(), timedOut, err
}

func readPidFile(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, pidFileName))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// killTree 杀进程树(Windows 用 taskkill /T;Unix 直接杀)。
func killTree(pid int) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	}
	if p, err := os.FindProcess(pid); err == nil {
		return p.Kill()
	}
	return nil
}

func truncateOutput(s string) string {
	if len(s) <= outputLimit {
		return s
	}
	return s[:outputLimit]
}
