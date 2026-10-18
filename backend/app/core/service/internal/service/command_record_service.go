package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CommandRecordService agent 指令台账查询服务(只读;写入走 AgentService
// 的下发与回执链路)。
type CommandRecordService struct {
	opsV1.UnimplementedCommandRecordServiceServer

	log  *log.Helper
	repo *data.CommandRecordRepo
}

func NewCommandRecordService(ctx *bootstrap.Context, repo *data.CommandRecordRepo) *CommandRecordService {
	return &CommandRecordService{
		log:  ctx.NewLoggerHelper("command-record/service/core-service"),
		repo: repo,
	}
}

func (s *CommandRecordService) ListCommandRecords(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCommandRecordResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *CommandRecordService) GetCommandRecord(ctx context.Context, req *opsV1.GetCommandRecordRequest) (*opsV1.CommandRecord, error) {
	return s.repo.Get(ctx, req)
}
