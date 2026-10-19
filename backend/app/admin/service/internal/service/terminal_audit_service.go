package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// TerminalAuditService 终端会话审计门面:透传 core 的 TerminalService 审计 RPC。
type TerminalAuditService struct {
	adminV1.TerminalAuditServiceHTTPServer

	log *log.Helper

	terminalServiceClient opsV1.TerminalServiceClient
}

func NewTerminalAuditService(ctx *bootstrap.Context, terminalServiceClient opsV1.TerminalServiceClient) *TerminalAuditService {
	return &TerminalAuditService{
		log:                   ctx.NewLoggerHelper("terminal-audit/service/admin-service"),
		terminalServiceClient: terminalServiceClient,
	}
}

func (s *TerminalAuditService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListTerminalSessionResponse, error) {
	return s.terminalServiceClient.ListTerminalSessions(ctx, req)
}

func (s *TerminalAuditService) Get(ctx context.Context, req *opsV1.GetTerminalSessionRequest) (*opsV1.TerminalSession, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.terminalServiceClient.GetTerminalSession(ctx, req)
}

func (s *TerminalAuditService) ListChunks(ctx context.Context, req *opsV1.ListTerminalChunksRequest) (*opsV1.ListTerminalChunkResponse, error) {
	if req == nil || req.GetSessionId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: sessionId required")
	}
	return s.terminalServiceClient.ListTerminalChunks(ctx, req)
}
