package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/cloudinstance"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CloudInstanceRepo 云资源实例清单仓储(手写转换,同 api_key 范式)。
type CloudInstanceRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewCloudInstanceRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CloudInstanceRepo {
	return &CloudInstanceRepo{
		log:       ctx.NewLoggerHelper("cloud-instance/repo/core-service"),
		entClient: entClient,
	}
}

// toDTO 实体 → proto。
func (r *CloudInstanceRepo) toDTO(e *ent.CloudInstance) *opsV1.CloudInstance {
	region := e.Region
	instanceID := e.InstanceID
	dto := &opsV1.CloudInstance{
		Id:               &e.ID,
		CloudAccountId:   &e.CloudAccountID,
		CloudAccountName: e.CloudAccountName,
		Region:           &region,
		Zone:             e.Zone,
		InstanceId:       &instanceID,
		InstanceName:     e.InstanceName,
		Status:           e.Status,
		InstanceType:     e.InstanceType,
		PrivateIp:        e.PrivateIP,
		PublicIp:         e.PublicIP,
		Matched:          &e.Matched,
		AgentNodeId:      e.AgentNodeID,
		AgentNodeName:    e.AgentNodeName,
		SyncedAt:         timeTimestamppb(e.SyncedAt),
		CreatedAt:        timeTimestamppb(e.CreatedAt),
		ExpiredAt:        timeTimestamppb(e.ExpiredAt),
	}
	if e.Provider != nil {
		provider := providerInstanceToProto(*e.Provider)
		dto.Provider = &provider
	}
	return dto
}

// providerInstanceToProto ent 枚举 → proto 枚举(与账号同一套值名,单独函数避免类型不匹配)。
func providerInstanceToProto(p cloudinstance.Provider) opsV1.CloudInstance_Provider {
	switch p {
	case cloudinstance.ProviderTencentCloud:
		return opsV1.CloudInstance_TENCENT_CLOUD
	case cloudinstance.ProviderHuaweiCloud:
		return opsV1.CloudInstance_HUAWEI_CLOUD
	case cloudinstance.ProviderAWS:
		return opsV1.CloudInstance_AWS
	default:
		return opsV1.CloudInstance_ALIYUN
	}
}

// providerInstanceToEnt proto 枚举 → ent 枚举。
func providerInstanceToEnt(p opsV1.CloudInstance_Provider) cloudinstance.Provider {
	switch p {
	case opsV1.CloudInstance_TENCENT_CLOUD:
		return cloudinstance.ProviderTencentCloud
	case opsV1.CloudInstance_HUAWEI_CLOUD:
		return cloudinstance.ProviderHuaweiCloud
	case opsV1.CloudInstance_AWS:
		return cloudinstance.ProviderAWS
	default:
		return cloudinstance.ProviderAliyun
	}
}

// List 过滤分页查询。
func (r *CloudInstanceRepo) List(ctx context.Context, req *opsV1.ListCloudInstancesRequest) (*opsV1.ListCloudInstancesResponse, error) {
	client := r.entClient.Client().CloudInstance

	preds := make([]predicate.CloudInstance, 0, 5)
	if req != nil {
		if req.CloudAccountId != nil && req.GetCloudAccountId() > 0 {
			preds = append(preds, cloudinstance.CloudAccountIDEQ(req.GetCloudAccountId()))
		}
		if req.Provider != nil && req.GetProvider() != "" {
			switch req.GetProvider() {
			case "TENCENT_CLOUD":
				preds = append(preds, cloudinstance.ProviderEQ(cloudinstance.ProviderTencentCloud))
			case "HUAWEI_CLOUD":
				preds = append(preds, cloudinstance.ProviderEQ(cloudinstance.ProviderHuaweiCloud))
			case "AWS":
				preds = append(preds, cloudinstance.ProviderEQ(cloudinstance.ProviderAWS))
			default:
				preds = append(preds, cloudinstance.ProviderEQ(cloudinstance.ProviderAliyun))
			}
		}
		if req.Region != nil && req.GetRegion() != "" {
			preds = append(preds, cloudinstance.RegionEQ(req.GetRegion()))
		}
		if req.Matched != nil {
			preds = append(preds, cloudinstance.MatchedEQ(req.GetMatched()))
		}
		if req.Keyword != nil && req.GetKeyword() != "" {
			preds = append(preds, cloudinstance.Or(
				cloudinstance.InstanceIDContains(req.GetKeyword()),
				cloudinstance.InstanceNameContains(req.GetKeyword()),
				cloudinstance.PrivateIPContains(req.GetKeyword()),
				cloudinstance.PublicIPContains(req.GetKeyword()),
			))
		}
	}

	query := client.Query()
	if len(preds) > 0 {
		query = query.Where(cloudinstance.And(preds...))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, err
	}

	page := uint32(1)
	pageSize := uint32(50)
	if req != nil {
		if req.GetPage() > 0 {
			page = req.GetPage()
		}
		if req.GetPageSize() > 0 {
			pageSize = req.GetPageSize()
		}
	}

	qb := query.Order(ent.Desc(cloudinstance.FieldSyncedAt), ent.Asc(cloudinstance.FieldRegion), ent.Asc(cloudinstance.FieldInstanceID)).
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize))

	rows, err := qb.All(ctx)
	if err != nil {
		return nil, err
	}

	resp := &opsV1.ListCloudInstancesResponse{Total: uint64(total)}
	for _, e := range rows {
		resp.Items = append(resp.Items, r.toDTO(e))
	}
	return resp, nil
}

// NodeRef 节点匹配引用。
type NodeRef struct {
	ID   uint32
	Name string
}

// BuildIPIndex 建节点 IP → 节点引用索引(对账用)。
func (r *CloudInstanceRepo) BuildIPIndex(ctx context.Context) (map[string]NodeRef, error) {
	nodes, err := r.entClient.Client().AgentNode.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	index := make(map[string]NodeRef, len(nodes)*2)
	for _, n := range nodes {
		name := ""
		if n.Name != nil {
			name = *n.Name
		}
		ref := NodeRef{ID: n.ID, Name: name}
		for _, ip := range n.Ips {
			if ip != "" {
				index[ip] = ref
			}
		}
	}
	return index, nil
}

// ReplaceForAccount 整账号全量替换:删旧插新(单批次事务语义由顺序执行保证,失败留下批数据可重试)。
func (r *CloudInstanceRepo) ReplaceForAccount(ctx context.Context, accountID uint32, accountName string, provider opsV1.CloudInstance_Provider, rows []*opsV1.CloudInstance, syncedAt time.Time) error {
	client := r.entClient.Client().CloudInstance

	if _, err := client.Delete().
		Where(cloudinstance.CloudAccountIDEQ(accountID)).
		Exec(ctx); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	provEnt := providerInstanceToEnt(provider)
	builders := make([]*ent.CloudInstanceCreate, 0, len(rows))
	for _, row := range rows {
		b := client.Create().
			SetCloudAccountID(accountID).
			SetCloudAccountName(accountName).
			SetProvider(provEnt).
			SetRegion(row.GetRegion()).
			SetInstanceID(row.GetInstanceId()).
			SetSyncedAt(syncedAt)
		if row.GetZone() != "" {
			b.SetZone(row.GetZone())
		}
		if row.GetInstanceName() != "" {
			b.SetInstanceName(row.GetInstanceName())
		}
		if row.GetStatus() != "" {
			b.SetStatus(row.GetStatus())
		}
		if row.GetInstanceType() != "" {
			b.SetInstanceType(row.GetInstanceType())
		}
		if row.GetPrivateIp() != "" {
			b.SetPrivateIP(row.GetPrivateIp())
		}
		if row.GetPublicIp() != "" {
			b.SetPublicIP(row.GetPublicIp())
		}
		if t := row.GetExpiredAt(); t != nil {
			b.SetExpiredAt(t.AsTime())
		}
		if row.GetMatched() {
			b.SetMatched(true)
			b.SetAgentNodeID(row.GetAgentNodeId())
			b.SetAgentNodeName(row.GetAgentNodeName())
		}
		builders = append(builders, b)
	}
	if _, err := client.CreateBulk(builders...).Save(ctx); err != nil {
		return err
	}
	return nil
}

// CountByAccount 按账号统计实例数(清单页概览用)。
func (r *CloudInstanceRepo) CountByAccount(ctx context.Context, accountID uint32) (int, error) {
	return r.entClient.Client().CloudInstance.Query().
		Where(cloudinstance.CloudAccountIDEQ(accountID)).
		Count(ctx)
}
