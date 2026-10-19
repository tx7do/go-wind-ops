package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/cloudcost"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CloudCostRepo 云成本月账单仓储(手写转换,同 api_key 范式)。
type CloudCostRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewCloudCostRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CloudCostRepo {
	return &CloudCostRepo{
		log:       ctx.NewLoggerHelper("cloud-cost/repo/core-service"),
		entClient: entClient,
	}
}

// ToDTO 实体 → proto(导出供 service 层 upsert 后返回)。
func (r *CloudCostRepo) ToDTO(e *ent.CloudCost) *opsV1.CloudCost {
	return r.toDTO(e)
}

// toDTO 实体 → proto。
func (r *CloudCostRepo) toDTO(e *ent.CloudCost) *opsV1.CloudCost {
	month := e.Month
	dto := &opsV1.CloudCost{
		Id:               &e.ID,
		CloudAccountId:   &e.CloudAccountID,
		CloudAccountName: e.CloudAccountName,
		Month:            &month,
		Amount:           &e.Amount,
		Currency:         &e.Currency,
		Remark:           e.Remark,
		CreatedAt:        timeTimestamppb(e.CreatedAt),
		UpdatedAt:        timeTimestamppb(e.UpdatedAt),
	}
	if e.Provider != nil {
		provider := providerCostToProto(*e.Provider)
		dto.Provider = &provider
	}
	return dto
}

// providerCostToProto ent 枚举 → proto 枚举。
func providerCostToProto(p cloudcost.Provider) opsV1.CloudCost_Provider {
	switch p {
	case cloudcost.ProviderTencentCloud:
		return opsV1.CloudCost_TENCENT_CLOUD
	case cloudcost.ProviderHuaweiCloud:
		return opsV1.CloudCost_HUAWEI_CLOUD
	case cloudcost.ProviderAWS:
		return opsV1.CloudCost_AWS
	default:
		return opsV1.CloudCost_ALIYUN
	}
}

// providerCostToEnt proto 枚举 → ent 枚举。
func providerCostToEnt(p opsV1.CloudCost_Provider) cloudcost.Provider {
	switch p {
	case opsV1.CloudCost_TENCENT_CLOUD:
		return cloudcost.ProviderTencentCloud
	case opsV1.CloudCost_HUAWEI_CLOUD:
		return cloudcost.ProviderHuaweiCloud
	case opsV1.CloudCost_AWS:
		return cloudcost.ProviderAWS
	default:
		return cloudcost.ProviderAliyun
	}
}

// costPreds 组装过滤条件。
func costPreds(req *opsV1.ListCloudCostsRequest) []predicate.CloudCost {
	preds := make([]predicate.CloudCost, 0, 3)
	if req == nil {
		return preds
	}
	if req.CloudAccountId != nil && req.GetCloudAccountId() > 0 {
		preds = append(preds, cloudcost.CloudAccountIDEQ(req.GetCloudAccountId()))
	}
	if req.Year != nil && req.GetYear() != "" {
		// month 形如 YYYY-MM,按年前缀匹配
		preds = append(preds, cloudcost.MonthHasPrefix(req.GetYear()+"-"))
	}
	if req.Provider != nil && req.GetProvider() != "" {
		switch req.GetProvider() {
		case "TENCENT_CLOUD":
			preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderTencentCloud))
		case "HUAWEI_CLOUD":
			preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderHuaweiCloud))
		case "AWS":
			preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderAWS))
		default:
			preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderAliyun))
		}
	}
	return preds
}

// List 过滤分页(month 倒序)。
func (r *CloudCostRepo) List(ctx context.Context, req *opsV1.ListCloudCostsRequest) (*opsV1.ListCloudCostsResponse, error) {
	client := r.entClient.Client().CloudCost

	preds := costPreds(req)
	query := client.Query()
	if len(preds) > 0 {
		query = query.Where(cloudcost.And(preds...))
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

	rows, err := query.Order(ent.Desc(cloudcost.FieldMonth), ent.Asc(cloudcost.FieldCloudAccountID)).
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	resp := &opsV1.ListCloudCostsResponse{Total: uint64(total)}
	for _, e := range rows {
		resp.Items = append(resp.Items, r.toDTO(e))
	}
	return resp, nil
}

// ListAll 不过滤全量(汇总用;量级=账号×月份数,内存聚合足够)。
func (r *CloudCostRepo) ListAll(ctx context.Context, req *opsV1.GetCloudCostSummaryRequest) ([]*ent.CloudCost, error) {
	preds := make([]predicate.CloudCost, 0, 2)
	if req != nil {
		if req.Year != nil && req.GetYear() != "" {
			preds = append(preds, cloudcost.MonthHasPrefix(req.GetYear()+"-"))
		}
		if req.Provider != nil && req.GetProvider() != "" {
			switch req.GetProvider() {
			case "TENCENT_CLOUD":
				preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderTencentCloud))
			case "HUAWEI_CLOUD":
				preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderHuaweiCloud))
			case "AWS":
				preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderAWS))
			default:
				preds = append(preds, cloudcost.ProviderEQ(cloudcost.ProviderAliyun))
			}
		}
	}
	query := r.entClient.Client().CloudCost.Query()
	if len(preds) > 0 {
		query = query.Where(cloudcost.And(preds...))
	}
	return query.All(ctx)
}

// SumMonthByCurrency 按月+币种汇总成本,返回总额与按账号的细分(预算检查用)。
func (r *CloudCostRepo) SumMonthByCurrency(ctx context.Context, month, currency string) (float64, map[uint32]float64, error) {
	rows, err := r.entClient.Client().CloudCost.
		Query().
		Where(
			cloudcost.MonthEQ(month),
			cloudcost.CurrencyEQ(currency),
		).
		All(ctx)
	if err != nil {
		return 0, nil, err
	}
	total := 0.0
	perAccount := make(map[uint32]float64)
	for _, row := range rows {
		total += row.Amount
		perAccount[row.CloudAccountID] += row.Amount
	}
	return total, perAccount, nil
}

// SumMonthsByCurrency 汇总多个月份(排除 skipMonth)的总额(环比均值用)。
func (r *CloudCostRepo) SumMonthsByCurrency(ctx context.Context, months []string, currency string) (float64, error) {
	rows, err := r.entClient.Client().CloudCost.
		Query().
		Where(
			cloudcost.MonthIn(months...),
			cloudcost.CurrencyEQ(currency),
		).
		All(ctx)
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, row := range rows {
		total += row.Amount
	}
	return total, nil
}

// UpsertByAccountMonth 按账号+月份 upsert(存在则更新金额/币种/备注/厂商快照)。
func (r *CloudCostRepo) UpsertByAccountMonth(ctx context.Context, accountID uint32, accountName string, provider opsV1.CloudCost_Provider, month string, amount float64, currency string, remark *string, operatorID uint32) (*ent.CloudCost, error) {
	client := r.entClient.Client().CloudCost

	existing, err := client.Query().
		Where(
			cloudcost.CloudAccountIDEQ(accountID),
			cloudcost.MonthEQ(month),
		).
		First(ctx)
	if err == nil && existing != nil {
		builder := client.UpdateOneID(existing.ID).
			SetAmount(amount).
			SetCurrency(currency).
			SetProvider(providerCostToEnt(provider))
		if accountName != "" {
			builder.SetCloudAccountName(accountName)
		}
		if remark != nil {
			builder.SetRemark(*remark)
		}
		if operatorID > 0 {
			builder.SetUpdatedBy(operatorID)
		}
		return builder.Save(ctx)
	}

	builder := client.Create().
		SetCloudAccountID(accountID).
		SetMonth(month).
		SetAmount(amount).
		SetCurrency(currency).
		SetProvider(providerCostToEnt(provider))
	if accountName != "" {
		builder.SetCloudAccountName(accountName)
	}
	if remark != nil {
		builder.SetRemark(*remark)
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	return builder.Save(ctx)
}

// Delete 删除。
func (r *CloudCostRepo) Delete(ctx context.Context, id uint32) error {
	return r.entClient.Client().CloudCost.DeleteOneID(id).Exec(ctx)
}
