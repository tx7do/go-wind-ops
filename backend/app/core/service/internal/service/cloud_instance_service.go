// cloud_instance_service.go 云资源实例清单:按云账号全地域同步实例快照,
// 与 agent_node 按 IP 对账;同步为整账号先删后插,单地域失败不中断其余地域。
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent/cloudaccount"
	pkgcrypto "go-wind-ops/pkg/crypto"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/cloudprovider"
)

// 同步保护参数:并发地域数与总实例上限,防失控。
const (
	syncConcurrency  = 4
	maxInstancesSync = 5000
	regionTimeout    = 20 * time.Second
)

type CloudInstanceService struct {
	opsV1.UnimplementedCloudInstanceServiceServer

	log              *log.Helper
	instanceRepo     *data.CloudInstanceRepo
	cloudAccountRepo *data.CloudAccountRepo
}

func NewCloudInstanceService(
	ctx *bootstrap.Context,
	instanceRepo *data.CloudInstanceRepo,
	cloudAccountRepo *data.CloudAccountRepo,
) *CloudInstanceService {
	return &CloudInstanceService{
		log:              ctx.NewLoggerHelper("cloud-instance/service/core-service"),
		instanceRepo:     instanceRepo,
		cloudAccountRepo: cloudAccountRepo,
	}
}

func (s *CloudInstanceService) ListCloudInstances(ctx context.Context, req *opsV1.ListCloudInstancesRequest) (*opsV1.ListCloudInstancesResponse, error) {
	return s.instanceRepo.List(ctx, req)
}

// SyncCloudAccountInstances 全地域同步:枚举地域 → 并发拉实例 → IP 对账 → 整账号替换落库。
func (s *CloudInstanceService) SyncCloudAccountInstances(ctx context.Context, req *opsV1.SyncCloudAccountInstancesRequest) (*opsV1.SyncCloudAccountInstancesResponse, error) {
	if req == nil || req.GetCloudAccountId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: cloudAccountId required")
	}

	entity, err := s.cloudAccountRepo.GetEntity(ctx, req.GetCloudAccountId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("cloud account not found")
	}
	if entity.Provider == nil || entity.AccessKey == nil || *entity.AccessKey == "" ||
		entity.SecretKey == nil || *entity.SecretKey == "" {
		return nil, opsV1.ErrorBadRequest("cloud account missing provider/accessKey/secretKey")
	}
	if *entity.Provider == cloudaccount.ProviderHuaweiCloud {
		return nil, opsV1.ErrorInternalServerError("华为云暂不支持资源同步,请通过控制台查看实例清单")
	}

	secret, decErr := pkgcrypto.DecryptIfNeeded(*entity.SecretKey)
	if decErr != nil {
		return nil, opsV1.ErrorInternalServerError("decrypt secret failed")
	}

	provider := providerVerifyName(*entity.Provider)
	ak := *entity.AccessKey
	baseRegion := ""
	if entity.Region != nil {
		baseRegion = *entity.Region
	}

	// 1) 枚举地域;失败则退化为仅账号配置地域
	regions, regionErr := cloudprovider.DescribeRegions(ctx, provider, ak, secret, baseRegion)
	errList := make([]string, 0, 4)
	if regionErr != nil || len(regions) == 0 {
		fallback := baseRegion
		if fallback == "" {
			fallback = cloudprovider.DefaultRegion(provider)
		}
		regions = []string{fallback}
		if regionErr != nil {
			errList = append(errList, fmt.Sprintf("地域枚举失败(%s),仅同步 %s: %s", provider, fallback, regionErr.Error()))
		}
	}

	// 2) 并发拉取各地域实例
	var mu sync.Mutex
	instances := make([]cloudprovider.Instance, 0, 128)
	sem := make(chan struct{}, syncConcurrency)
	var wg sync.WaitGroup
	for _, region := range regions {
		if len(instances) >= maxInstancesSync {
			break
		}
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			rctx, cancel := context.WithTimeout(ctx, regionTimeout)
			defer cancel()
			list, err := cloudprovider.ListInstancesInRegion(rctx, provider, ak, secret, region)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errList = append(errList, fmt.Sprintf("%s: %s", region, err.Error()))
				return
			}
			if len(instances)+len(list) > maxInstancesSync {
				list = list[:maxInstancesSync-len(instances)]
			}
			instances = append(instances, list...)
		}(region)
	}
	wg.Wait()

	// 3) IP 对账(agent_node 内网/公网 IP → 节点)
	ipIndex, err := s.instanceRepo.BuildIPIndex(ctx)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("build node ip index failed")
	}

	now := time.Now()
	rows := make([]*opsV1.CloudInstance, 0, len(instances))
	matched := 0
	for i := range instances {
		ins := &instances[i]
		var expired *timestamppb.Timestamp
		if ins.ExpiredAt != nil {
			expired = timestamppb.New(*ins.ExpiredAt)
		}
		row := &opsV1.CloudInstance{
			Region:       &ins.Region,
			Zone:         &ins.Zone,
			InstanceId:   &ins.InstanceID,
			InstanceName: &ins.InstanceName,
			Status:       &ins.Status,
			InstanceType: &ins.InstanceType,
			PrivateIp:    &ins.PrivateIP,
			PublicIp:     &ins.PublicIP,
			ExpiredAt:    expired,
		}
		if ref, ok := matchNode(ipIndex, ins); ok {
			matched++
			matchedFlag := true
			nodeID := ref.ID
			nodeName := ref.Name
			row.Matched = &matchedFlag
			row.AgentNodeId = &nodeID
			row.AgentNodeName = &nodeName
		} else {
			unmatched := false
			row.Matched = &unmatched
		}
		rows = append(rows, row)
	}

	// 4) 整账号替换落库
	if err := s.instanceRepo.ReplaceForAccount(ctx, req.GetCloudAccountId(), entity.Name, providerInstanceEnum(provider), rows, now); err != nil {
		s.log.Errorf("cloud instances replace failed (account=%d): %s", req.GetCloudAccountId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("replace instances failed")
	}

	s.log.Infof("cloud instances synced (account=%d provider=%s regions=%d total=%d matched=%d errors=%d operator=%d)",
		req.GetCloudAccountId(), provider, len(regions), len(rows), matched, len(errList), operatorIDOf(ctx))

	total := uint32(len(rows))
	matchedCount := uint32(matched)
	regionCount := uint32(len(regions))
	return &opsV1.SyncCloudAccountInstancesResponse{
		CloudAccountId: req.GetCloudAccountId(),
		Total:          total,
		Matched:        matchedCount,
		Regions:        regionCount,
		Errors:         errList,
	}, nil
}

// matchNode 按内网优先、公网兜底匹配节点。
func matchNode(ipIndex map[string]data.NodeRef, ins *cloudprovider.Instance) (data.NodeRef, bool) {
	if ins.PrivateIP != "" {
		if ref, ok := ipIndex[ins.PrivateIP]; ok {
			return ref, true
		}
	}
	if ins.PublicIP != "" {
		if ref, ok := ipIndex[ins.PublicIP]; ok {
			return ref, true
		}
	}
	return data.NodeRef{}, false
}

// providerInstanceEnum 同步摘要用的厂商标识 → proto 枚举。
func providerInstanceEnum(provider string) opsV1.CloudInstance_Provider {
	switch provider {
	case "TENCENT_CLOUD":
		return opsV1.CloudInstance_TENCENT_CLOUD
	case "HUAWEI_CLOUD":
		return opsV1.CloudInstance_HUAWEI_CLOUD
	case "AWS":
		return opsV1.CloudInstance_AWS
	default:
		return opsV1.CloudInstance_ALIYUN
	}
}
