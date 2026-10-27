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
	"go-wind-ops/app/core/service/internal/data/ent/configdistribution"
	"go-wind-ops/app/core/service/internal/data/ent/configtemplate"
	"go-wind-ops/app/core/service/internal/data/ent/inspection"
	"go-wind-ops/app/core/service/internal/data/ent/logsnapshot"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ── 巡检报告 ──

type InspectionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Inspection, ent.Inspection]

	repository *entCrud.Repository[
		ent.InspectionQuery, ent.InspectionSelect,
		ent.InspectionCreate, ent.InspectionCreateBulk,
		ent.InspectionUpdate, ent.InspectionUpdateOne,
		ent.InspectionDelete,
		predicate.Inspection,
		opsV1.Inspection, ent.Inspection,
	]
}

func NewInspectionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *InspectionRepo {
	repo := &InspectionRepo{
		log:       ctx.NewLoggerHelper("inspection/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Inspection, ent.Inspection](),
	}
	repo.init()
	return repo
}

func (r *InspectionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.InspectionQuery, ent.InspectionSelect,
		ent.InspectionCreate, ent.InspectionCreateBulk,
		ent.InspectionUpdate, ent.InspectionUpdateOne,
		ent.InspectionDelete,
		predicate.Inspection,
		opsV1.Inspection, ent.Inspection,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *InspectionRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListInspectionResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().Inspection.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListInspectionResponse{Total: 0}, nil
	}
	return &opsV1.ListInspectionResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *InspectionRepo) GetEntity(ctx context.Context, id uint32) (*ent.Inspection, error) {
	return r.entClient.Client().Inspection.
		Query().
		Where(inspection.IDEQ(id)).
		First(ctx)
}

// Create 建一行 RUNNING。
func (r *InspectionRepo) Create(ctx context.Context, operatorID uint32) (*ent.Inspection, error) {
	builder := r.entClient.Client().Inspection.Create().SetStatus(inspection.StatusRunning)
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create inspection: %s", err.Error())
		return nil, err
	}
	return row, nil
}

// Finish 终态化(幂等,仅 RUNNING 行生效)。
func (r *InspectionRepo) Finish(ctx context.Context, id uint32, status, summaryJSON string, nodeTotal, nodeOnline, projectTotal, projectRunning, alertOpen, certExpiring int, failErr string) error {
	now := time.Now()
	update := r.entClient.Client().Inspection.
		UpdateOneID(id).
		SetStatus(inspection.Status(status)).
		SetNodeTotal(nodeTotal).
		SetNodeOnline(nodeOnline).
		SetProjectTotal(projectTotal).
		SetProjectRunning(projectRunning).
		SetAlertOpen(alertOpen).
		SetCertExpiring(certExpiring).
		SetNillableFinishedAt(&now)
	if summaryJSON != "" {
		update.SetSummary(summaryJSON)
	}
	if failErr != "" {
		update.SetError(failErr)
	}
	_, err := update.Save(ctx)
	return err
}

// ── 日志快照 ──

type LogSnapshotRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.LogSnapshot, ent.LogSnapshot]

	repository *entCrud.Repository[
		ent.LogSnapshotQuery, ent.LogSnapshotSelect,
		ent.LogSnapshotCreate, ent.LogSnapshotCreateBulk,
		ent.LogSnapshotUpdate, ent.LogSnapshotUpdateOne,
		ent.LogSnapshotDelete,
		predicate.LogSnapshot,
		opsV1.LogSnapshot, ent.LogSnapshot,
	]
}

func NewLogSnapshotRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *LogSnapshotRepo {
	repo := &LogSnapshotRepo{
		log:       ctx.NewLoggerHelper("log-snapshot/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.LogSnapshot, ent.LogSnapshot](),
	}
	repo.init()
	return repo
}

func (r *LogSnapshotRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.LogSnapshotQuery, ent.LogSnapshotSelect,
		ent.LogSnapshotCreate, ent.LogSnapshotCreateBulk,
		ent.LogSnapshotUpdate, ent.LogSnapshotUpdateOne,
		ent.LogSnapshotDelete,
		predicate.LogSnapshot,
		opsV1.LogSnapshot, ent.LogSnapshot,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *LogSnapshotRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListLogSnapshotResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().LogSnapshot.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListLogSnapshotResponse{Total: 0}, nil
	}
	return &opsV1.ListLogSnapshotResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *LogSnapshotRepo) GetEntity(ctx context.Context, id uint32) (*ent.LogSnapshot, error) {
	return r.entClient.Client().LogSnapshot.
		Query().
		Where(logsnapshot.IDEQ(id)).
		First(ctx)
}

// SearchByKeyword 内容模糊检索(ILIKE),按时间倒序取最新 limit 份。
func (r *LogSnapshotRepo) SearchByKeyword(ctx context.Context, keyword string, projectID uint32, limit int) ([]*ent.LogSnapshot, error) {
	query := r.entClient.Client().LogSnapshot.
		Query().
		Where(logsnapshot.ContentContains(keyword))
	if projectID > 0 {
		query = query.Where(logsnapshot.ProjectIDEQ(projectID))
	}
	return query.
		Order(ent.Desc(logsnapshot.FieldID)).
		Limit(limit).
		All(ctx)
}

// Create 落一条快照(content 为 UTF-8 归一文本供检索;contentB64 为原始
// 字节 base64 供保真查看,空串不落)。
func (r *LogSnapshotRepo) Create(ctx context.Context, projectID, nodeID uint32, projectName, content, contentB64, trigger string, operatorID uint32) (*ent.LogSnapshot, error) {
	builder := r.entClient.Client().LogSnapshot.
		Create().
		SetProjectID(projectID).
		SetNodeID(nodeID).
		SetProjectName(projectName).
		SetContent(content).
		SetSizeBytes(int64(len(content))).
		SetTrigger(trigger)
	if contentB64 != "" {
		builder.SetContentB64(contentB64)
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create log snapshot: %s", err.Error())
		return nil, err
	}
	return row, nil
}

func (r *LogSnapshotRepo) DeleteEntity(ctx context.Context, id uint32) error {
	return r.entClient.Client().LogSnapshot.
		DeleteOneID(id).
		Exec(ctx)
}

// ── 配置文件中心 ──

type ConfigTemplateRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.ConfigTemplate, ent.ConfigTemplate]

	repository *entCrud.Repository[
		ent.ConfigTemplateQuery, ent.ConfigTemplateSelect,
		ent.ConfigTemplateCreate, ent.ConfigTemplateCreateBulk,
		ent.ConfigTemplateUpdate, ent.ConfigTemplateUpdateOne,
		ent.ConfigTemplateDelete,
		predicate.ConfigTemplate,
		opsV1.ConfigTemplate, ent.ConfigTemplate,
	]
}

func NewConfigTemplateRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ConfigTemplateRepo {
	repo := &ConfigTemplateRepo{
		log:       ctx.NewLoggerHelper("config-template/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.ConfigTemplate, ent.ConfigTemplate](),
	}
	repo.init()
	return repo
}

func (r *ConfigTemplateRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ConfigTemplateQuery, ent.ConfigTemplateSelect,
		ent.ConfigTemplateCreate, ent.ConfigTemplateCreateBulk,
		ent.ConfigTemplateUpdate, ent.ConfigTemplateUpdateOne,
		ent.ConfigTemplateDelete,
		predicate.ConfigTemplate,
		opsV1.ConfigTemplate, ent.ConfigTemplate,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *ConfigTemplateRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListConfigTemplateResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().ConfigTemplate.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListConfigTemplateResponse{Total: 0}, nil
	}
	return &opsV1.ListConfigTemplateResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *ConfigTemplateRepo) GetEntity(ctx context.Context, id uint32) (*ent.ConfigTemplate, error) {
	return r.entClient.Client().ConfigTemplate.
		Query().
		Where(configtemplate.IDEQ(id)).
		First(ctx)
}

func (r *ConfigTemplateRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().ConfigTemplate.Query().Where(configtemplate.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(configtemplate.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query config template taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *ConfigTemplateRepo) Create(ctx context.Context, req *opsV1.CreateConfigTemplateRequest, operatorID uint32) (*opsV1.ConfigTemplate, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().ConfigTemplate.Create().
		SetName(req.Data.GetName()).
		SetEnv(req.Data.GetEnv()).
		SetTargetPath(req.Data.GetTargetPath()).
		SetContent(req.Data.GetContent()).
		SetNillableDescription(req.Data.Description)
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create config template: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}
	return r.mapper.ToDTO(t), nil
}

func (r *ConfigTemplateRepo) Update(ctx context.Context, req *opsV1.UpdateConfigTemplateRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().ConfigTemplate.UpdateOneID(req.GetId())
	builder.Where(configtemplate.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.ConfigTemplate) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableEnv(req.Data.Env).
				SetNillableTargetPath(req.Data.TargetPath).
				SetNillableContent(req.Data.Content).
				SetNillableDescription(req.Data.Description)
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(configtemplate.FieldID, req.GetId()))
		},
	)
	return err
}

func (r *ConfigTemplateRepo) Delete(ctx context.Context, req *opsV1.DeleteConfigTemplateRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}
	delBuilder := r.entClient.Client().ConfigTemplate.Delete()
	delBuilder.Where(configtemplate.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("config template not found")
		}
		r.log.Errorf("delete config template [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

type ConfigDistributionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.ConfigDistribution, ent.ConfigDistribution]

	repository *entCrud.Repository[
		ent.ConfigDistributionQuery, ent.ConfigDistributionSelect,
		ent.ConfigDistributionCreate, ent.ConfigDistributionCreateBulk,
		ent.ConfigDistributionUpdate, ent.ConfigDistributionUpdateOne,
		ent.ConfigDistributionDelete,
		predicate.ConfigDistribution,
		opsV1.ConfigDistribution, ent.ConfigDistribution,
	]
}

func NewConfigDistributionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ConfigDistributionRepo {
	repo := &ConfigDistributionRepo{
		log:       ctx.NewLoggerHelper("config-distribution/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.ConfigDistribution, ent.ConfigDistribution](),
	}
	repo.init()
	return repo
}

func (r *ConfigDistributionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ConfigDistributionQuery, ent.ConfigDistributionSelect,
		ent.ConfigDistributionCreate, ent.ConfigDistributionCreateBulk,
		ent.ConfigDistributionUpdate, ent.ConfigDistributionUpdateOne,
		ent.ConfigDistributionDelete,
		predicate.ConfigDistribution,
		opsV1.ConfigDistribution, ent.ConfigDistribution,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// Create 建 PENDING 记录。
func (r *ConfigDistributionRepo) Create(ctx context.Context, templateID uint32, templateName string, nodeID, projectID uint32, targetPath, requestID string, operatorID uint32) (*ent.ConfigDistribution, error) {
	builder := r.entClient.Client().ConfigDistribution.
		Create().
		SetTemplateID(templateID).
		SetTemplateName(templateName).
		SetNodeID(nodeID).
		SetTargetPath(targetPath).
		SetCommandRequestID(requestID).
		SetStatus(configdistribution.StatusPending)
	if projectID > 0 {
		builder.SetProjectID(projectID)
	}
	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create config distribution: %s", err.Error())
		return nil, err
	}
	return row, nil
}

// FinalizeByCommandRequestID 按指令幂等键终态化(仅 PENDING 行生效)。
func (r *ConfigDistributionRepo) FinalizeByCommandRequestID(ctx context.Context, requestID, status, errMsg string) error {
	now := time.Now()
	_, err := r.entClient.Client().ConfigDistribution.
		Update().
		Where(
			configdistribution.CommandRequestIDEQ(requestID),
			configdistribution.StatusEQ(configdistribution.StatusPending),
		).
		SetStatus(configdistribution.Status(status)).
		SetNillableError(ptrOrNil(errMsg)).
		SetNillableFinishedAt(&now).
		Save(ctx)
	return err
}

// ListByTemplate 按模板取下发记录(时间倒序)。
func (r *ConfigDistributionRepo) ListByTemplate(ctx context.Context, templateID uint32, page, pageSize int) ([]*ent.ConfigDistribution, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	query := r.entClient.Client().ConfigDistribution.
		Query().
		Where(configdistribution.TemplateIDEQ(templateID))
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := query.
		Order(ent.Desc(configdistribution.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	return rows, total, nil
}

// CertExpiringSoonRow 证书到期巡检行(跨表快查)。
type CertExpiringSoonRow struct {
	Name    string
	Host    string
	Expires time.Time
}

// ListCertsExpiringWithin 取 30 天内到期的启用证书(巡检用)。
func (r *InspectionRepo) ListCertsExpiringWithin(ctx context.Context, certRepo *CertRepo, days int) []CertExpiringSoonRow {
	rows, err := certRepo.ListEnabled(ctx)
	if err != nil {
		return nil
	}
	deadline := time.Now().AddDate(0, 0, days)
	var out []CertExpiringSoonRow
	for _, row := range rows {
		if row.NotAfter == nil || row.NotAfter.After(deadline) {
			continue
		}
		out = append(out, CertExpiringSoonRow{Name: row.Name, Host: row.Host, Expires: *row.NotAfter})
	}
	return out
}
