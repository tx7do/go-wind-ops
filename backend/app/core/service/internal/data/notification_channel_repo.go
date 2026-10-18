package data

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/notificationchannel"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type NotificationChannelRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
	mapper    *mapper.CopierMapper[opsV1.NotificationChannel, ent.NotificationChannel]
	repository *entCrud.Repository[
		ent.NotificationChannelQuery, ent.NotificationChannelSelect,
		ent.NotificationChannelCreate, ent.NotificationChannelCreateBulk,
		ent.NotificationChannelUpdate, ent.NotificationChannelUpdateOne,
		ent.NotificationChannelDelete,
		predicate.NotificationChannel,
		opsV1.NotificationChannel, ent.NotificationChannel,
	]
}

func NewNotificationChannelRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *NotificationChannelRepo {
	repo := &NotificationChannelRepo{
		log:       ctx.NewLoggerHelper("notification-channel/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.NotificationChannel, ent.NotificationChannel](),
	}
	repo.init()
	return repo
}

func (r *NotificationChannelRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.NotificationChannelQuery, ent.NotificationChannelSelect,
		ent.NotificationChannelCreate, ent.NotificationChannelCreateBulk,
		ent.NotificationChannelUpdate, ent.NotificationChannelUpdateOne,
		ent.NotificationChannelDelete,
		predicate.NotificationChannel,
		opsV1.NotificationChannel, ent.NotificationChannel,
	](r.mapper)
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *NotificationChannelRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().NotificationChannel.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}
	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, opsV1.ErrorInternalServerError("query count failed")
	}
	return count, nil
}

func (r *NotificationChannelRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListNotificationChannelResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().NotificationChannel.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListNotificationChannelResponse{}, nil
	}
	return &opsV1.ListNotificationChannelResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *NotificationChannelRepo) Get(ctx context.Context, req *opsV1.GetNotificationChannelRequest) (*opsV1.NotificationChannel, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	dto, err := r.repository.Get(ctx, r.entClient.Client().NotificationChannel.Query(), nil, notificationchannel.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("notification channel not found")
		}
		return nil, opsV1.ErrorInternalServerError("query failed")
	}
	return dto, nil
}

func (r *NotificationChannelRepo) GetEntity(ctx context.Context, id uint32) (*ent.NotificationChannel, error) {
	return r.entClient.Client().NotificationChannel.Query().Where(notificationchannel.IDEQ(id)).First(ctx)
}

func (r *NotificationChannelRepo) Create(ctx context.Context, req *opsV1.CreateNotificationChannelRequest, operatorID uint32) (*opsV1.NotificationChannel, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().NotificationChannel.Create().
		SetName(req.Data.GetName()).
		SetType(notificationchannel.Type(req.Data.GetType().String())).
		SetEnabled(req.Data.GetEnabled())
	if req.Data.WebhookUrl != nil { builder.SetWebhookURL(req.Data.GetWebhookUrl()) }
	if req.Data.WebhookSecret != nil { builder.SetWebhookSecret(req.Data.GetWebhookSecret()) }
	if req.Data.WebhookSignStyle != nil { builder.SetWebhookSignStyle(notificationchannel.WebhookSignStyle(req.Data.GetWebhookSignStyle())) }
	if req.Data.SmtpHost != nil { builder.SetSMTPHost(req.Data.GetSmtpHost()) }
	if req.Data.SmtpPort != nil { builder.SetSMTPPort(int(req.Data.GetSmtpPort())) }
	if req.Data.SmtpUsername != nil { builder.SetSMTPUsername(req.Data.GetSmtpUsername()) }
	if req.Data.SmtpPassword != nil { builder.SetSMTPPassword(req.Data.GetSmtpPassword()) }
	if req.Data.SmtpFrom != nil { builder.SetSMTPFrom(req.Data.GetSmtpFrom()) }
	if req.Data.Description != nil { builder.SetDescription(req.Data.GetDescription()) }
	if operatorID > 0 { builder.SetCreatedBy(operatorID) }

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create notification channel: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}
	return r.mapper.ToDTO(t), nil
}

func (r *NotificationChannelRepo) Update(ctx context.Context, req *opsV1.UpdateNotificationChannelRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().NotificationChannel.UpdateOneID(req.GetId())
	builder.Where(notificationchannel.IDEQ(req.GetId()))
	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.NotificationChannel) {
			builder.SetNillableName(req.Data.Name).
				SetNillableWebhookURL(req.Data.WebhookUrl).
				SetNillableWebhookSecret(req.Data.WebhookSecret).
				SetNillableSMTPHost(req.Data.SmtpHost).
				SetNillableSMTPUsername(req.Data.SmtpUsername).
				SetNillableSMTPFrom(req.Data.SmtpFrom).
				SetNillableDescription(req.Data.Description).
				SetEnabled(req.Data.GetEnabled())
			if req.Data.WebhookSignStyle != nil {
				builder.SetWebhookSignStyle(notificationchannel.WebhookSignStyle(req.Data.GetWebhookSignStyle()))
			}
			if req.Data.SmtpPort != nil { builder.SetSMTPPort(int(req.Data.GetSmtpPort())) }
			if req.Data.SmtpPassword != nil && *req.Data.SmtpPassword != "" {
				builder.SetSMTPPassword(req.Data.GetSmtpPassword())
			}
		},
		func(s *sql.Selector) { s.Where(sql.EQ(notificationchannel.FieldID, req.GetId())) },
	)
	return err
}

func (r *NotificationChannelRepo) Delete(ctx context.Context, req *opsV1.DeleteNotificationChannelRequest) error {
	if req == nil { return opsV1.ErrorBadRequest("invalid parameter") }
	del := r.entClient.Client().NotificationChannel.Delete()
	del.Where(notificationchannel.IDEQ(req.GetId()))
	if _, err := del.Exec(ctx); err != nil {
		if ent.IsNotFound(err) { return opsV1.ErrorNotFound("not found") }
		return opsV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

// ListEnabled 取所有启用的渠道。
func (r *NotificationChannelRepo) ListEnabled(ctx context.Context) ([]*ent.NotificationChannel, error) {
	return r.entClient.Client().NotificationChannel.
		Query().Where(notificationchannel.EnabledEQ(true)).All(ctx)
}
