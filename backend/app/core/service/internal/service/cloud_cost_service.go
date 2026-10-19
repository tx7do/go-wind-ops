// cloud_cost_service.go 云成本月账单:录入/导入/汇总。
// 轻量台账:不对接计费 API、不换汇;导入按账号名解析,逐行报错不中断。
package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent/cloudcost"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ent 枚举值别名(汇总分组的厂商名映射用)。
const (
	cloudcostProviderTencent = cloudcost.ProviderTencentCloud
	cloudcostProviderHuawei  = cloudcost.ProviderHuaweiCloud
	cloudcostProviderAWS     = cloudcost.ProviderAWS
)

// monthPattern 账单月份 YYYY-MM。
var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type CloudCostService struct {
	opsV1.UnimplementedCloudCostServiceServer

	log              *log.Helper
	costRepo         *data.CloudCostRepo
	cloudAccountRepo *data.CloudAccountRepo
}

func NewCloudCostService(
	ctx *bootstrap.Context,
	costRepo *data.CloudCostRepo,
	cloudAccountRepo *data.CloudAccountRepo,
) *CloudCostService {
	return &CloudCostService{
		log:              ctx.NewLoggerHelper("cloud-cost/service/core-service"),
		costRepo:         costRepo,
		cloudAccountRepo: cloudAccountRepo,
	}
}

func (s *CloudCostService) ListCloudCosts(ctx context.Context, req *opsV1.ListCloudCostsRequest) (*opsV1.ListCloudCostsResponse, error) {
	return s.costRepo.List(ctx, req)
}

// resolveAccount 按名称解析云账号(导入用);找不到返回错误。
func (s *CloudCostService) resolveAccount(ctx context.Context, name string) (accountID uint32, accountName, providerName string, err error) {
	if name == "" {
		return 0, "", "", fmt.Errorf("缺少云账号名")
	}
	noPaging := true
	resp, listErr := s.cloudAccountRepo.List(ctx, &paginationV1.PagingRequest{NoPaging: &noPaging})
	if listErr != nil {
		return 0, "", "", listErr
	}
	for _, acc := range resp.GetItems() {
		if acc.GetName() == name {
			return acc.GetId(), acc.GetName(), providerEnumName(acc.GetProvider()), nil
		}
	}
	return 0, "", "", fmt.Errorf("云账号不存在: %s", name)
}

// providerEnumName proto 枚举 → cloudprovider 厂商标识。
func providerEnumName(p opsV1.CloudAccount_Provider) string {
	switch p {
	case opsV1.CloudAccount_TENCENT_CLOUD:
		return "TENCENT_CLOUD"
	case opsV1.CloudAccount_HUAWEI_CLOUD:
		return "HUAWEI_CLOUD"
	case opsV1.CloudAccount_AWS:
		return "AWS"
	default:
		return "ALIYUN"
	}
}

// providerCostEnum 厂商标识 → 成本消息的厂商枚举。
func providerCostEnum(name string) opsV1.CloudCost_Provider {
	switch name {
	case "TENCENT_CLOUD":
		return opsV1.CloudCost_TENCENT_CLOUD
	case "HUAWEI_CLOUD":
		return opsV1.CloudCost_HUAWEI_CLOUD
	case "AWS":
		return opsV1.CloudCost_AWS
	default:
		return opsV1.CloudCost_ALIYUN
	}
}

func (s *CloudCostService) UpsertCloudCost(ctx context.Context, req *opsV1.UpsertCloudCostRequest) (*opsV1.CloudCost, error) {
	if req == nil || req.Data == nil || req.Data.GetCloudAccountId() == 0 ||
		!monthPattern.MatchString(req.Data.GetMonth()) {
		return nil, opsV1.ErrorBadRequest("invalid parameter: cloudAccountId and month(YYYY-MM) required")
	}
	if req.Data.GetAmount() < 0 {
		return nil, opsV1.ErrorBadRequest("amount must be >= 0")
	}

	acc, err := s.cloudAccountRepo.GetEntity(ctx, req.Data.GetCloudAccountId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("cloud account not found")
	}
	if acc.Provider == nil {
		return nil, opsV1.ErrorBadRequest("cloud account missing provider")
	}

	currency := req.Data.GetCurrency()
	if currency == "" {
		currency = "CNY"
	}

	entity, err := s.costRepo.UpsertByAccountMonth(
		ctx,
		req.Data.GetCloudAccountId(), acc.Name, providerCostEnum(providerVerifyName(*acc.Provider)),
		req.Data.GetMonth(), req.Data.GetAmount(), currency,
		req.Data.Remark, operatorIDOf(ctx),
	)
	if err != nil {
		s.log.Errorf("cloud cost upsert failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("upsert failed")
	}
	return s.costRepo.ToDTO(entity), nil
}

func (s *CloudCostService) DeleteCloudCost(ctx context.Context, req *opsV1.DeleteCloudCostRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.costRepo.Delete(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}

// ImportCloudCosts 批量导入:按账号名解析,逐行校验落库,失败行进 errors 不中断。
func (s *CloudCostService) ImportCloudCosts(ctx context.Context, req *opsV1.ImportCloudCostsRequest) (*opsV1.ImportCloudCostsResponse, error) {
	if req == nil || len(req.GetRows()) == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: rows required")
	}

	// 账号名 → 实体 缓存,免逐行查询
	accountCache := map[string]struct {
		id       uint32
		name     string
		provider string
	}{}
	resolve := func(name string) (id uint32, accName, provider string, err error) {
		if c, ok := accountCache[name]; ok {
			return c.id, c.name, c.provider, nil
		}
		id, accName, provider, err = s.resolveAccount(ctx, name)
		if err == nil {
			accountCache[name] = struct {
				id       uint32
				name     string
				provider string
			}{id, accName, provider}
		}
		return
	}

	imported := uint32(0)
	errList := make([]string, 0, 4)
	for i, row := range req.GetRows() {
		lineNo := i + 1
		if !monthPattern.MatchString(row.GetMonth()) {
			errList = append(errList, fmt.Sprintf("第%d行: 月份格式须为 YYYY-MM(%s)", lineNo, row.GetMonth()))
			continue
		}
		if row.GetAmount() < 0 {
			errList = append(errList, fmt.Sprintf("第%d行: 金额不能为负", lineNo))
			continue
		}
		accID, accName, provider, rErr := resolve(row.GetCloudAccountName())
		if rErr != nil {
			errList = append(errList, fmt.Sprintf("第%d行: %s", lineNo, rErr.Error()))
			continue
		}
		currency := row.GetCurrency()
		if currency == "" {
			currency = "CNY"
		}
		if _, uErr := s.costRepo.UpsertByAccountMonth(
			ctx, accID, accName, providerCostEnum(provider),
			row.GetMonth(), row.GetAmount(), currency, row.Remark, operatorIDOf(ctx),
		); uErr != nil {
			errList = append(errList, fmt.Sprintf("第%d行: 落库失败 %s", lineNo, uErr.Error()))
			continue
		}
		imported++
	}

	if imported == 0 && len(errList) > 0 {
		// 全部失败视为请求无效,便于前端整体报错
		return nil, opsV1.ErrorBadRequest("%s", errList[0])
	}
	return &opsV1.ImportCloudCostsResponse{Imported: imported, Errors: errList}, nil
}

// GetCloudCostSummary 月度汇总:月 × 厂商 × 币种 合计(不换汇)。
func (s *CloudCostService) GetCloudCostSummary(ctx context.Context, req *opsV1.GetCloudCostSummaryRequest) (*opsV1.GetCloudCostSummaryResponse, error) {
	rows, err := s.costRepo.ListAll(ctx, req)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("summary query failed")
	}

	type key struct {
		month    string
		provider string
		currency string
	}
	totals := map[key]float64{}
	for _, e := range rows {
		provider := "ALIYUN"
		if e.Provider != nil {
			switch *e.Provider {
			case cloudcostProviderTencent:
				provider = "TENCENT_CLOUD"
			case cloudcostProviderHuawei:
				provider = "HUAWEI_CLOUD"
			case cloudcostProviderAWS:
				provider = "AWS"
			}
		}
		totals[key{e.Month, provider, e.Currency}] += e.Amount
	}

	items := make([]*opsV1.CloudCostSummaryItem, 0, len(totals))
	for k, v := range totals {
		month := k.month
		provider := k.provider
		currency := k.currency
		total := v
		items = append(items, &opsV1.CloudCostSummaryItem{
			Month:    month,
			Provider: provider,
			Currency: currency,
			Total:    total,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].GetMonth() != items[j].GetMonth() {
			return items[i].GetMonth() < items[j].GetMonth()
		}
		return items[i].GetProvider() < items[j].GetProvider()
	})
	return &opsV1.GetCloudCostSummaryResponse{Items: items}, nil
}
