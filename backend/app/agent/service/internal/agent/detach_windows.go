//go:build windows

package agent

import "syscall"

// detachSysProcAttr Windows:隐藏接力脚本窗口。
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
