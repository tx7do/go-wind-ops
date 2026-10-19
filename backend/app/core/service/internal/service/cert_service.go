package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const certProbeTimeout = 10 * time.Second

// CertService 证书到期台账:CRUD + TLS 直连探测 + 到期告警检查。
// 证书告警落 ops_alerts(source=CERT,monitor_id=证书 ID 供去重/恢复),
// 并经通知渠道外呼。
type CertService struct {
	opsV1.UnimplementedCertServiceServer

	log              *log.Helper
	certRepo         *data.CertRepo
	alertRepo        *data.AlertRepo
	notifChannelRepo *data.NotificationChannelRepo
	deliveryRepo     *data.NotificationDeliveryRepo
}

func NewCertService(ctx *bootstrap.Context, certRepo *data.CertRepo, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo) *CertService {
	return &CertService{
		log:              ctx.NewLoggerHelper("cert/service/core-service"),
		certRepo:         certRepo,
		alertRepo:        alertRepo,
		notifChannelRepo: notifChannelRepo,
		deliveryRepo:     deliveryRepo,
	}
}

func (s *CertService) ListCerts(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCertResponse, error) {
	return s.certRepo.List(ctx, req)
}

func (s *CertService) GetCert(ctx context.Context, req *opsV1.GetCertRequest) (*opsV1.Cert, error) {
	return s.certRepo.Get(ctx, req)
}

func (s *CertService) CreateCert(ctx context.Context, req *opsV1.CreateCertRequest) (*opsV1.Cert, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetHost() == "" {
		return nil, opsV1.ErrorBadRequest("name/host are required")
	}
	if req.Data.GetProbeCron() != "" {
		if _, err := cron.ParseStandard(req.Data.GetProbeCron()); err != nil {
			return nil, opsV1.ErrorBadRequest("invalid cron expression")
		}
	}

	taken, err := s.certRepo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("cert name already exists")
	}

	return s.certRepo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *CertService) UpdateCert(ctx context.Context, req *opsV1.UpdateCertRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetProbeCron() != "" {
		if _, err := cron.ParseStandard(req.Data.GetProbeCron()); err != nil {
			return nil, opsV1.ErrorBadRequest("invalid cron expression")
		}
	}
	if req.Data.Name != nil {
		taken, err := s.certRepo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("cert name already exists")
		}
	}

	if err := s.certRepo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CertService) DeleteCert(ctx context.Context, req *opsV1.DeleteCertRequest) (*emptypb.Empty, error) {
	if err := s.certRepo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ProbeCert 立即探测一次:TLS 握手读取对端证书并回填台账。
func (s *CertService) ProbeCert(ctx context.Context, req *opsV1.ProbeCertRequest) (*opsV1.ProbeCertResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	row, err := s.certRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("cert not found")
	}

	issuer, notBefore, notAfter, probeErr := probeTLS(ctx, row.Host, int(row.Port))
	if probeErr != nil {
		_ = s.certRepo.ApplyProbeResult(ctx, row.ID, "", nil, nil, probeErr.Error())
		return &opsV1.ProbeCertResponse{Error: transPtrString(probeErr.Error())}, nil
	}
	_ = s.certRepo.ApplyProbeResult(ctx, row.ID, issuer, notBefore, notAfter, "")

	return &opsV1.ProbeCertResponse{
		Issuer:        transPtrString(issuer),
		NotAfter:      timestamppb.New(*notAfter),
		RemainingDays: transPtrInt32(int32(time.Until(*notAfter).Hours() / 24)),
	}, nil
}

// probeTLS 直连 host:port 做 TLS 握手读取对端证书(不校验链,只取有效期)。
func probeTLS(ctx context.Context, host string, port int) (issuer string, notBefore, notAfter *time.Time, err error) {
	if port <= 0 {
		port = 443
	}
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: certProbeTimeout},
		Config:    &tls.Config{InsecureSkipVerify: true, ServerName: host},
	}
	nc, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return "", nil, nil, err
	}
	defer nc.Close()
	conn, ok := nc.(*tls.Conn)
	if !ok {
		return "", nil, nil, fmt.Errorf("not a tls connection")
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", nil, nil, fmt.Errorf("no peer certificate")
	}
	leaf := state.PeerCertificates[0]
	issuer = leaf.Issuer.String()
	if leaf.Issuer.CommonName != "" {
		issuer = leaf.Issuer.CommonName
	} else if len(leaf.Issuer.Organization) > 0 {
		issuer = leaf.Issuer.Organization[0]
	}
	nb, na := leaf.NotBefore, leaf.NotAfter
	return issuer, &nb, &na, nil
}

// RunScheduledProbes 调度探测:配置了 probe_cron 且距上次探测跨越了下一个
// 触发点的证书执行探测(last_probed_at 作锚点,探测幂等)。
func (s *CertService) RunScheduledProbes(ctx context.Context) error {
	rows, err := s.certRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, row := range rows {
		spec := row.ProbeCron
		if spec == "" {
			continue
		}
		schedule, err := cron.ParseStandard(spec)
		if err != nil {
			continue
		}
		last := now.Add(-24 * time.Hour)
		if row.LastProbedAt != nil {
			last = row.LastProbedAt.In(time.Local)
		}
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		s.probeAndNotify(ctx, row)
	}
	return nil
}

// RunExpiryCheck 到期检查:剩余天数 ≤ 阈值且无未恢复告警 → 告警 + 外呼;
// 续期后(剩余天数回升)自动 resolve + RECOVERY 记录 + 恢复通知。
func (s *CertService) RunExpiryCheck(ctx context.Context) error {
	rows, err := s.certRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.NotAfter == nil {
			continue
		}
		remaining := time.Until(*row.NotAfter)
		name := row.Name
		if name == "" {
			name = row.Host
		}

		if remaining <= 0 {
			if already, _ := s.alertRepo.HasUnresolvedAlert(ctx, row.ID); !already {
				msg := fmt.Sprintf("证书 [%s](%s) 已于 %s 过期", name, row.Host, row.NotAfter.Format("2006-01-02"))
				s.fireCertAlert(ctx, row, msg)
			}
			continue
		}
		if int(remaining.Hours()/24) <= row.DaysBeforeExpire {
			if already, _ := s.alertRepo.HasUnresolvedAlert(ctx, row.ID); !already {
				msg := fmt.Sprintf("证书 [%s](%s) 将于 %s 到期,剩余 %d 天",
					name, row.Host, row.NotAfter.Format("2006-01-02"), int(remaining.Hours()/24))
				s.fireCertAlert(ctx, row, msg)
			}
			continue
		}

		// 未到告警窗口:如有历史告警(续期后)则恢复
		unresolved, _ := s.alertRepo.HasUnresolvedAlert(ctx, row.ID)
		if !unresolved {
			continue
		}
		if _, err = s.alertRepo.ResolveAllByMonitor(ctx, row.ID); err != nil {
			s.log.Warnf("resolve cert alerts [%d]: %s", row.ID, err.Error())
		}
		msg := fmt.Sprintf("证书 [%s](%s) 已续期,新到期时间 %s,告警恢复",
			name, row.Host, row.NotAfter.Format("2006-01-02"))
		if a, err := s.alertRepo.CreateRecovery(ctx, row.ID, 0, 0, "CERT", msg); err != nil {
			s.log.Errorf("create cert recovery alert failed: %s", err.Error())
		} else {
			SendAlertToChannels(ctx, s.notifChannelRepo, s.deliveryRepo, a.ID, "证书恢复: "+name, msg)
		}
	}
	return nil
}

// fireCertAlert 落告警(source=CERT,monitor_id=证书 ID)并通知渠道。
func (s *CertService) fireCertAlert(ctx context.Context, row *ent.Cert, msg string) {
	a, err := s.alertRepo.Create(ctx, row.ID, 0, 0, "CERT", "WARNING", msg, false)
	if err != nil {
		s.log.Errorf("create cert alert failed: %s", err.Error())
		return
	}
	name := row.Name
	if name == "" {
		name = row.Host
	}
	SendAlertToChannels(ctx, s.notifChannelRepo, s.deliveryRepo, a.ID, "证书告警: "+name, msg)
	s.log.Warnf("CERT ALERT: %s", msg)
}

// probeAndNotify 探测并更新台账(调度用;失败只落 last_error,不告警)。
func (s *CertService) probeAndNotify(ctx context.Context, row *ent.Cert) {
	issuer, notBefore, notAfter, err := probeTLS(ctx, row.Host, int(row.Port))
	if err != nil {
		_ = s.certRepo.ApplyProbeResult(ctx, row.ID, "", nil, nil, err.Error())
		s.log.Warnf("cert probe failed (cert=%d host=%s): %s", row.ID, row.Host, err.Error())
		return
	}
	_ = s.certRepo.ApplyProbeResult(ctx, row.ID, issuer, notBefore, notAfter, "")
}

func transPtrInt32(v int32) *int32 {
	return &v
}
