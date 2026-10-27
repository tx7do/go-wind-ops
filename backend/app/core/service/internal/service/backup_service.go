package service

import (
	"strings"
	"path"
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/go-utils/trans"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const (
	// backupPresignExpiry 备份上传预签名有效期(大包上传留足时间)。
	backupPresignExpiry = time.Hour
	// backupDownloadExpiry 备份下载预签名有效期。
	backupDownloadExpiry = 30 * time.Minute
	// backupBucket 备份对象桶。
	backupBucket = oss.BucketBackups
)

// BackupService 备份任务管理:CRUD + 立即触发 + 记录查询/下载。触发流程:
// 任务 → 节点 → 预签名 PUT → 下发 backup_run 指令(agent 打包 zip 直传
// MinIO)→ 创建 PENDING 记录,回执驱动终态(见 AgentService.applyBackupResult)。
type BackupService struct {
	opsV1.UnimplementedBackupServiceServer

	log        *log.Helper
	taskRepo   *data.BackupTaskRepo
	recordRepo *data.BackupRecordRepo
	nodeRepo   *data.AgentNodeRepo
	ossClient  *oss.MinIOClient

	agentService *AgentService
}

func NewBackupService(ctx *bootstrap.Context, taskRepo *data.BackupTaskRepo, recordRepo *data.BackupRecordRepo, nodeRepo *data.AgentNodeRepo, ossClient *oss.MinIOClient, agentService *AgentService) *BackupService {
	return &BackupService{
		log:          ctx.NewLoggerHelper("backup/service/core-service"),
		taskRepo:     taskRepo,
		recordRepo:   recordRepo,
		nodeRepo:     nodeRepo,
		ossClient:    ossClient,
		agentService: agentService,
	}
}

func (s *BackupService) ListBackupTasks(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBackupTaskResponse, error) {
	return s.taskRepo.List(ctx, req)
}

func (s *BackupService) GetBackupTask(ctx context.Context, req *opsV1.GetBackupTaskRequest) (*opsV1.BackupTask, error) {
	return s.taskRepo.Get(ctx, req)
}

func (s *BackupService) CreateBackupTask(ctx context.Context, req *opsV1.CreateBackupTaskRequest) (*opsV1.BackupTask, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetNodeId() == 0 || req.Data.GetSourceDir() == "" {
		return nil, opsV1.ErrorBadRequest("name/nodeId/sourceDir are required")
	}
	if req.Data.GetCron() != "" {
		if _, err := cron.ParseStandard(req.Data.GetCron()); err != nil {
			return nil, opsV1.ErrorBadRequest("invalid cron expression")
		}
	}
	if req.Data.GetRetention() < 0 {
		return nil, opsV1.ErrorBadRequest("retention must be >= 0")
	}

	taken, err := s.taskRepo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("backup task name already exists")
	}

	return s.taskRepo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *BackupService) UpdateBackupTask(ctx context.Context, req *opsV1.UpdateBackupTaskRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetCron() != "" {
		if _, err := cron.ParseStandard(req.Data.GetCron()); err != nil {
			return nil, opsV1.ErrorBadRequest("invalid cron expression")
		}
	}
	if req.Data.Name != nil {
		taken, err := s.taskRepo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("backup task name already exists")
		}
	}

	if err := s.taskRepo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *BackupService) DeleteBackupTask(ctx context.Context, req *opsV1.DeleteBackupTaskRequest) (*emptypb.Empty, error) {
	if err := s.taskRepo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// TriggerBackup 立即触发一次备份(异步):预签名 PUT → 下发指令 →
// 创建 PENDING 记录;终态由 agent 回执驱动。
func (s *BackupService) TriggerBackup(ctx context.Context, req *opsV1.TriggerBackupRequest) (*opsV1.TriggerBackupResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}

	task, err := s.taskRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("backup task not found")
	}

	nodeID := task.NodeID
	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		return nil, opsV1.ErrorBadRequest("backup task node not found")
	}

	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
	}
	if err = s.ossClient.EnsureBucketExists(ctx, backupBucket); err != nil {
		return nil, opsV1.ErrorInternalServerError("ensure bucket failed")
	}

	objectKey := fmt.Sprintf("backups/%d/%s.zip", task.ID, id.NewGUIDv4(false))
	urlObj, err := s.ossClient.GetClient().PresignedPutObject(ctx, backupBucket, objectKey, backupPresignExpiry)
	if err != nil {
		s.log.Errorf("presign backup upload url (task=%d): %s", task.ID, err.Error())
		return nil, opsV1.ErrorInternalServerError("presign url failed")
	}

	operatorID := operatorIDOf(ctx)
	payload := &signal.BackupRunPayload{
		TaskID:    task.ID,
		SourceDir: task.SourceDir,
		UploadURL: urlObj.String(),
	}
	requestId, err := s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionBackupRun, payload, operatorID)
	if err != nil {
		s.log.Errorf("dispatch backup failed (task=%d node=%d): %s", task.ID, nodeID, err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	record, err := s.recordRepo.Create(ctx, task.ID, nodeID, backupBucket, objectKey, requestId, operatorID)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("create backup record failed")
	}

	s.log.Infof("backup triggered (task=%d record=%d node=%d)", task.ID, record.ID, nodeID)
	return &opsV1.TriggerBackupResponse{
		RecordId:         &record.ID,
		CommandRequestId: trans.Ptr(requestId),
	}, nil
}

func (s *BackupService) ListBackupRecords(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBackupRecordResponse, error) {
	return s.recordRepo.List(ctx, req)
}

// GetBackupRecordDownloadUrl 成功备份的下载预签名 URL。
func (s *BackupService) GetBackupRecordDownloadUrl(ctx context.Context, req *opsV1.GetBackupRecordDownloadUrlRequest) (*opsV1.GetBackupRecordDownloadUrlResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}

	record, err := s.recordRepo.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	if record.GetStatus() != "SUCCESS" {
		return nil, opsV1.ErrorBadRequest("backup record is not successful")
	}
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
	}

	u, err := s.ossClient.GetClient().PresignedGetObject(ctx, record.GetBucket(), record.GetObjectKey(), backupDownloadExpiry, nil)
	if err != nil {
		s.log.Errorf("presign backup download url (record=%d): %s", record.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("presign url failed")
	}

	return &opsV1.GetBackupRecordDownloadUrlResponse{
		Url:       trans.Ptr(u.String()),
		ObjectKey: trans.Ptr(record.GetObjectKey()),
		SizeBytes: trans.Ptr(record.GetSizeBytes()),
	}, nil
}

// RestoreBackupRecord 恢复备份包到指定节点:预签名下载 URL 经 file_push
// 下发,agent 落盘到 target_dir/<recordId>.zip(目录须在 AGENT_ALLOWED_DIRS
// 授权白名单内,agent 侧强制校验)。v1 只做"送达",解压/替换由使用者在
// 目标机执行——恢复属于低频敏感动作,不静默自动化。
func (s *BackupService) RestoreBackupRecord(ctx context.Context, req *opsV1.RestoreBackupRecordRequest) (*opsV1.RestoreBackupRecordResponse, error) {
	if req == nil || req.GetRecordId() == 0 || req.GetNodeId() == 0 || strings.TrimSpace(req.GetTargetDir()) == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: recordId/nodeId/targetDir required")
	}

	record, err := s.recordRepo.Get(ctx, &opsV1.GetBackupRecordDownloadUrlRequest{Id: trans.Ptr(req.GetRecordId())})
	if err != nil {
		return nil, err
	}
	if record.GetStatus() != "SUCCESS" {
		return nil, opsV1.ErrorBadRequest("backup record is not successful")
	}
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
	}

	node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: trans.Ptr(req.GetNodeId())})
	if err != nil || node == nil {
		return nil, opsV1.ErrorNotFound("target node not found")
	}

	u, err := s.ossClient.GetClient().PresignedGetObject(ctx, record.GetBucket(), record.GetObjectKey(), backupDownloadExpiry, nil)
	if err != nil {
		s.log.Errorf("presign restore url (record=%d): %s", record.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("presign url failed")
	}

	fileName := fmt.Sprintf("restore-%d.zip", record.GetId())
	payload := &signal.FilePushPayload{
		Path: path.Join(strings.TrimRight(req.GetTargetDir(), "/\\"), fileName),
		Url:  u.String(),
	}
	operatorID := operatorIDOf(ctx)
	requestId, err := s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionFilePush, payload, operatorID)
	if err != nil {
		s.log.Errorf("dispatch restore failed (record=%d node=%d): %s", record.GetId(), node.GetId(), err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}
	s.log.Infof("backup restore dispatched (record=%d node=%d dir=%s requestId=%s)",
		record.GetId(), node.GetId(), req.GetTargetDir(), requestId)

	return &opsV1.RestoreBackupRecordResponse{
		RequestId: requestId,
		FileName:  fileName,
	}, nil
}
