package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CloudBudgetService 云成本月度预算:CRUD + 周期检查(阈值/环比突增告警)。
// 预算告警落 ops_alerts(source=BUDGET,monitor_id=预算 ID 供去重/恢复),
// 并经通知渠道外呼(投递台账随之落库)。
type CloudBudgetService struct {
	opsV1.UnimplementedCloudBudgetServiceServer

	log        *log.Helper
	budgetRepo *data.CloudBudgetRepo
	costRepo   *data.CloudCostRepo
	accountRepo *data.CloudAccountRepo
	alertRepo  *data.AlertRepo
	notifChannelRepo *data.NotificationChannelRepo
	deliveryRepo     *data.NotificationDeliveryRepo
}

func NewCloudBudgetService(ctx *bootstrap.Context, budgetRepo *data.CloudBudgetRepo, costRepo *data.CloudCostRepo, accountRepo *data.CloudAccountRepo, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo) *CloudBudgetService {
	return &CloudBudgetService{
		log:              ctx.NewLoggerHelper("cloud-budget/service/core-service"),
		budgetRepo:       budgetRepo,
		costRepo:         costRepo,
		accountRepo:      accountRepo,
		alertRepo:        alertRepo,
		notifChannelRepo: notifChannelRepo,
		deliveryRepo:     deliveryRepo,
	}
}

func (s *CloudBudgetService) ListCloudBudgets(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCloudBudgetResponse, error) {
	return s.budgetRepo.List(ctx, req)
}

func (s *CloudBudgetService) GetCloudBudget(ctx context.Context, req *opsV1.GetCloudBudgetRequest) (*opsV1.CloudBudget, error) {
	return s.budgetRepo.Get(ctx, req)
}

func (s *CloudBudgetService) CreateCloudBudget(ctx context.Context, req *opsV1.CreateCloudBudgetRequest) (*opsV1.CloudBudget, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetMonth() == "" || req.Data.GetAmount() <= 0 {
		return nil, opsV1.ErrorBadRequest("month/amount are required")
	}
	if len(req.Data.GetMonth()) != 7 || req.Data.GetMonth()[4] != '-' {
		return nil, opsV1.ErrorBadRequest("month must be YYYY-MM")
	}
	if req.Data.GetThresholdPercent() <= 0 {
		req.Data.ThresholdPercent = transPtrFloat(100)
	}
	if req.Data.GetCurrency() == "" {
		req.Data.Currency = transPtrString("CNY")
	}

	taken, err := s.budgetRepo.IsKeyTaken(ctx, req.Data.GetCloudAccountId(), req.Data.GetMonth(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("budget for this account+month already exists")
	}

	return s.budgetRepo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *CloudBudgetService) UpdateCloudBudget(ctx context.Context, req *opsV1.UpdateCloudBudgetRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.Month != nil && len(req.Data.GetMonth()) != 7 {
		return nil, opsV1.ErrorBadRequest("month must be YYYY-MM")
	}

	if req.Data.Month != nil || req.Data.CloudAccountId != nil {
		accountID := req.Data.GetCloudAccountId()
		month := req.Data.GetMonth()
		if req.Data.Month == nil || req.Data.CloudAccountId == nil {
			cur, err := s.budgetRepo.GetEntity(ctx, req.GetId())
			if err != nil {
				return nil, opsV1.ErrorNotFound("cloud budget not found")
			}
			if req.Data.Month == nil {
				month = cur.Month
			}
			if req.Data.CloudAccountId == nil {
				accountID = cur.CloudAccountID
			}
		}
		taken, err := s.budgetRepo.IsKeyTaken(ctx, accountID, month, req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("budget for this account+month already exists")
		}
	}

	if err := s.budgetRepo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CloudBudgetService) DeleteCloudBudget(ctx context.Context, req *opsV1.DeleteCloudBudgetRequest) (*emptypb.Empty, error) {
	if err := s.budgetRepo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// CheckCloudBudget 手动触发一次检查(force=重置防重位后立即重算)。
func (s *CloudBudgetService) CheckCloudBudget(ctx context.Context, req *opsV1.CheckCloudBudgetRequest) (*emptypb.Empty, error) {
	if err := s.RunBudgetCheck(ctx, true); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// RunBudgetCheck 预算检查主逻辑:仅当月预算参与;force=true 时先清防重位。
func (s *CloudBudgetService) RunBudgetCheck(ctx context.Context, force bool) error {
	budgets, err := s.budgetRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	curMonth := time.Now().Format("2006-01")
	for _, b := range budgets {
		if b.Month != curMonth {
			continue
		}
		if force {
			_ = s.budgetRepo.SetNotified(ctx, b.ID, false)
			_ = s.budgetRepo.SetAnomalyNotified(ctx, b.ID, false)
		}

		total, perAccount, err := s.costRepo.SumMonthByCurrency(ctx, b.Month, b.Currency)
		if err != nil {
			s.log.Warnf("budget [%d] sum costs: %s", b.ID, err.Error())
			continue
		}
		subject := s.budgetSubject(ctx, b.CloudAccountID)

		// ── 阈值检查 ──
		threshold := b.Amount * b.ThresholdPercent / 100
		if b.Amount > 0 && threshold > 0 && total >= threshold && !b.Notified {
			already, _ := s.alertRepo.HasUnresolvedAlert(ctx, b.ID)
			if !already {
				msg := fmt.Sprintf("云成本预算 [%s %s] %s 已达 %.1f %s(预算 %.1f 的 %.0f%%)%s",
					subject, b.Month, b.Currency, total, b.Currency, b.Amount, b.ThresholdPercent, budgetScopeSuffix(b.CloudAccountID))
				s.fireBudgetAlert(ctx, b, subject, msg)
			}
			_ = s.budgetRepo.SetNotified(ctx, b.ID, true)
		}

		// ── 环比突增检查(前 3 个自然月均值)──
		if b.AnomalyRatio > 0 && !b.AnomalyNotified {
			prevMonths := prevMonths(b.Month, 3)
			var prevTotal float64
			if prevTotal, err = s.costRepo.SumMonthsByCurrency(ctx, prevMonths, b.Currency); err == nil {
				avg := prevTotal / float64(len(prevMonths))
				if avg > 0 && total > avg*b.AnomalyRatio {
					already, _ := s.alertRepo.HasUnresolvedAlert(ctx, b.ID)
					if !already {
						_ = perAccount
						msg := fmt.Sprintf("云成本突增 [%s %s] %s 本月 %.1f,超前 3 月均值(%.1f)的 %.1f 倍%s",
							subject, b.Month, b.Currency, total, avg, b.AnomalyRatio, budgetScopeSuffix(b.CloudAccountID))
						s.fireBudgetAlert(ctx, b, subject, msg)
					}
					_ = s.budgetRepo.SetAnomalyNotified(ctx, b.ID, true)
				}
			}
		}
	}
	return nil
}

// fireBudgetAlert 落告警(source=BUDGET,monitor_id=预算 ID)并通知渠道。
func (s *CloudBudgetService) fireBudgetAlert(ctx context.Context, b *ent.CloudBudget, subject, msg string) {
	a, err := s.alertRepo.Create(ctx, b.ID, 0, 0, "BUDGET", "WARNING", msg, false)
	if err != nil {
		s.log.Errorf("create budget alert failed: %s", err.Error())
		return
	}
	SendAlertToChannels(ctx, s.notifChannelRepo, s.deliveryRepo, a.ID, "云成本告警: "+subject, msg)
	s.log.Warnf("BUDGET ALERT: %s", msg)
}

// budgetSubject 预算主体名(账号名或"全部账号")。
func (s *CloudBudgetService) budgetSubject(ctx context.Context, accountID uint32) string {
	if accountID == 0 {
		return "全部账号"
	}
	acc, err := s.accountRepo.GetEntity(ctx, accountID)
	if err != nil || acc == nil {
		return fmt.Sprintf("账号#%d", accountID)
	}
	return acc.Name
}

// prevMonths 取给定 YYYY-MM 之前的 N 个自然月。
func prevMonths(month string, n int) []string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return nil
	}
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, t.AddDate(0, -i, 0).Format("2006-01"))
	}
	return out
}

func budgetScopeSuffix(accountID uint32) string {
	if accountID == 0 {
		return "(全部账号合计)"
	}
	return ""
}

func transPtrFloat(v float64) *float64 {
	return &v
}

func transPtrString(v string) *string {
	return &v
}
