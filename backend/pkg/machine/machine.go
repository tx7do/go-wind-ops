// Package machine 提供轻量的机器标识、静态指纹与资源快照采集(gopsutil),
// 供 agent 端组装注册与心跳消息使用。只保留运维平台需要的最小字段集,
// 完整硬件清单采集后续按需再扩展。
package machine

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
	idutil "github.com/tx7do/go-utils/id"

	"go-wind-ops/pkg/signal"
)

// MachineID 返回稳定的机器标识:优先 tx7do UnifyMachineID(机器 GUID/序列号
// 归一化的 32 位 hex),失败时回退主机名。作为 agent 全局唯一 ID。
func MachineID() string {
	if mid, err := idutil.UnifyMachineID(); err == nil && mid != "" {
		return mid
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "unknown-host"
	}
	return hostname
}

// Fingerprint 采集机器静态指纹,用于注册/重连时上报。
func Fingerprint(agentVersion string) signal.AgentInfo {
	info := signal.AgentInfo{
		Hostname:     hostname(),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: agentVersion,
		IPs:          localIPs(),
	}

	if hi, err := host.Info(); err == nil && hi != nil {
		info.Platform = hi.Platform
		info.PlatformVersion = hi.PlatformVersion
		info.KernelVersion = hi.KernelVersion
		info.UptimeSeconds = hi.Uptime
	}
	if cores, err := cpu.Counts(true); err == nil {
		info.CPUCores = cores
	}
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		info.MemTotalBytes = vm.Total
	}
	return info
}

// Snapshot 采集一次资源占用快照,随心跳上报。
// 首次调用 CPU 百分比可能为 0(gopsutil 以两次采样差计算)。
func Snapshot() signal.AgentMetrics {
	m := signal.AgentMetrics{}
	if percent, err := cpu.Percent(0, false); err == nil && len(percent) > 0 {
		m.CPUPercent = percent[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		m.MemoryUsedPercent = vm.UsedPercent
		m.MemTotalBytes = vm.Total
		m.MemUsedBytes = vm.Used
	}
	if du, err := disk.Usage(HostPath()); err == nil && du != nil {
		m.DiskUsedPercent = du.UsedPercent
	}
	if hi, err := host.Info(); err == nil && hi != nil {
		m.UptimeSeconds = hi.Uptime
	}
	return m
}

// HostPath 返回磁盘用量统计的根路径(Windows 为盘符根,Linux 为 /)。
func HostPath() string {
	if runtime.GOOS == "windows" {
		return `C:\`
	}
	return "/"
}

// ProcessStartTime 返回当前进程启动时刻(诊断用;失败返回零值)。
func ProcessStartTime() time.Time {
	if p, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if ms, err := p.CreateTime(); err == nil {
			return time.UnixMilli(ms)
		}
	}
	return time.Time{}
}

var (
	hostnameOnce sync.Once
	hostnameVal  string
)

func hostname() string {
	hostnameOnce.Do(func() {
		name, err := os.Hostname()
		if err != nil || name == "" {
			name = "unknown-host"
		}
		hostnameVal = name
	})
	return hostnameVal
}

// localIPs 返回非环回的 IPv4/IPv6 地址(排除 link-local),供节点管理展示。
func localIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		ip := ipnet.IP.String()
		if _, dup := seen[ip]; dup {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	if len(out) == 0 {
		out = append(out, fmt.Sprintf("127.0.0.1"))
	}
	return out
}
