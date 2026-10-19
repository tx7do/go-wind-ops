// plugin_loader.go 插件化指令:agent 启动时扫描 plugins/ 目录
// (AGENT_PLUGINS_DIR 可配,缺省 ./plugins),把脚本文件注册为
// plugin_<文件名主干> 的自定义 action;另注册 plugin_list 查询清单。
// 按扩展名选解释器:.bat/.cmd→cmd、.ps1→powershell、.sh→bash、
// .py→python、.exe→直连 argv;其他扩展名忽略。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go-wind-ops/pkg/shellexec"
	"go-wind-ops/pkg/signal"
)

// pluginStemPattern 插件文件名主干约束(小写字母开头,字母/数字/下划线)。
var pluginStemPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

const (
	pluginExecTimeoutDefault = 60 * time.Second
	pluginExecTimeoutMax     = 600 * time.Second
)

// pluginDef 一个已注册的插件脚本。
type pluginDef struct {
	action string
	path   string
	kind   string
	size   int64
	mod    time.Time
}

// pluginKindByExt 按扩展名定执行方式;返回空串表示不支持。
func pluginKindByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".bat", ".cmd":
		return "bat"
	case ".ps1":
		return "ps1"
	case ".sh":
		return "sh"
	case ".py":
		return "python"
	case ".exe":
		return "binary"
	default:
		return ""
	}
}

// registerPluginHandlers 启动时扫描插件目录并注册处理器。
func (s *Service) registerPluginHandlers() {
	dir := os.Getenv("AGENT_PLUGINS_DIR")
	if dir == "" {
		dir = "./plugins"
	}
	plugins := s.scanPluginDir(dir)

	// 固定清单指令
	s.RegisterHandler(signal.CommandActionPluginList, s.handlePluginList)

	for i := range plugins {
		p := plugins[i]
		s.RegisterHandler(signal.CommandAction(p.action), s.makePluginHandler(p))
	}
	s.plugins = plugins
	s.log().Infof("plugin handlers registered (dir=%s count=%d)", dir, len(plugins))
}

// scanPluginDir 扫描目录,取合法脚本文件,按 action 名排序。
func (s *Service) scanPluginDir(dir string) []pluginDef {
	entries, err := os.ReadDir(dir)
	if err != nil {
		s.log().Infof("plugin dir not available (dir=%s): %s", dir, err.Error())
		return nil
	}
	var out []pluginDef
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if !pluginStemPattern.MatchString(stem) {
			continue
		}
		kind := pluginKindByExt(filepath.Ext(e.Name()))
		if kind == "" {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		abs, err := filepath.Abs(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, pluginDef{
			action: string(signal.CommandActionPluginPrefix) + stem,
			path:   abs,
			kind:   kind,
			size:   info.Size(),
			mod:    info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].action < out[j].action })
	return out
}

// makePluginHandler 生成单个插件的执行处理器。
func (s *Service) makePluginHandler(p pluginDef) CommandHandler {
	return func(_ context.Context, env *signal.CommandEnvelope) (any, error) {
		payload := &signal.PluginExecPayload{}
		if len(env.Payload) > 0 {
			if err := json.Unmarshal(env.Payload, payload); err != nil {
				return nil, fmt.Errorf("decode payload: %w", err)
			}
		}
		timeout := time.Duration(payload.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = pluginExecTimeoutDefault
		}
		if timeout > pluginExecTimeoutMax {
			timeout = pluginExecTimeoutMax
		}

		start := time.Now()
		exitCode, output, timedOut, err := runPlugin(s, p.kind, p.path, payload.Args, timeout)
		duration := time.Since(start)

		out := strings.TrimSpace(output)
		if len(out) > signal.PluginOutputMax {
			out = out[:signal.PluginOutputMax] + "\n...(truncated)"
		}
		if timedOut {
			return nil, fmt.Errorf("plugin %s timeout after %s", p.action, timeout)
		}
		if err != nil {
			// 非零退出码不是传输错误:带退出码与输出回执,由调用方判定
			if exitCode != 0 {
				return &signal.PluginExecResult{
					Action:   p.action,
					ExitCode: exitCode,
					Output:   out,
					Duration: duration.String(),
				}, nil
			}
			return nil, fmt.Errorf("plugin %s: %s", p.action, err.Error())
		}
		return &signal.PluginExecResult{
			Action:   p.action,
			ExitCode: exitCode,
			Output:   out,
			Duration: duration.String(),
		}, nil
	}
}

// runPlugin 按类型选解释器执行(RunArgv 直连 argv,超时杀树)。
func runPlugin(s *Service, kind, path string, args []string, timeout time.Duration) (int, string, bool, error) {
	var name string
	var argv []string
	switch kind {
	case "bat":
		// cmd.exe 按自有规则解析参数(\" 视为字面反斜杠+引号结束,另有 % 变量
		// 展开与 &|<>^ 管道元字符),无安全转义可用——含元字符的参数直接拒绝。
		for _, a := range args {
			if strings.ContainsAny(a, "\"%&|<>^!") {
				return 0, "", false, fmt.Errorf("plugin arg contains cmd metacharacter, rejected: %q", a)
			}
		}
		name, argv = "cmd", append([]string{"/c", path}, args...)
	case "ps1":
		name, argv = "powershell", append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path}, args...)
	case "sh":
		name, argv = "bash", append([]string{path}, args...)
	case "python":
		name, argv = "python", append([]string{path}, args...)
	default:
		name, argv = path, args
	}
	return shellexec.RunArgv(s.ctx.Context(), "", timeout, name, argv...)
}

// handlePluginList 返回插件清单。
func (s *Service) handlePluginList(_ context.Context, _ *signal.CommandEnvelope) (any, error) {
	dir := os.Getenv("AGENT_PLUGINS_DIR")
	if dir == "" {
		dir = "./plugins"
	}
	list := make([]signal.PluginInfo, 0, len(s.plugins))
	for _, p := range s.plugins {
		list = append(list, signal.PluginInfo{
			Action:   p.action,
			File:     p.path,
			Kind:     p.kind,
			Size:     p.size,
			Modified: p.mod.Format(time.RFC3339),
		})
	}
	return &signal.PluginListResult{Dir: dir, Plugins: list}, nil
}
