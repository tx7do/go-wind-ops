package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ScriptService 运维脚本管理门面:透传 core 的 ScriptService。
type ScriptService struct {
	adminV1.ScriptServiceHTTPServer

	log *log.Helper

	scriptServiceClient opsV1.ScriptServiceClient
}

func NewScriptService(ctx *bootstrap.Context, scriptServiceClient opsV1.ScriptServiceClient) *ScriptService {
	return &ScriptService{
		log:                 ctx.NewLoggerHelper("script/service/admin-service"),
		scriptServiceClient: scriptServiceClient,
	}
}

func (s *ScriptService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptResponse, error) {
	return s.scriptServiceClient.ListScripts(ctx, req)
}

func (s *ScriptService) Get(ctx context.Context, req *opsV1.GetScriptRequest) (*opsV1.Script, error) {
	return s.scriptServiceClient.GetScript(ctx, req)
}

func (s *ScriptService) Create(ctx context.Context, req *opsV1.CreateScriptRequest) (*opsV1.Script, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.scriptServiceClient.CreateScript(ctx, req)
}

func (s *ScriptService) Update(ctx context.Context, req *opsV1.UpdateScriptRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}

	req.Data.Id = req.Id

	return s.scriptServiceClient.UpdateScript(ctx, req)
}

func (s *ScriptService) Delete(ctx context.Context, req *opsV1.DeleteScriptRequest) (*emptypb.Empty, error) {
	_, err := s.scriptServiceClient.DeleteScript(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ScriptService) Execute(ctx context.Context, req *opsV1.ExecuteScriptRequest) (*opsV1.ExecuteScriptResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.scriptServiceClient.ExecuteScript(ctx, req)
}
