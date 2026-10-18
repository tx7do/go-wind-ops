package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/oss"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// artifactUrlExpiry 产物下载预签名有效期。
const artifactUrlExpiry = time.Hour

// BuildHistoryService 构建历史查询与产物下载(只读)。
type BuildHistoryService struct {
	opsV1.UnimplementedBuildHistoryServiceServer

	log         *log.Helper
	repo        *data.BuildHistoryRepo
	buildRepo   *data.BuildRepo
	repoRepo    *data.RepositoryRepo
	ossClient   *oss.MinIOClient
}

func NewBuildHistoryService(ctx *bootstrap.Context, repo *data.BuildHistoryRepo, buildRepo *data.BuildRepo, repoRepo *data.RepositoryRepo, ossClient *oss.MinIOClient) *BuildHistoryService {
	return &BuildHistoryService{
		log:       ctx.NewLoggerHelper("build-history/service/core-service"),
		repo:      repo,
		buildRepo: buildRepo,
		repoRepo:  repoRepo,
		ossClient: ossClient,
	}
}

func (s *BuildHistoryService) ListBuildHistories(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBuildHistoryResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *BuildHistoryService) GetBuildHistory(ctx context.Context, req *opsV1.GetBuildHistoryRequest) (*opsV1.BuildHistory, error) {
	return s.repo.Get(ctx, req)
}

// GetArtifactDownloadUrl 签发产物下载预签名 URL(1 小时有效)。
// GetBuildHistoryLog 增量返回构建日志:offset 为调用方已持有的字节长度,
// 返回自该偏移起的新增片段。日志只追加,字节偏移即安全切分点;若服务端
// 日志短于 offset(异常截断),退化为全量返回。
func (s *BuildHistoryService) GetBuildHistoryLog(ctx context.Context, req *opsV1.GetBuildHistoryLogRequest) (*opsV1.GetBuildHistoryLogResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	entity, err := s.repo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("build history not found")
	}

	logText := ""
	if entity.Log != nil {
		logText = *entity.Log
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(logText)) {
		offset = 0
	}

	delta := logText[offset:]
	newOffset := int64(len(logText))
	status := ""
	if entity.Status != nil {
		status = string(*entity.Status)
	}
	finished := status == "SUCCESS" || status == "FAILED"

	errMsg := ""
	if entity.Error != nil {
		errMsg = *entity.Error
	}

	return &opsV1.GetBuildHistoryLogResponse{
		Delta:     &delta,
		NewOffset: &newOffset,
		Status:    &status,
		Finished:  &finished,
		Error:     &errMsg,
	}, nil
}

func (s *BuildHistoryService) GetArtifactDownloadUrl(ctx context.Context, req *opsV1.GetArtifactDownloadUrlRequest) (*opsV1.GetArtifactDownloadUrlResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	history, err := s.repo.Get(ctx, &opsV1.GetBuildHistoryRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	if history.GetStatus() != opsV1.BuildHistory_SUCCESS || history.GetArtifactObjectKey() == "" {
		return nil, opsV1.ErrorBadRequest("artifact not available for this build history")
	}

	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return nil, opsV1.ErrorServiceUnavailable("oss client not configured")
	}

	urlObj, err := s.ossClient.GetClient().PresignedGetObject(ctx, history.GetArtifactBucket(), history.GetArtifactObjectKey(), artifactUrlExpiry, nil)
	if err != nil {
		s.log.Errorf("presign artifact url [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("presign url failed")
	}

	return &opsV1.GetArtifactDownloadUrlResponse{
		Url:       urlObj.String(),
		ObjectKey: history.GetArtifactObjectKey(),
		Size:      history.GetArtifactSize(),
	}, nil
}
