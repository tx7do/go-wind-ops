//go:build !windows

package agent

import "syscall"

// detachSysProcAttr Unix:新会话脱离父进程。
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
