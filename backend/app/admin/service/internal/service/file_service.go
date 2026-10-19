package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// FileService 远端文件管理门面:透传 core 经 MQTT 代理的 agent 文件操作。
type FileService struct {
	adminV1.FileServiceHTTPServer

	log *log.Helper

	fileServiceClient opsV1.FileServiceClient
}

func NewFileService(ctx *bootstrap.Context, fileServiceClient opsV1.FileServiceClient) *FileService {
	return &FileService{
		log:               ctx.NewLoggerHelper("file/service/admin-service"),
		fileServiceClient: fileServiceClient,
	}
}

func (s *FileService) ListAgentFiles(ctx context.Context, req *opsV1.ListAgentFilesRequest) (*opsV1.ListAgentFilesResponse, error) {
	if req == nil || req.GetAgentId() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId required")
	}
	// dir 允许为空:agent 端以自身工作目录兜底(响应 dir 回带实际路径)
	return s.fileServiceClient.ListAgentFiles(ctx, req)
}

func (s *FileService) ReadAgentFile(ctx context.Context, req *opsV1.ReadAgentFileRequest) (*opsV1.ReadAgentFileResponse, error) {
	if req == nil || req.GetAgentId() == "" || req.GetPath() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/path required")
	}
	return s.fileServiceClient.ReadAgentFile(ctx, req)
}

func (s *FileService) WriteAgentFile(ctx context.Context, req *opsV1.WriteAgentFileRequest) (*opsV1.WriteAgentFileResponse, error) {
	if req == nil || req.GetAgentId() == "" || req.GetPath() == "" || req.GetContentBase64() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/path/contentBase64 required")
	}
	return s.fileServiceClient.WriteAgentFile(ctx, req)
}

func (s *FileService) DeleteAgentFile(ctx context.Context, req *opsV1.DeleteAgentFileRequest) (*opsV1.DeleteAgentFileResponse, error) {
	if req == nil || req.GetAgentId() == "" || req.GetPath() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/path required")
	}
	return s.fileServiceClient.DeleteAgentFile(ctx, req)
}

func (s *FileService) PullAgentFile(ctx context.Context, req *opsV1.PullAgentFileRequest) (*opsV1.PullAgentFileResponse, error) {
	if req == nil || req.GetAgentId() == "" || req.GetPath() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/path required")
	}
	return s.fileServiceClient.PullAgentFile(ctx, req)
}

func (s *FileService) PrepareAgentFilePush(ctx context.Context, req *opsV1.PrepareAgentFilePushRequest) (*opsV1.PrepareAgentFilePushResponse, error) {
	if req == nil || req.GetAgentId() == "" || req.GetPath() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/path required")
	}
	return s.fileServiceClient.PrepareAgentFilePush(ctx, req)
}

func (s *FileService) CompleteAgentFilePush(ctx context.Context, req *opsV1.CompleteAgentFilePushRequest) (*opsV1.CompleteAgentFilePushResponse, error) {
	if req == nil || req.GetAgentId() == "" || req.GetPath() == "" || req.GetObjectKey() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/path/objectKey required")
	}
	return s.fileServiceClient.CompleteAgentFilePush(ctx, req)
}
