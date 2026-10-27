// approval_gate.go 审批闸门:APPROVAL_REQUIRED_ACTIONS 配置的动作在派发前
// 先建 PENDING 审批单并拒绝直接执行,审批人(Approve,platform_admin 门)确认
// 后由审批服务重下发存储的载荷。默认空 = 不启用(行为同旧版)。
// 仅接入单目标动作(agent_upgrade/script_exec/plugin_exec)——批量/多目标
// 动作与"审批单重放到单一 agent"的模型不匹配:批量入口(ExecuteScriptBatch)
// 在 script_exec 需审批时直接 fail-closed 拒绝(approvalHoldsAction),
// 不得成为绕审旁路;其余多目标动作的防线仍是触发面权限码+危险命令确认。
package service

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/eventhook"
)

// ApprovalRequiredReason 审批闸门拦截错误专用 reason(前端可据此引导到审批页;
// message 携带审批单号)。契约字符串,改动需同步前端。
const ApprovalRequiredReason = "APPROVAL_REQUIRED"

var (
	approvalActionsOnce sync.Once
	approvalActions     map[string]struct{}
)

// parseApprovalActions 解析逗号分隔的动作清单(纯函数,可测)。
func parseApprovalActions(v string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, item := range strings.Split(v, ",") {
		if item = strings.ToLower(strings.TrimSpace(item)); item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

func approvalRequiredActions() map[string]struct{} {
	approvalActionsOnce.Do(func() {
		approvalActions = parseApprovalActions(os.Getenv("APPROVAL_REQUIRED_ACTIONS"))
	})
	return approvalActions
}

// approvalHoldsAction 查询某动作当前是否被审批闸门约束(纯查询,供批量等
// 无法套用"单目标审批单"模型的入口做 fail-closed 拒绝)。
func approvalHoldsAction(action string) bool {
	_, ok := approvalRequiredActions()[action]
	return ok
}

// approvalExpireHours 建单默认审批时长(APPROVAL_EXPIRE_HOURS,缺省 24;
// 0 或负 = 永不过期)。到期由 cron 扫描置 EXPIRED,须重新发起。
func approvalExpireHours() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("APPROVAL_EXPIRE_HOURS"))); err == nil {
		return v
	}
	return 24
}

// approvalGate 命中需审批清单时创建 PENDING 审批单并返回 403(专用 reason);
// 未启用/未命中返回 nil(照常下发)。建单即走通知渠道外呼(恢复类语义,
// 不受维护窗口静默影响——待办提醒必须可达)。
func approvalGate(
	ctx context.Context,
	log *log.Helper,
	repo *data.ApprovalRepo,
	notifChannelRepo *data.NotificationChannelRepo,
	deliveryRepo *data.NotificationDeliveryRepo,
	maintenanceRepo *data.MaintenanceWindowRepo,
	action string,
	refID uint32,
	agentID, opName, payloadJSON string,
	operatorID uint32,
) error {
	if _, ok := approvalRequiredActions()[action]; !ok {
		return nil
	}
	var expiresAt *time.Time
	if h := approvalExpireHours(); h > 0 {
		t := time.Now().Add(time.Duration(h) * time.Hour)
		expiresAt = &t
	}
	rec, err := repo.Create(ctx, action, refID, agentID, opName, payloadJSON, operatorID, expiresAt)
	if err != nil {
		return err
	}
	log.Warnf("approval gate: operation held for approval (id=%d action=%s agent=%s operator=%d expires_at=%s)",
		rec.ID, action, agentID, operatorID, expiresAt)
	eventhook.Emit("approval.created", map[string]any{
		"approvalId": rec.ID, "action": action, "agent": agentID,
		"operation": opName, "requester": operatorID, "expiresAt": expiresAt,
	})

	// 待办外呼:标题带单号与动作,便于审批人定位
	title := "审批待办 #" + strconv.Itoa(int(rec.ID)) + ": " + opName
	content := fmt.Sprintf("动作 %s 在节点 %s 上等待审批(单号 %d%s),经审批页通过后执行。",
		action, agentID, rec.ID, expiryHint(expiresAt))
	SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, maintenanceRepo, rec.ID, title, content, false)

	return errors.New(http.StatusForbidden, ApprovalRequiredReason,
		"approval #"+strconv.Itoa(int(rec.ID))+" required for "+action)
}

func expiryHint(t *time.Time) string {
	if t == nil {
		return ",永不过期"
	}
	return ",截止 "+t.Format("2006-01-02 15:04")
}

