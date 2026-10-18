// file_service.go 远端文件管理:core 经 MQTT 同步指令代理 agent 侧的
// 文件浏览/读写/删除。路径合法性由 agent 的授权目录白名单兜底校验。
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// fileSyncTimeout 单条文件指令的回执等待上限。
const fileSyncTimeout = 15 * time.Second

type FileService struct {
	opsV1.UnimplementedFileServiceServer

	log      *log.Helper
	agentSvc *AgentService
	nodeRepo *data.AgentNodeRepo
}

func NewFileService(ctx *bootstrap.Context, agentSvc *AgentService, nodeRepo *data.AgentNodeRepo) *FileService {
	return &FileService{
		log:      ctx.NewLoggerHelper("file/service/core-service"),
		agentSvc: agentSvc,
		nodeRepo: nodeRepo,
	}
}

// callSync 下发文件指令并等待回执;agent 侧失败时转为 gRPC 错误。
func (s *FileService) callSync(ctx context.Context, agentId string, action signal.CommandAction, payload any) (*signal.CommandResult, error) {
	res, err := s.agentSvc.PublishCommandSync(ctx, 0, agentId, action, payload, operatorIDOf(ctx), fileSyncTimeout)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("file command failed: %s", err.Error())
	}
	if res == nil || !res.Success {
		msg := "unknown error"
		if res != nil && res.Error != "" {
			msg = res.Error
		}
		return nil, opsV1.ErrorInternalServerError("agent rejected %s: %s", action, msg)
	}
	return res, nil
}

// resolveAgentID 兼容前端传节点 ID 或 agent 标识;返回可用于 MQTT 主题的 agentId。
func (s *FileService) resolveAgentID(ctx context.Context, agentId string) (string, error) {
	if agentId == "" {
		return "", opsV1.ErrorBadRequest("agentId required")
	}
	if s.nodeRepo != nil {
		if _, err := s.nodeRepo.GetByAgentID(ctx, agentId); err != nil {
			return "", opsV1.ErrorNotFound("agent not enrolled: %s", agentId)
		}
	}
	return agentId, nil
}

func (s *FileService) ListAgentFiles(ctx context.Context, req *opsV1.ListAgentFilesRequest) (*opsV1.ListAgentFilesResponse, error) {
	if req == nil || req.GetDir() == "" {
		return nil, opsV1.ErrorBadRequest("dir required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileList, map[string]string{"dir": req.GetDir()})
	if err != nil {
		return nil, err
	}

	var out struct {
		Dir     string `json:"dir"`
		Entries []struct {
			Name  string `json:"name"`
			Size  int64  `json:"size"`
			IsDir bool   `json:"is_dir"`
		} `json:"entries"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode file list: %s", err.Error())
	}

	resp := &opsV1.ListAgentFilesResponse{Dir: &out.Dir}
	for _, e := range out.Entries {
		name, size, isDir := e.Name, e.Size, e.IsDir
		resp.Entries = append(resp.Entries, &opsV1.AgentFileEntry{
			Name:  &name,
			Size:  &size,
			IsDir: &isDir,
		})
	}
	return resp, nil
}

func (s *FileService) ReadAgentFile(ctx context.Context, req *opsV1.ReadAgentFileRequest) (*opsV1.ReadAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" {
		return nil, opsV1.ErrorBadRequest("path required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileRead, map[string]string{"path": req.GetPath()})
	if err != nil {
		return nil, err
	}

	var out struct {
		Path          string `json:"path"`
		Size          int64  `json:"size"`
		Truncated     bool   `json:"truncated"`
		ContentBase64 string `json:"content_base64"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode file content: %s", err.Error())
	}

	return &opsV1.ReadAgentFileResponse{
		Path:          &out.Path,
		Size:          &out.Size,
		Truncated:     &out.Truncated,
		ContentBase64: &out.ContentBase64,
	}, nil
}

func (s *FileService) WriteAgentFile(ctx context.Context, req *opsV1.WriteAgentFileRequest) (*opsV1.WriteAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" || req.GetContentBase64() == "" {
		return nil, opsV1.ErrorBadRequest("path/contentBase64 required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileWrite, map[string]string{
		"path":    req.GetPath(),
		"content": req.GetContentBase64(),
	})
	if err != nil {
		return nil, err
	}

	var out struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode write result: %s", err.Error())
	}

	return &opsV1.WriteAgentFileResponse{Path: &out.Path, Size: &out.Size}, nil
}

func (s *FileService) DeleteAgentFile(ctx context.Context, req *opsV1.DeleteAgentFileRequest) (*opsV1.DeleteAgentFileResponse, error) {
	if req == nil || req.GetPath() == "" {
		return nil, opsV1.ErrorBadRequest("path required")
	}
	agentId, err := s.resolveAgentID(ctx, req.GetAgentId())
	if err != nil {
		return nil, err
	}

	res, err := s.callSync(ctx, agentId, signal.CommandActionFileDelete, map[string]string{"path": req.GetPath()})
	if err != nil {
		return nil, err
	}

	var out struct {
		Deleted string `json:"deleted"`
	}
	if err = json.Unmarshal(res.Data, &out); err != nil {
		return nil, opsV1.ErrorInternalServerError("decode delete result: %s", err.Error())
	}

	return &opsV1.DeleteAgentFileResponse{Deleted: &out.Deleted}, nil
}
