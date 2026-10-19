package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/metadata"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// logQueryTimeout 日志查询同步指令的回执等待时长。
const logQueryTimeout = 10 * time.Second

// ProjectService 受管机器应用项目管理:CRUD 落库,生命周期操作经 MQTT
// 指令下发到节点 agent(agent 侧校验授权目录),结果回执写指令台账并
// 回填项目运行状态。
type ProjectService struct {
	opsV1.UnimplementedProjectServiceServer

	log  *log.Helper
	repo *data.ProjectRepo
	nodeRepo *data.AgentNodeRepo

	agentService *AgentService
}

func NewProjectService(ctx *bootstrap.Context, repo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, agentService *AgentService) *ProjectService {
	return &ProjectService{
		log:          ctx.NewLoggerHelper("project/service/core-service"),
		repo:         repo,
		nodeRepo:     nodeRepo,
		agentService: agentService,
	}
}

func (s *ProjectService) ListProjects(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListProjectResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *ProjectService) GetProject(ctx context.Context, req *opsV1.GetProjectRequest) (*opsV1.Project, error) {
	return s.repo.Get(ctx, req)
}

func (s *ProjectService) CreateProject(ctx context.Context, req *opsV1.CreateProjectRequest) (*opsV1.Project, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetNodeId() == 0 || req.Data.GetName() == "" || req.Data.GetProjectDir() == "" {
		return nil, opsV1.ErrorBadRequest("nodeId/name/projectDir are required")
	}

	// 节点必须已准入
	if _, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: req.Data.NodeId}); err != nil {
		return nil, opsV1.ErrorBadRequest("node not found")
	}

	taken, err := s.repo.IsNameTaken(ctx, req.Data.GetNodeId(), req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("project name already exists on this node")
	}

	return s.repo.Create(ctx, req, s.operatorID(ctx))
}

func (s *ProjectService) UpdateProject(ctx context.Context, req *opsV1.UpdateProjectRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	if req.Data.Name != nil {
		nodeID := req.Data.GetNodeId()
		if nodeID == 0 {
			existing, err := s.repo.Get(ctx, &opsV1.GetProjectRequest{Id: &req.Id})
			if err != nil {
				return nil, err
			}
			nodeID = existing.GetNodeId()
		}
		taken, err := s.repo.IsNameTaken(ctx, nodeID, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("project name already exists on this node")
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ProjectService) DeleteProject(ctx context.Context, req *opsV1.DeleteProjectRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// OperateProject 下发项目生命周期操作。payload 携带完整执行要素(目录/
// 命令),agent 侧做授权目录二次校验后执行;结果按 request_id 回写台账。
func (s *ProjectService) OperateProject(ctx context.Context, req *opsV1.OperateProjectRequest) (*opsV1.OperateProjectResponse, error) {
	if req == nil || req.GetOperation() == opsV1.ProjectOperation_PROJECT_OPERATION_UNSPECIFIED {
		return nil, opsV1.ErrorBadRequest("invalid parameter: operation required")
	}

	project, err := s.repo.Get(ctx, &opsV1.GetProjectRequest{Id: &req.Id})
	if err != nil {
		return nil, err
	}

	nodeID := project.GetNodeId()
	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		return nil, err
	}

	operation := operationName(req.GetOperation())
	payload := &signal.ProjectOperationPayload{
		ProjectId:     project.GetId(),
		Name:          project.GetName(),
		Dir:           project.GetProjectDir(),
		StartCommand:  project.GetStartCommand(),
		StopCommand:   project.GetStopCommand(),
		StatusCommand: project.GetStatusCommand(),
	}

	requestId, err := s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandAction("project_"+operation), payload, s.operatorID(ctx))
	if err != nil {
		s.log.Errorf("operate project failed (project=%d operation=%s): %s", project.GetId(), operation, err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	return &opsV1.OperateProjectResponse{RequestId: requestId}, nil
}

// GetProjectLog 查询项目运行日志尾部:经同步指令等 agent 回执(10s 超时)。
func (s *ProjectService) GetProjectLog(ctx context.Context, req *opsV1.GetProjectLogRequest) (*opsV1.GetProjectLogResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	project, node, err := s.resolveWithNode(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	res, err := s.agentService.PublishCommandSync(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionProjectLog,
		&signal.ProjectLogPayload{
			ProjectId: project.GetId(),
			Dir:       project.GetProjectDir(),
			MaxBytes:  int64(req.GetMaxBytes()),
			Offset:    req.GetOffset(),
		},
		operatorIDOf(ctx), logQueryTimeout)
	if err != nil {
		s.log.Errorf("query project log failed (project=%d): %s", project.GetId(), err.Error())
		return nil, opsV1.ErrorServiceUnavailable("%s", "query project log failed: "+err.Error())
	}
	if !res.Success {
		return nil, opsV1.ErrorInternalServerError("%s", "query project log: "+res.Error)
	}

	var out signal.ProjectLogResult
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode project log result")
	}

	contentB64, derr := base64.StdEncoding.DecodeString(out.ContentB64)
	if derr != nil {
		contentB64 = nil
	}

	return &opsV1.GetProjectLogResponse{
		Content:    out.Content,
		Size:       out.Size,
		Truncated:  out.Truncated,
		ContentB64: contentB64,
		Offset:     out.Offset,
	}, nil
}

// ClearProjectLog 清空项目运行日志(异步指令)。
func (s *ProjectService) ClearProjectLog(ctx context.Context, req *opsV1.ClearProjectLogRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	project, node, err := s.resolveWithNode(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	_, err = s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionProjectLogClear,
		&signal.ProjectLogPayload{ProjectId: project.GetId(), Dir: project.GetProjectDir()},
		operatorIDOf(ctx))
	if err != nil {
		return nil, opsV1.ErrorServiceUnavailable("clear project log failed: mqtt channel unavailable")
	}
	return &emptypb.Empty{}, nil
}

// resolveWithNode 取项目及其节点。
func (s *ProjectService) resolveWithNode(ctx context.Context, projectID uint32) (*opsV1.Project, *opsV1.AgentNode, error) {
	project, err := s.repo.Get(ctx, &opsV1.GetProjectRequest{Id: &projectID})
	if err != nil {
		return nil, nil, err
	}
	nodeID := project.GetNodeId()
	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		return nil, nil, opsV1.ErrorBadRequest("project node not found")
	}
	return project, node, nil
}

func operationName(op opsV1.ProjectOperation) string {
	switch op {
	case opsV1.ProjectOperation_START:
		return "start"
	case opsV1.ProjectOperation_STOP:
		return "stop"
	case opsV1.ProjectOperation_RESTART:
		return "restart"
	case opsV1.ProjectOperation_STATUS:
		return "status"
	default:
		return "unknown"
	}
}

func (s *ProjectService) operatorID(ctx context.Context) uint32 {
	operator, err := metadata.FromServerContext(ctx)
	if err != nil || operator == nil {
		return 0
	}
	return uint32(operator.GetUserId())
}
