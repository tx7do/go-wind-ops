package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/cert"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type CertRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Cert, ent.Cert]

	repository *entCrud.Repository[
		ent.CertQuery, ent.CertSelect,
		ent.CertCreate, ent.CertCreateBulk,
		ent.CertUpdate, ent.CertUpdateOne,
		ent.CertDelete,
		predicate.Cert,
		opsV1.Cert, ent.Cert,
	]
}

func NewCertRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CertRepo {
	repo := &CertRepo{
		log:       ctx.NewLoggerHelper("cert/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Cert, ent.Cert](),
	}

	repo.init()

	return repo
}

func (r *CertRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.CertQuery, ent.CertSelect,
		ent.CertCreate, ent.CertCreateBulk,
		ent.CertUpdate, ent.CertUpdateOne,
		ent.CertDelete,
		predicate.Cert,
		opsV1.Cert, ent.Cert,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *CertRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCertResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Cert.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListCertResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListCertResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListEnabled 取启用的证书(调度探测/到期检查用)。
func (r *CertRepo) ListEnabled(ctx context.Context) ([]*ent.Cert, error) {
	return r.entClient.Client().Cert.
		Query().
		Where(cert.EnabledEQ(true)).
		All(ctx)
}

func (r *CertRepo) Get(ctx context.Context, req *opsV1.GetCertRequest) (*opsV1.Cert, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Cert.Query()

	dto, err := r.repository.Get(ctx, builder, nil, cert.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("cert not found")
		}
		r.log.Errorf("query cert [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(探测/检查用)。
func (r *CertRepo) GetEntity(ctx context.Context, id uint32) (*ent.Cert, error) {
	return r.entClient.Client().Cert.
		Query().
		Where(cert.IDEQ(id)).
		First(ctx)
}

// IsNameTaken 校验证书名唯一性。
func (r *CertRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Cert.Query().Where(cert.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(cert.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query cert name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *CertRepo) Create(ctx context.Context, req *opsV1.CreateCertRequest, operatorID uint32) (*opsV1.Cert, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Cert.Create().
		SetName(req.Data.GetName()).
		SetHost(req.Data.GetHost()).
		SetPort(int(req.Data.GetPort())).
		SetDaysBeforeExpire(int(req.Data.GetDaysBeforeExpire())).
		SetProbeCron(req.Data.GetProbeCron()).
		SetEnabled(req.Data.GetEnabled()).
		SetNillableDescription(req.Data.Description)

	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert cert failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *CertRepo) Update(ctx context.Context, req *opsV1.UpdateCertRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Cert.UpdateOneID(req.GetId())
	builder.Where(cert.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Cert) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableHost(req.Data.Host).
				SetNillableProbeCron(req.Data.ProbeCron).
				SetNillableDescription(req.Data.Description).
				SetEnabled(req.Data.GetEnabled())

			if req.Data.Port != nil && req.Data.GetPort() > 0 {
				builder.SetPort(int(req.Data.GetPort()))
			}
			if req.Data.DaysBeforeExpire != nil && req.Data.GetDaysBeforeExpire() > 0 {
				builder.SetDaysBeforeExpire(int(req.Data.GetDaysBeforeExpire()))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(cert.FieldID, req.GetId()))
		},
	)

	return err
}

// ApplyProbeResult 回填探测结果(成功清 last_error,失败清有效期字段)。
func (r *CertRepo) ApplyProbeResult(ctx context.Context, id uint32, issuer string, notBefore, notAfter *time.Time, probeErr string) error {
	now := time.Now()
	update := r.entClient.Client().Cert.
		UpdateOneID(id).
		SetLastProbedAt(now)
	if probeErr != "" {
		update.SetLastError(probeErr)
	} else {
		update.ClearLastError()
		update.SetNillableIssuer(&issuer)
		update.SetNillableNotBefore(notBefore)
		update.SetNillableNotAfter(notAfter)
	}
	_, err := update.Save(ctx)
	return err
}

func (r *CertRepo) Delete(ctx context.Context, req *opsV1.DeleteCertRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Cert.Delete()
	delBuilder.Where(cert.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("cert not found")
		}
		r.log.Errorf("delete cert [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}
