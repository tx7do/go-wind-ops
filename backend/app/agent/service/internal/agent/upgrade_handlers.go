// upgrade_handlers.go 实现 agent_upgrade 指令:下载新二进制 → 回执 →
// 延迟自替换重启。替换由接力脚本完成:等当前进程退出 → 旧二进制改名腾位 →
// 新二进制落回原路径 → 以原参数重启 → 清理。回执先于退出发送(3s 余量)。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-wind-ops/pkg/filetransfer"
	"go-wind-ops/pkg/signal"
)

// registerAgentUpgradeHandlers 注册 agent 升级指令处理器。
func (s *Service) registerAgentUpgradeHandlers() {
	s.RegisterHandler(signal.CommandActionAgentUpgrade, s.handleAgentUpgrade)
}

// handleAgentUpgrade 下载新二进制并排程自替换;回执里的版本为目标版本号
// (实际版本以重启后心跳上报为准)。
func (s *Service) handleAgentUpgrade(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	var p signal.AgentUpgradePayload
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
	}
	if p.Url == "" {
		return nil, fmt.Errorf("url required")
	}

	ext := ".bin"
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	tmpFile, err := os.CreateTemp("", "ops-agent-upgrade-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	// 下载失败时自行清理;成功后由接力脚本删除
	downloaded := false
	defer func() {
		if !downloaded {
			_ = os.Remove(tmpPath)
		}
	}()

	size, sha256hex, err := filetransfer.DownloadToFile(ctx, p.Url, tmpPath)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	downloaded = true

	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}
	exePath = filepath.Clean(exePath)
	wd, _ := os.Getwd()

	// 回执由 handleCommand 在本函数返回后发布;3s 后退出并接力替换
	go func() {
		time.Sleep(3 * time.Second)
		s.log().Infof("agent upgrade: exiting for self-replace (version=%s size=%d)", p.Version, size)
		if err := spawnUpgradeScript(exePath, wd, tmpPath); err != nil {
			s.log().Errorf("agent upgrade script spawn failed: %s", err.Error())
			_ = os.Remove(tmpPath)
			os.Exit(1)
		}
		os.Exit(3)
	}()

	s.log().Infof("agent upgrade downloaded (version=%s size=%d sha256=%s), replacing in 3s",
		p.Version, size, sha256hex[:12])
	return &signal.AgentUpgradeResult{
		Version:   p.Version,
		SizeBytes: size,
		SHA256:    sha256hex,
		Output:    "downloaded; self-replace scheduled",
	}, nil
}

// spawnUpgradeScript 生成接力脚本并以脱离会话的方式启动。
func spawnUpgradeScript(exePath, wd, newPath string) error {
	var scriptPath string
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		scriptPath = newPath + ".upgrade.bat"
		bat := "@echo off\r\n" +
			"ping -n 3 127.0.0.1 >nul\r\n" +
			fmt.Sprintf("move /y \"%s\" \"%s.old\" >nul\r\n", exePath, exePath) +
			fmt.Sprintf("copy /y \"%s\" \"%s\" >nul\r\n", newPath, exePath) +
			fmt.Sprintf("del \"%s\"\r\n", newPath) +
			fmt.Sprintf("cd /d \"%s\"\r\n", wd) +
			fmt.Sprintf("start \"\" \"%s\" %s\r\n", exePath, strings.Join(os.Args[1:], " ")) +
			fmt.Sprintf("del \"%s.old\"\r\n", exePath) +
			"(goto) 2>nul & del \"%~f0\"\r\n"
		if err := os.WriteFile(scriptPath, []byte(bat), 0o644); err != nil {
			return err
		}
		cmd = exec.Command("cmd", "/c", scriptPath)
	} else {
		scriptPath = newPath + ".upgrade.sh"
		sh := "#!/bin/sh\n" +
			"sleep 2\n" +
			fmt.Sprintf("mv -f \"%s\" \"%s.old\"\n", exePath, exePath) +
			fmt.Sprintf("cp \"%s\" \"%s\"\n", newPath, exePath) +
			fmt.Sprintf("chmod +x \"%s\"\n", exePath) +
			fmt.Sprintf("rm -f \"%s\"\n", newPath) +
			fmt.Sprintf("cd \"%s\"\n", wd) +
			fmt.Sprintf("nohup \"%s\" %s >/dev/null 2>&1 &\n", exePath, strings.Join(os.Args[1:], " ")) +
			fmt.Sprintf("rm -f \"%s.old\"\n", exePath) +
			fmt.Sprintf("rm -f \"%s\"\n", scriptPath)
		if err := os.WriteFile(scriptPath, []byte(sh), 0o755); err != nil {
			return err
		}
		cmd = exec.Command("sh", scriptPath)
	}

	cmd.SysProcAttr = detachSysProcAttr()
	cmd.Dir = wd
	return cmd.Start()
}

