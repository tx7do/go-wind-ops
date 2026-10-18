// Package shellexec 提供带超时与进程树终止的命令执行。
//
// Windows 陷阱一:exec.CommandContext 超时只杀 cmd.exe 外壳,孙进程(如 ping)
// 会继承输出管道继续运行,导致管道 EOF 永不到来、调用方卡死。因此超时统一
// 用定时器 + killTree(taskkill /T)杀整棵进程树。
//
// Windows 陷阱二:cmd /c 对命令串里的引号有自己的剥离规则,含引号/特殊字符
// 的 URL 会被拆坏。参数化程序(git 等)请用 RunGit/RunArgv 直传 argv,
// 不要走 shell。
package shellexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const (
	// OutputLimit 输出截断上限。
	OutputLimit = 64 << 10
)

// Command 平台 shell 包装(不挂 ctx 自动杀进程,超时由 Run 自己控)。
func Command(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", command)
	}
	return exec.Command("sh", "-c", command)
}

// Run 同步执行 shell 命令并限时,返回退出码、合并输出(截断到 OutputLimit)、
// 是否超时与错误。
func Run(ctx context.Context, dir, command string, timeout time.Duration) (exitCode int, output string, timedOut bool, err error) {
	return runCmd(ctx, dir, Command(command), timeout)
}

// RunGit 以 argv 直传执行 git 子命令(不经 shell,URL/路径无引号歧义)。
func RunGit(ctx context.Context, dir string, timeout time.Duration, args ...string) (exitCode int, output string, timedOut bool, err error) {
	return runCmd(ctx, dir, exec.Command("git", args...), timeout)
}

// RunArgv 以 argv 直接执行程序(不经 shell)。
func RunArgv(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) (exitCode int, output string, timedOut bool, err error) {
	return runCmd(ctx, dir, exec.Command(name, args...), timeout)
}

// KillTree 杀进程树(Windows 用 taskkill /T;Unix 直接杀)。
func KillTree(pid int) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(pid)).Run()
	}
	if p, err := os.FindProcess(pid); err == nil {
		return p.Kill()
	}
	return nil
}

// Truncate 输出截断。
func Truncate(s string) string {
	if len(s) <= OutputLimit {
		return s
	}
	return s[:OutputLimit]
}

// runCmd 执行已构造好的命令并限时。
func runCmd(ctx context.Context, dir string, cmd *exec.Cmd, timeout time.Duration) (exitCode int, output string, timedOut bool, err error) {
	if dir != "" {
		cmd.Dir = dir
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err = cmd.Start(); err != nil {
		return 0, Truncate(buf.String()), false, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timedOutCh := make(chan struct{}, 1)
	timer := time.AfterFunc(timeout, func() {
		_ = KillTree(cmd.Process.Pid)
		select {
		case timedOutCh <- struct{}{}:
		default:
		}
	})
	defer timer.Stop()

	select {
	case err = <-done:
	case <-ctx.Done():
		_ = KillTree(cmd.Process.Pid)
		<-done
		if cmd.ProcessState != nil {
			return cmd.ProcessState.ExitCode(), Truncate(buf.String()), false, ctx.Err()
		}
		return -1, Truncate(buf.String()), false, ctx.Err()
	}

	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	select {
	case <-timedOutCh:
		timedOut = true
	default:
	}
	return exitCode, Truncate(buf.String()), timedOut, err
}
