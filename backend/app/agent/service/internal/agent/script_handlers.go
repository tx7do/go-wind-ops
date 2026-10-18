// script_handlers.go 实现脚本远程执行指令(script_exec):
// 把 core 下发的脚本内容落地为临时文件,以平台 shell 执行,捕获合并输出
// 与退出码回传(记入指令台账)。脚本内容大小与超时上限由 core 侧校验。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go-wind-ops/pkg/signal"
)

// handleScriptExec 执行脚本。
func (s *Service) handleScriptExec(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.ScriptExecPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if strings.TrimSpace(payload.Content) == "" {
		return nil, fmt.Errorf("payload.content is required")
	}
	if payload.ScriptId == 0 {
		return nil, fmt.Errorf("payload.script_id is required")
	}

	timeout := payload.TimeoutSec
	if timeout == 0 {
		timeout = signal.ScriptExecTimeoutDefault
	}
	if timeout > signal.ScriptExecTimeoutMax {
		timeout = signal.ScriptExecTimeoutMax
	}

	scriptFile, cleanup, err := writeTempScript(payload.Content)
	if err != nil {
		return nil, fmt.Errorf("materialize script: %w", err)
	}
	defer cleanup()

	started := time.Now()
	timeoutDur := time.Duration(timeout) * time.Second

	exitCode, output, timedOut, err := runShell(ctx, "", quotePath(scriptFile), timeoutDur)

	result := signal.ScriptExecResult{
		ScriptId: payload.ScriptId,
		ExitCode: exitCode,
		Output:   truncateOutput(output),
		Duration: time.Since(started).Round(time.Millisecond).String(),
	}
	if timedOut {
		result.ExitCode = -1
		result.Output = truncateOutput(result.Output + "\n[timeout after "+strconv.FormatUint(uint64(timeout), 10)+"s]")
	}
	if err != nil && !timedOut && exitCode == 0 {
		// 非退出码失败(启动失败等),且无法从 ProcessState 取码:标记 -1
		result.ExitCode = -1
	}

	return result, nil
}

// writeTempScript 把脚本内容写入临时文件并赋予可执行位;返回清理函数。
func writeTempScript(content string) (path string, cleanup func(), err error) {
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".cmd"
	}
	f, err := os.CreateTemp("", "ops-script-*"+ext)
	if err != nil {
		return "", nil, err
	}
	path = f.Name()

	if _, err = f.WriteString(content); err != nil {
		f.Close()
		os.Remove(path)
		return "", nil, err
	}
	f.Close()

	if runtime.GOOS != "windows" {
		if err = os.Chmod(path, 0o700); err != nil {
			os.Remove(path)
			return "", nil, err
		}
	}

	return path, func() { os.Remove(path) }, nil
}

// quotePath 临时脚本路径执行时加引号(防路径空格)。
func quotePath(path string) string {
	if runtime.GOOS == "windows" {
		// cmd /c 对引号内的整条命令解析特殊;临时路径无空格场景居多,
		// 保守做法:仅路径含空格时加引号。
		if strings.ContainsAny(path, " \t") {
			return `"` + path + `"`
		}
		return path
	}
	return `"` + path + `"`
}
