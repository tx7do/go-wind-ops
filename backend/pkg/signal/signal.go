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

	// 项目生命周期操作:载荷为 ProjectOperationPayload,结果为 ProjectOperationResult。
	CommandActionProjectStart   CommandAction = "project_start"
	CommandActionProjectStop    CommandAction = "project_stop"
	CommandActionProjectRestart CommandAction = "project_restart"
	CommandActionProjectStatus  CommandAction = "project_status"

	// 脚本远程执行:载荷为 ScriptExecPayload,结果为 ScriptExecResult。
	CommandActionScriptExec CommandAction = "script_exec"

	// 构建产物分发:载荷为 DistributionPayload,结果为 DistributionResult。
	// 产物内容经 MinIO 预签名 URL 直下,不过 MQTT。
	CommandActionDistApply CommandAction = "dist_apply"

	// 项目运行日志:查询为同步指令(core 等待回执),清空为异步。
	CommandActionProjectLog      CommandAction = "project_log"
	CommandActionProjectLogClear CommandAction = "project_log_clear"
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
// 项目生命周期操作
// ============================================================================

// ProjectOperationPayload 项目操作的载荷:core 从项目记录组装,agent 在
// Dir 内执行对应命令。Dir 必须落在 agent 授权目录内,agent 执行前二次校验。
type ProjectOperationPayload struct {
	ProjectId     uint32 `json:"project_id"`
	Name          string `json:"name"`
	Dir           string `json:"dir"`
	StartCommand  string `json:"start_command,omitempty"`
	StopCommand   string `json:"stop_command,omitempty"`
	StatusCommand string `json:"status_command,omitempty"`
}

// ProjectOperationResult 项目操作的回执数据。Status 为 agent 判定的
// RUNNING/STOPPED,core 直接回填项目表;Output 截断到 64KB。
type ProjectOperationResult struct {
	ProjectId uint32 `json:"project_id"`
	Operation string `json:"operation"`
	Pid       uint64 `json:"pid,omitempty"`
	Alive     bool   `json:"alive,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Status    string `json:"status,omitempty"`
	Output    string `json:"output,omitempty"`
}

// ============================================================================
// 脚本远程执行
// ============================================================================

const (
	// ScriptContentLimit 脚本内容上限(经 MQTT 指令载荷下发,过大应走 MinIO)。
	ScriptContentLimit = 256 << 10
	// ScriptExecTimeoutMax 执行超时上限(秒)。
	ScriptExecTimeoutMax = 600
	// ScriptExecTimeoutDefault 执行超时默认(秒)。
	ScriptExecTimeoutDefault = 60
)

// ScriptExecPayload 脚本执行载荷:core 从脚本记录组装。
type ScriptExecPayload struct {
	ScriptId   uint32 `json:"script_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	TimeoutSec uint32 `json:"timeout_sec,omitempty"`
}

// ScriptExecResult 脚本执行回执。
type ScriptExecResult struct {
	ScriptId uint32 `json:"script_id"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	Duration string `json:"duration,omitempty"`
}

// ============================================================================
// 构建产物分发
// ============================================================================

// AfterOpt 分发后动作取值。
const (
	AfterNone    = "NONE"
	AfterRestart = "RESTART"
	AfterStart   = "START"
)

// DistributionPayload 分发载荷:core 组装(产物走预签名 URL,agent 直接下载)。
type DistributionPayload struct {
	DistributionId uint32 `json:"distribution_id"`
	ProjectId      uint32 `json:"project_id"`
	ProjectName    string `json:"project_name"`
	ProjectDir     string `json:"project_dir"`
	ClearOld       bool   `json:"clear_old,omitempty"`
	DownloadUrl    string `json:"download_url"`
	FileName       string `json:"file_name"`
	AfterOpt       string `json:"after_opt,omitempty"`
	// 项目操作命令(AfterOpt=RESTART/START 时由 agent 联动执行)
	StartCommand  string `json:"start_command,omitempty"`
	StopCommand   string `json:"stop_command,omitempty"`
	StatusCommand string `json:"status_command,omitempty"`
}

// DistributionResult 分发回执。
type DistributionResult struct {
	DistributionId uint32 `json:"distribution_id"`
	DeployedFiles  int    `json:"deployed_files"`
	Output         string `json:"output,omitempty"`
}

// ============================================================================
// 项目运行日志
// ============================================================================

// ProjectLogPayload 日志查询载荷(core 从项目记录组装)。
type ProjectLogPayload struct {
	ProjectId uint32 `json:"project_id"`
	Dir       string `json:"dir"`
	MaxBytes  int64  `json:"max_bytes,omitempty"` // 读取尾部字节数,默认 64KB,上限 256KB
}

// ProjectLogResult 日志查询回执。
type ProjectLogResult struct {
	ProjectId uint32 `json:"project_id"`
	Size      int64  `json:"size"`      // 日志文件总大小
	Truncated bool   `json:"truncated"` // 是否只返回了尾部
	Content   string `json:"content"`
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

// 文件管理操作
const (
	CommandActionDockerDeploy CommandAction = "docker_deploy"

	CommandActionTerminalOpen  CommandAction = "terminal_open"  // 打开终端会话(同步等待回执)
	CommandActionTerminalClose CommandAction = "terminal_close" // 关闭终端会话

	CommandActionPluginList CommandAction = "plugin_list" // 插件清单(同步回执)

	// CommandActionPluginPrefix 插件指令前缀:agent 启动时把 plugins/ 目录的
	// 脚本文件注册为 plugin_<文件名主干> 的自定义 action。
	CommandActionPluginPrefix CommandAction = "plugin_"

	CommandActionFileList   CommandAction = "file_list"
	CommandActionFileRead   CommandAction = "file_read"
	CommandActionFileWrite  CommandAction = "file_write"
	CommandActionFileDelete CommandAction = "file_delete"
)

// ============================================================================
// Docker 镜像部署
// ============================================================================

// DockerDeployPayload docker_deploy 指令载荷:镜像 tag + 容器参数。
// Image 支持 {buildNo} 占位符,由 core 侧替换为实际构建号。
type DockerDeployPayload struct {
	DistributionId uint32   `json:"distribution_id"`
	ProjectId      uint32   `json:"project_id,omitempty"`
	Image          string   `json:"image"`
	ContainerName  string   `json:"container_name"`
	Ports          []string `json:"ports,omitempty"`   // host:container
	Envs           []string `json:"envs,omitempty"`    // K=V
	Volumes        []string `json:"volumes,omitempty"` // host:container
	Restart        string   `json:"restart,omitempty"` // 缺省 unless-stopped
	Network        string   `json:"network,omitempty"`
}

// DockerDeployResult docker_deploy 回执。
type DockerDeployResult struct {
	Image         string `json:"image"`
	ContainerName string `json:"container_name,omitempty"`
	ContainerId   string `json:"container_id,omitempty"`
	Output        string `json:"output,omitempty"`
}

// ============================================================================
// Web 终端
// ============================================================================

// TerminalSessionMax 单 agent 并发终端会话上限。
const TerminalSessionMax = 5

// TerminalOpenPayload terminal_open 指令载荷;shell 由 agent 按平台默认
// (Windows=cmd.exe,Unix=$SHELL 或 /bin/bash)。管道伪终端:不支持交互式
// 全屏程序(vim/htop)与 Ctrl+C 控制台事件。
type TerminalOpenPayload struct {
	SessionId string `json:"session_id"`
	Cols      uint32 `json:"cols,omitempty"`
	Rows      uint32 `json:"rows,omitempty"`
}

// TerminalClosePayload terminal_close 指令载荷。
type TerminalClosePayload struct {
	SessionId string `json:"session_id"`
}

// TerminalIO 终端输入/输出字节(base64 承载,过 MQTT JSON codec);
// Closed=true 表示会话已结束(空帧)。
type TerminalIO struct {
	SessionId string `json:"session_id"`
	Data      string `json:"data"`  // base64
	Closed    bool   `json:"closed,omitempty"`
}

// ============================================================================
// 插件化指令(agent plugins/ 目录脚本)
// ============================================================================

// PluginActionNameMax 插件 action 主干长度上限(不含 plugin_ 前缀)。
const PluginActionNameMax = 40

// PluginOutputMax 插件输出回执截断上限。
const PluginOutputMax = 64 << 10

// PluginInfo 插件脚本注册信息。
type PluginInfo struct {
	Action   string `json:"action"`   // 完整 action(plugin_db_backup)
	File     string `json:"file"`     // 脚本绝对路径
	Kind     string `json:"kind"`     // bat|ps1|sh|python|binary
	Size     int64  `json:"size"`     // 字节
	Modified string `json:"modified"` // RFC3339
}

// PluginListResult plugin_list 回执。
type PluginListResult struct {
	Dir     string       `json:"dir"`
	Plugins []PluginInfo `json:"plugins"`
}

// PluginExecPayload 插件执行载荷。
type PluginExecPayload struct {
	Args       []string `json:"args,omitempty"`
	TimeoutSec uint32   `json:"timeout_sec,omitempty"` // 缺省 60,上限 600
}

// PluginExecResult 插件执行回执。
type PluginExecResult struct {
	Action   string `json:"action"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	Duration string `json:"duration,omitempty"`
}
