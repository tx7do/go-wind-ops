package serviceid

const (
	AdminService = "admin-service" // 后台管理服务
	FrontService = "front-service" // 前台服务

	CoreService  = "core-service"  // 核心服务
	AgentService = "agent-service" // 边缘代理服务

	DTMService = "dtm-service" // DTM服务
)

var (
	DtmServiceAddress = MakeDiscoveryAddress(DTMService)
)
