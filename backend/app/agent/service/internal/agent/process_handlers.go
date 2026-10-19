// process_handlers.go 实现进程守护指令(process_list):进程存在性监控的
// agent 侧探测端。只读进程清单,不涉及文件系统,无需授权目录校验。
package agent

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"go-wind-ops/pkg/signal"
)

// registerProcessHandlers 注册进程守护指令处理器。
func (s *Service) registerProcessHandlers() {
	s.RegisterHandler(signal.CommandActionProcessList, s.handleProcessList)
}

// handleProcessList 列出(可按名过滤)本机进程:Windows 走 tasklist CSV,
// Unix 走 ps。进程名过滤在 agent 侧完成以压缩回执体积。
func (s *Service) handleProcessList(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	var p signal.ProcessListPayload
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
	}
	limit := p.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	procs, err := listProcesses()
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(p.Name))
	out := make([]signal.ProcessInfo, 0, len(procs))
	for _, proc := range procs {
		if needle != "" && !strings.Contains(strings.ToLower(proc.Name), needle) {
			continue
		}
		out = append(out, proc)
		if len(out) >= limit {
			break
		}
	}
	return signal.ProcessListResult{Total: len(out), Processes: out}, nil
}

type procRow struct {
	name  string
	pid   uint32
	memMB float64
}

// listProcesses 采集全量进程快照(不过滤,过滤在上层做)。
func listProcesses() ([]signal.ProcessInfo, error) {
	rows, err := readProcRows()
	if err != nil {
		return nil, err
	}
	out := make([]signal.ProcessInfo, 0, len(rows))
	seen := make(map[uint32]bool, len(rows))
	for _, r := range rows {
		if r.pid == 0 || seen[r.pid] {
			continue
		}
		seen[r.pid] = true
		out = append(out, signal.ProcessInfo{Name: r.name, PID: r.pid, MemMB: r.memMB})
	}
	return out, nil
}

func readProcRows() ([]procRow, error) {
	if runtime.GOOS == "windows" {
		return readProcRowsWindows()
	}
	return readProcRowsUnix()
}

// readProcRowsWindows tasklist /fo csv /nh:"name","pid",...,"memKb"。
func readProcRowsWindows() ([]procRow, error) {
	out, err := exec.Command("tasklist", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil, fmt.Errorf("tasklist: %w", err)
	}
	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse tasklist csv: %w", err)
	}
	rows := make([]procRow, 0, len(records))
	for _, rec := range records {
		if len(rec) < 5 {
			continue
		}
		pid, _ := strconv.ParseUint(strings.TrimSpace(rec[1]), 10, 32)
		memKB, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSuffix(strings.TrimSpace(rec[4]), " K"), ",", ""), 64)
		rows = append(rows, procRow{name: strings.TrimSpace(rec[0]), pid: uint32(pid), memMB: memKB / 1024})
	}
	return rows, nil
}

// readProcRowsUnix ps -eo pid,pmem,comm --no-headers:comm 可能含路径,取末端。
func readProcRowsUnix() ([]procRow, error) {
	out, err := exec.Command("ps", "-eo", "pid,pmem,comm", "--no-headers").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	rows := make([]procRow, 0, 256)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, _ := strconv.ParseUint(fields[0], 10, 32)
		pmem, _ := strconv.ParseFloat(fields[1], 64)
		comm := fields[2]
		if i := strings.LastIndexByte(comm, '/'); i >= 0 {
			comm = comm[i+1:]
		}
		rows = append(rows, procRow{name: comm, pid: uint32(pid), memMB: pmem})
	}
	return rows, nil
}
