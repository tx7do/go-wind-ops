//go:build !windows

// terminal_ctrl_other.go Web 终端 Ctrl+C 的非 Windows 空实现:Unix 内核
// pty 的行纪律原生把 0x03 翻成 SIGINT,无需额外投递。
package agent

// prepareChildCtrlC Windows 专用,其余平台无操作。
func prepareChildCtrlC() {}

// restoreSelfCtrlC Windows 专用,其余平台无操作。
func restoreSelfCtrlC() {}

// interruptForeground 其余平台无操作(^C 经 PTY 透传即达)。
func interruptForeground(targetPid int) error { return nil }
