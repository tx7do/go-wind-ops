package service

import (
	"context"
	"encoding/base64"
	"time"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/terminalchunk"
	"go-wind-ops/app/core/service/internal/data/ent/terminalsession"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ── 终端会话审计 RPC(只读;写入由录制器驱动)──

// ListTerminalSessions 查询终端会话审计列表。
func (s *TerminalService) ListTerminalSessions(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListTerminalSessionResponse, error) {
	return s.sessionRepo.List(ctx, req)
}

// GetTerminalSession 查询终端会话详情。
func (s *TerminalService) GetTerminalSession(ctx context.Context, req *opsV1.GetTerminalSessionRequest) (*opsV1.TerminalSession, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	row, err := s.sessionRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("terminal session not found")
	}
	return terminalSessionToDTO(row), nil
}

// ListTerminalChunks 拉取会话录制分块(payload 以原始字节返回)。
func (s *TerminalService) ListTerminalChunks(ctx context.Context, req *opsV1.ListTerminalChunksRequest) (*opsV1.ListTerminalChunkResponse, error) {
	if req == nil || req.GetSessionId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: sessionId required")
	}
	rows, err := s.chunkRepo.ListBySession(ctx, req.GetSessionId(), int(req.GetFromSeq()), int(req.GetLimit()))
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("query terminal chunks failed")
	}

	resp := &opsV1.ListTerminalChunkResponse{}
	for _, row := range rows {
		if req.GetLimit() > 0 && len(resp.Items) >= int(req.GetLimit()) {
			truncated := true
			resp.Truncated = &truncated
			break
		}
		raw, derr := base64.StdEncoding.DecodeString(row.Payload)
		if derr != nil {
			continue
		}
		direction := "OUTPUT"
		if row.Direction == terminalchunk.DirectionInput {
			direction = "INPUT"
		}
		item := &opsV1.TerminalChunkItem{
			Seq:       transPtrInt32(int32(row.Seq)),
			Direction: &direction,
			Payload:   raw,
		}
		if row.CreatedAt != nil {
			item.CreatedAt = timestamppb.New(*row.CreatedAt)
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

// terminalSessionToDTO ent → proto。
func terminalSessionToDTO(row *ent.TerminalSession) *opsV1.TerminalSession {
	if row == nil {
		return nil
	}
	status := "OPEN"
	if row.Status == terminalsession.StatusClosed {
		status = "CLOSED"
	}
	return &opsV1.TerminalSession{
		Id:          &row.ID,
		SessionId:   &row.SessionID,
		AgentId:     &row.AgentID,
		NodeId:      uint32Ptr(row.NodeID),
		OperatorId:  uint32Ptr(row.OperatorID),
		Status:      &status,
		InputBytes:  &row.InputBytes,
		OutputBytes: &row.OutputBytes,
		ChunkCount:  transPtrInt32(int32(row.ChunkCount)),
		Truncated:   &row.Truncated,
		CreatedAt:   timestamppbOfPtr(row.CreatedAt),
		ClosedAt:    timestamppbOfPtr(row.ClosedAt),
	}
}

// timestamppbOfPtr *time.Time → *timestamppb.Timestamp(nil 安全)。
func timestamppbOfPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func uint32Ptr(v uint32) *uint32 { return &v }
