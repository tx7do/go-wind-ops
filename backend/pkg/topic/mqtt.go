package topic

import "fmt"

const (
	MqttTopicPrefix = "ops/" // go-wind-ops MQTT 主题前缀
)

// ============================================================================
// MQTT 共享订阅（$share）
//
// core 服务以多副本部署时,普通通配订阅（如 ops/agent/+/heartbeat）是广播语义:
// EMQX 会把每条 agent 上行消息投递给所有副本,造成心跳/信令的重复消费。
// 共享订阅（$share/{group}/{filter}）让同组副本负载均衡,每条消息只投给组内
// 一个实例;单实例部署行为不变。仅用于订阅过滤器——发布必须用原始主题,
// $share 前缀对 publish 无意义且非法。
// ============================================================================

const (
	MqttShareGroupCore = "gwo" // core 服务共享订阅组
)

// SharedSubscription 把主题过滤器包装为共享订阅形式。
func SharedSubscription(group, filter string) string {
	return "$share/" + group + "/" + filter
}

const (
	// 鉴权与注册

	MqttTopicAuth = MqttTopicPrefix + "agent/%s/auth/%s" // auth/{request|response}

	// 心跳与状态

	MqttTopicHeartbeat = MqttTopicPrefix + "agent/%s/heartbeat" // Agent -> Core (心跳,内含指标快照)
	MqttTopicMetrics   = MqttTopicPrefix + "agent/%s/metrics"   // Agent -> Core (单独指标上报)

	// 控制指令

	MqttTopicCommand       = MqttTopicPrefix + "agent/%s/command"        // Core -> Agent (下发指令)
	MqttTopicCommandAck    = MqttTopicPrefix + "agent/%s/command/ack"    // Agent -> Core (指令确认)
	MqttTopicCommandResult = MqttTopicPrefix + "agent/%s/command/result" // Agent -> Core (执行结果)

	// Web 终端双向流 (会话开关走 command 指令主题,字节流走这里;
	// 通配订阅 + sessionId 路由,不经 $share——终端流是连接级点对点)

	MqttTopicTerminalInput  = MqttTopicPrefix + "agent/%s/terminal/%s/input"  // Core -> Agent (键盘输入)
	MqttTopicTerminalOutput = MqttTopicPrefix + "agent/%s/terminal/%s/output" // Agent -> Core (回显输出)
	MqttTopicTerminalResize = MqttTopicPrefix + "agent/%s/terminal/%s/resize" // Core -> Agent (PTY 尺寸)

	// 文件传输信令 (文件内容走 MinIO/S3 预签名 URL,不过 MQTT)

	MqttTopicFileRequest  = MqttTopicPrefix + "agent/%s/file/request"  // Agent -> Core (请求上传/下载链接)
	MqttTopicFileResponse = MqttTopicPrefix + "agent/%s/file/response" // Core -> Agent (返回预签名 URL)
	MqttTopicFileComplete = MqttTopicPrefix + "agent/%s/file/complete" // Agent -> Core (传输完成通知)
)

func MakeMqttTopicAuth(agentId string, request bool) string {
	direction := "response"
	if request {
		direction = "request"
	}
	return fmt.Sprintf(MqttTopicAuth, agentId, direction)
}

func MakeMqttTopicHeartbeat(agentId string) string {
	return fmt.Sprintf(MqttTopicHeartbeat, agentId)
}

func MakeMqttTopicMetrics(agentId string) string {
	return fmt.Sprintf(MqttTopicMetrics, agentId)
}

func MakeMqttTopicCommand(agentId string) string {
	return fmt.Sprintf(MqttTopicCommand, agentId)
}

func MakeMqttTopicCommandAck(agentId string) string {
	return fmt.Sprintf(MqttTopicCommandAck, agentId)
}

func MakeMqttTopicCommandResult(agentId string) string {
	return fmt.Sprintf(MqttTopicCommandResult, agentId)
}

func MakeMqttTopicFileRequest(agentId string) string {
	return fmt.Sprintf(MqttTopicFileRequest, agentId)
}

func MakeMqttTopicFileResponse(agentId string) string {
	return fmt.Sprintf(MqttTopicFileResponse, agentId)
}

func MakeMqttTopicFileComplete(agentId string) string {
	return fmt.Sprintf(MqttTopicFileComplete, agentId)
}

// MakeMqttTopicTerminalInput 终端输入主题;通配订阅传 "+"。
func MakeMqttTopicTerminalInput(agentId, sessionId string) string {
	return fmt.Sprintf(MqttTopicTerminalInput, agentId, sessionId)
}

// MakeMqttTopicTerminalOutput 终端输出主题;通配订阅传 "+"。
func MakeMqttTopicTerminalOutput(agentId, sessionId string) string {
	return fmt.Sprintf(MqttTopicTerminalOutput, agentId, sessionId)
}

// MakeMqttTopicTerminalResize 终端尺寸主题;通配订阅传 "+"。
func MakeMqttTopicTerminalResize(agentId, sessionId string) string {
	return fmt.Sprintf(MqttTopicTerminalResize, agentId, sessionId)
}
