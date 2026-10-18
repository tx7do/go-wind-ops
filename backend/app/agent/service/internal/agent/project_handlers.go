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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"go-wind-ops/pkg/shellexec"
	"go-wind-ops/pkg/signal"
)

const (
	pidFileName       = ".ops-project.pid"
	logFileName       = ".ops-project.log"
	outputLimit       = 64 << 10 // 回传输出上限 64KB
	startGracePeriod  = 800 * time.Millisecond
	shellWaitTimeout  = 30 * time.Second
	logReadDefault    = 64 << 10  // 日志读取默认尾部字节数
	logReadMax        = 256 << 10 // 日志读取上限
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
	s.RegisterHandler(signal.CommandActionProjectLog, s.handleProjectLog)
	s.RegisterHandler(signal.CommandActionProjectLogClear, s.handleProjectLogClear)
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
// 进程/Shell 工具(实现收敛到 pkg/shellexec;此处保留薄别名)
// ============================================================================

// handleProjectLog 读取项目运行日志尾部。
func (s *Service) handleProjectLog(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.ProjectLogPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.Dir == "" || payload.ProjectId == 0 {
		return nil, fmt.Errorf("payload.dir/project_id are required")
	}

	maxBytes := payload.MaxBytes
	if maxBytes <= 0 {
		maxBytes = logReadDefault
	}
	if maxBytes > logReadMax {
		maxBytes = logReadMax
	}

	logPath := filepath.Join(payload.Dir, logFileName)
	info, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return signal.ProjectLogResult{ProjectId: payload.ProjectId, Content: ""}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat project log: %w", err)
	}

	content, truncated, err := readLogFileTail(logPath, info.Size(), maxBytes)
	if err != nil {
		return nil, fmt.Errorf("read project log: %w", err)
	}

	return signal.ProjectLogResult{
		ProjectId: payload.ProjectId,
		Size:      info.Size(),
		Truncated: truncated,
		Content:   content,
	}, nil
}

// handleProjectLogClear 清空项目运行日志。
func (s *Service) handleProjectLogClear(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.ProjectLogPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.Dir == "" {
		return nil, fmt.Errorf("payload.dir is required")
	}

	if err := os.Truncate(filepath.Join(payload.Dir, logFileName), 0); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("truncate project log: %w", err)
	}
	return map[string]any{"cleared": true}, nil
}

// readLogFileTail 读文件尾部 maxBytes 字节。
func readLogFileTail(path string, size, maxBytes int64) (content string, truncated bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
		truncated = true
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return "", truncated, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", truncated, err
	}
	return string(data), truncated, nil
}

// startDetached 以分离进程执行启动命令:存活超过宽限期视为成功。
// 进程输出追加到 <dir>/.ops-project.log(不落内存——长驻进程的管道
// buffer 会无限增长);返回 pid、是否存活、启动即失败时的输出片段。
func startDetached(ctx context.Context, dir, command string) (int, bool, string, error) {
	// 目录缺失时自动创建(首次启动/分发前手动启动场景)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, false, "", fmt.Errorf("ensure project dir: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(dir, logFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, false, "", fmt.Errorf("open project log: %w", err)
	}
	defer logFile.Close()

	fmt.Fprintf(logFile, "\n===== start %s cmd: %s =====\n", time.Now().Format("2006-01-02 15:04:05"), command)

	cmd := shellexec.Command(command)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(logFile, "===== start failed: %s =====\n", err.Error())
		return 0, false, err.Error(), err
	}
	pid := cmd.Process.Pid
	fmt.Fprintf(logFile, "===== pid %d =====\n", pid)

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	select {
	case err := <-exited:
		// 启动即退出:回读日志尾部辅助排障
		return pid, false, readLogTail(dir, outputLimit), err
	case <-ctx.Done():
		_ = killTree(pid)
		return pid, false, readLogTail(dir, outputLimit), ctx.Err()
	case <-time.After(startGracePeriod):
		// 存活超过宽限期:启动成功;Wait 交给进程自行结束,避免僵尸
		go func() { <-exited }()
		return pid, true, "", nil
	}
}

// readLogTail 读取日志文件尾部(排障辅助)。
func readLogTail(dir string, maxBytes int64) string {
	data, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		return ""
	}
	if int64(len(data)) > maxBytes {
		data = data[int64(len(data))-maxBytes:]
	}
	return string(data)
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

// runShell 同步执行 shell 命令并限时(killTree 杀整棵进程树)。
func runShell(ctx context.Context, dir, command string, timeout time.Duration) (exitCode int, output string, timedOut bool, err error) {
	return shellexec.Run(ctx, dir, command, timeout)
}

// runShellAwait 同步执行 shell 命令(默认超时)。
func runShellAwait(ctx context.Context, dir, command string) (int, string, error) {
	exitCode, output, _, err := shellexec.Run(ctx, dir, command, shellWaitTimeout)
	return exitCode, output, err
}

func killTree(pid int) error {
	return shellexec.KillTree(pid)
}

func truncateOutput(s string) string {
	return shellexec.Truncate(s)
}
