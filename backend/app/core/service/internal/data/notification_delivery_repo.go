package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/notificationdelivery"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type NotificationDeliveryRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.NotificationDelivery, ent.NotificationDelivery]

	repository *entCrud.Repository[
		ent.NotificationDeliveryQuery, ent.NotificationDeliverySelect,
		ent.NotificationDeliveryCreate, ent.NotificationDeliveryCreateBulk,
		ent.NotificationDeliveryUpdate, ent.NotificationDeliveryUpdateOne,
		ent.NotificationDeliveryDelete,
		predicate.NotificationDelivery,
		opsV1.NotificationDelivery, ent.NotificationDelivery,
	]
}

func NewNotificationDeliveryRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *NotificationDeliveryRepo {
	repo := &NotificationDeliveryRepo{
		log:       ctx.NewLoggerHelper("notification-delivery/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.NotificationDelivery, ent.NotificationDelivery](),
	}

	repo.init()

	return repo
}

func (r *NotificationDeliveryRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.NotificationDeliveryQuery, ent.NotificationDeliverySelect,
		ent.NotificationDeliveryCreate, ent.NotificationDeliveryCreateBulk,
		ent.NotificationDeliveryUpdate, ent.NotificationDeliveryUpdateOne,
		ent.NotificationDeliveryDelete,
		predicate.NotificationDelivery,
		opsV1.NotificationDelivery, ent.NotificationDelivery,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *NotificationDeliveryRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListNotificationDeliveryResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().NotificationDelivery.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListNotificationDeliveryResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListNotificationDeliveryResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// Create 落一条投递记录(status: SENT/FAILED)。
func (r *NotificationDeliveryRepo) Create(ctx context.Context, alertID, channelID uint32, channelType, target, status, lastError, title, content string) error {
	builder := r.entClient.Client().NotificationDelivery.
		Create().
		SetAlertID(alertID).
		SetChannelID(channelID).
		SetChannelType(channelType).
		SetStatus(notificationdelivery.Status(status)).
		SetTitle(title).
		SetContent(content)
	if target != "" {
		builder.SetTarget(target)
	}
	if lastError != "" {
		builder.SetLastError(lastError)
	}
	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("create notification delivery: %s", err.Error())
		return err
	}
	return nil
}

// Get 按 ID 取投递记录。
func (r *NotificationDeliveryRepo) Get(ctx context.Context, id uint32) (*opsV1.NotificationDelivery, error) {
	builder := r.entClient.Client().NotificationDelivery.Query()

	dto, err := r.repository.Get(ctx, builder, nil, notificationdelivery.IDEQ(id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("notification delivery not found")
		}
		r.log.Errorf("query notification delivery [%d]: %s", id, err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}
	return dto, nil
}
