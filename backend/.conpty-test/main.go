package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/windows"
)

var consoleMu sync.Mutex

func init() {
	// agent 服务自身对 Ctrl+C 免疫(否则投递事件会把自己带上)
	windows.SetConsoleCtrlHandler(0, 1)
}

func sendCtrlC(targetPid uint32) error {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	_ = windows.FreeConsole()
	if err := windows.AttachConsole(targetPid); err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0)
	_ = windows.FreeConsole()
	return err
}

func main() {
	p, _ := pty.New()
	defer p.Close()
	c := p.Command("cmd.exe")
	c.Env = os.Environ()
	c.Start()
	var sb strings.Builder
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	time.Sleep(1400 * time.Millisecond)
	p.Write([]byte("ping -n 60 127.0.0.1\r"))
	time.Sleep(2200 * time.Millisecond)
	before := sb.Len()
	if err := sendCtrlC(c.Process.Pid); err != nil {
		fmt.Println("sendCtrlC err:", err)
	}
	time.Sleep(2500 * time.Millisecond)
	after := sb.String()[before:]
	ok := strings.Contains(after, "^C") || regexp.MustCompile(`[A-Za-z]:\[^>]*>`).MatchString(after)
	fmt.Printf("ATTACH-CTRLC killed=%v tail=%q\n", ok, func() string {
		if len(after) > 150 {
			return after[len(after)-150:]
		}
		return after
	}())
}
