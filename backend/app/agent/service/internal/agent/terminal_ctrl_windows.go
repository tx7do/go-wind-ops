//go:build windows

// terminal_ctrl_windows.go Web 终端 Ctrl+C 投递的 Windows 实现。
//
// 背景: ConPTY 的 VT 输入流不把 0x03 翻译成控制台事件(官方 WT 终端同样
// 直写 0x03,靠 conhost 行纪律处理;但运行中的前台子进程收不到),因此
// Ctrl+C 需经 AttachConsole+GenerateConsoleCtrlEvent 显式投递。
// 两个关键坑(实测):①SetConsoleCtrlHandler(NULL,TRUE) 的"忽略 CTRL_C"
// 标志会被 CreateProcess 遗传,起 cmd 前必须显式清除,否则整条 cmd→子进程
// 链都收不到 CTRL_C;②投递事件会波及 agent 自身,起完子进程要立即给自己
// 设回免疫。
package agent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole    = kernel32.NewProc("FreeConsole")
	procAttachConsole  = kernel32.NewProc("AttachConsole")
	procGenCtrlEvent   = kernel32.NewProc("GenerateConsoleCtrlEvent")
	procSetCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
)

// prepareChildCtrlC 起伪终端子进程前调用:清除本进程继承的"忽略 CTRL_C"
// 标志,让子进程链保持 Ctrl+C 可用。
func prepareChildCtrlC() {
	procSetCtrlHandler.Call(0, 0)
}

// restoreSelfCtrlC 子进程起好后调用:给 agent 自身设回免疫,避免后续投递
// CTRL_C 时波及自己。
func restoreSelfCtrlC() {
	procSetCtrlHandler.Call(0, 1)
}

// interruptForeground 向目标 pid 所在控制台投递 CTRL_C,中断其前台子进程。
// 调用方须持有 consoleMu。
func interruptForeground(targetPid int) error {
	procFreeConsole.Call()
	ret, _, err := procAttachConsole.Call(uintptr(targetPid))
	if ret == 0 {
		procFreeConsole.Call()
		return fmt.Errorf("attach console: %v", err)
	}
	_, _, _ = procGenCtrlEvent.Call(0, 0) // CTRL_C_EVENT, 0=当前控制台全部进程
	procFreeConsole.Call()
	return nil
}
