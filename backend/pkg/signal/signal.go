// Package signal 定义 core 与 agent 之间的 MQTT 信令消息。
//
// 与 cloud-system 的信令协议同构:消息为纯 Go 结构体,经 kratos broker 的
// json codec 序列化(QoS 1);所有下行指令带 request_id 幂等键,agent 以
// command/result 回传执行结果。业务轮若需要对外 REST/gRPC 暴露时再 proto 化。
package signal

import (
	"encoding/json"
	"time"
)

// ============================================================================
// 鉴权与注册
// ============================================================================

// AgentAuthRequest agent 连接 MQTT 成功后发起的准入请求。
// 准入模型:agent 本地持久化随机 token(首次启动生成);core 校验
// SHA-256(token) 与 agent_node 表落库哈希一致;agent 首次出现时按
// TOFU 信任首连并落库哈希(预留 EMQX webhook/ACL 升级位)。
type AgentAuthRequest struct {
	AgentId string    `json:"agent_id"`
	Token   string    `json:"token"`
	Info    AgentInfo `json:"info"`
}

// AgentInfo agent 机器静态指纹(注册/重连时上报)。
type AgentInfo struct {
	Hostname        string   `json:"hostname"`
	OS              string   `json:"os"`              // linux/windows/darwin
	Arch            string   `json:"arch"`            // amd64/arm64/...
	Platform        string   `json:"platform"`        // ubuntu/windows 10 ...
	PlatformVersion string   `json:"platform_version"`
	KernelVersion   string   `json:"kernel_version"`
	AgentVersion    string   `json:"agent_version"`
	UptimeSeconds   uint64   `json:"uptime_seconds"`
	IPs             []string `json:"ips,omitempty"`
	CPUCores        int      `json:"cpu_cores"`
	MemTotalBytes   uint64   `json:"mem_total_bytes"`
	Labels          []string `json:"labels,omitempty"`
}

// AgentAuthResponse core 对准入请求的应答。
type AgentAuthResponse struct {
	OK         bool      `json:"ok"`
	Reason     string    `json:"reason,omitempty"`
	ServerTime time.Time `json:"server_time"`
}

// ============================================================================
// 心跳与指标
// ============================================================================

// Heartbeat 周期心跳,内含一次指标快照(core 侧更新在线状态与最新指标)。
type Heartbeat struct {
	AgentId   string       `json:"agent_id"`
	Timestamp time.Time    `json:"timestamp"`
	Version   string       `json:"version,omitempty"`
	Metrics   AgentMetrics `json:"metrics"`
}

// AgentMetrics 机器资源占用快照。
type AgentMetrics struct {
	CPUPercent        float64 `json:"cpu_percent"`
	MemoryUsedPercent float64 `json:"memory_used_percent"`
	DiskUsedPercent   float64 `json:"disk_used_percent"`
	MemTotalBytes     uint64  `json:"mem_total_bytes"`
	MemUsedBytes      uint64  `json:"mem_used_bytes"`
	UptimeSeconds     uint64  `json:"uptime_seconds"`
}

// ============================================================================
// 控制指令
// ============================================================================

// CommandAction 指令动作枚举,业务轮按域扩展(取值风格与 cloud-system 的
// CommandAction 一致:小写下划线)。
type CommandAction string

const (
	CommandActionPing CommandAction = "ping" // 连通性/时延探测,闭环验证用
)

// CommandEnvelope core 下发给 agent 的统一指令信封。
// RequestId 为幂等键(UUID),agent 必须在 CommandResult 中原样回传。
type CommandEnvelope struct {
	RequestId  string          `json:"request_id"`
	AgentId    string          `json:"agent_id"`
	Action     CommandAction   `json:"action"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	IssuedAt   time.Time       `json:"issued_at,omitempty"`
	OperatorId uint32          `json:"operator_id,omitempty"` // 触发指令的用户(审计用)
}

// CommandResult agent 对指令的执行结果回执。
type CommandResult struct {
	RequestId  string          `json:"request_id"`
	AgentId    string          `json:"agent_id"`
	Action     CommandAction   `json:"action"`
	Success    bool            `json:"success"`
	Error      string          `json:"error,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	FinishedAt time.Time       `json:"finished_at,omitempty"`
}

// PingPayload / PingResult ping 指令的载荷与回执数据。
type PingPayload struct {
	SentAt time.Time `json:"sent_at"`
}

type PingResult struct {
	SentAt     time.Time `json:"sent_at"`
	ReceivedAt time.Time `json:"received_at"`
	AgentTime  time.Time `json:"agent_time"`
}

// ============================================================================
// 文件传输信令 (内容走 MinIO/S3 预签名 URL,不过 MQTT)
// ============================================================================

// FileRequestDirection 传输方向。
type FileRequestDirection string

const (
	FileUpload   FileRequestDirection = "upload"   // agent -> 对象存储
	FileDownload FileRequestDirection = "download" // 对象存储 -> agent
)

// FileTransferRequest agent 请求一条预签名传输链路。
type FileTransferRequest struct {
	AgentId   string               `json:"agent_id"`
	RequestId string               `json:"request_id"`
	Direction FileRequestDirection `json:"direction"`
	ObjectKey string               `json:"object_key"` // 对象存储键(上传时可由 core 生成后回填)
}

// FileTransferResponse core 返回的预签名链路。
type FileTransferResponse struct {
	RequestId    string            `json:"request_id"`
	OK           bool              `json:"ok"`
	Reason       string            `json:"reason,omitempty"`
	Url          string            `json:"url,omitempty"`  // 预签名 PUT/GET URL
	ObjectKey    string            `json:"object_key,omitempty"`
	ExpireAt     time.Time         `json:"expire_at,omitempty"`
	ExpectedSHA2 string            `json:"expected_sha256,omitempty"` // 下载时校验
	Headers      map[string]string `json:"headers,omitempty"`         // 附加 HTTP 头
}

// FileTransferComplete agent 传输完成回执(对账:大小/校验和)。
type FileTransferComplete struct {
	RequestId string `json:"request_id"`
	ObjectKey string `json:"object_key"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}
