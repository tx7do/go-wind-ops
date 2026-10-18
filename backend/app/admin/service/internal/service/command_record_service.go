package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CommandRecordService agent 指令台账查询门面:透传 core 的只读查询。
type CommandRecordService struct {
	adminV1.CommandRecordServiceHTTPServer

	log *log.Helper

	commandRecordServiceClient opsV1.CommandRecordServiceClient
}

func NewCommandRecordService(ctx *bootstrap.Context, commandRecordServiceClient opsV1.CommandRecordServiceClient) *CommandRecordService {
	return &CommandRecordService{
		log:                        ctx.NewLoggerHelper("command-record/service/admin-service"),
		commandRecordServiceClient: commandRecordServiceClient,
	}
}

func (s *CommandRecordService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCommandRecordResponse, error) {
	return s.commandRecordServiceClient.ListCommandRecords(ctx, req)
}

func (s *CommandRecordService) Get(ctx context.Context, req *opsV1.GetCommandRecordRequest) (*opsV1.CommandRecord, error) {
	return s.commandRecordServiceClient.GetCommandRecord(ctx, req)
}
