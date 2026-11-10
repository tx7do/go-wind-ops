package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/scriptversion"
)

// ScriptVersionRepo 脚本内容版本历史仓储。
type ScriptVersionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewScriptVersionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ScriptVersionRepo {
	return &ScriptVersionRepo{
		log:       ctx.NewLoggerHelper("script-version/repo/core-service"),
		entClient: entClient,
	}
}

// Snapshot 落一条内容快照。
func (r *ScriptVersionRepo) Snapshot(ctx context.Context, scriptID int64, content, remark string, operatorID int64) error {
	builder := r.entClient.Client().ScriptVersion.Create().
		SetScriptID(scriptID).
		SetContent(content).
		SetRemark(remark)
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	if _, err := builder.Save(ctx); err != nil {
		r.log.Errorf("snapshot script version (script=%d): %s", scriptID, err.Error())
		return err
	}
	return nil
}

// ListByScript 版本列表(新→旧;内容全量返回,历史页直接查看)。
func (r *ScriptVersionRepo) ListByScript(ctx context.Context, scriptID int64) ([]*ent.ScriptVersion, error) {
	return r.entClient.Client().ScriptVersion.Query().
		Where(scriptversion.ScriptIDEQ(scriptID)).
		Order(ent.Desc(scriptversion.FieldID)).
		All(ctx)
}

// Get 取单条快照(恢复时取目标内容)。
func (r *ScriptVersionRepo) Get(ctx context.Context, id int64) (*ent.ScriptVersion, error) {
	return r.entClient.Client().ScriptVersion.Get(ctx, id)
}
