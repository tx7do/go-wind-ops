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

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-ops/app/core/service/internal/data"
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

// approvalGate 命中需审批清单时创建 PENDING 审批单并返回 403(专用 reason);
// 未启用/未命中返回 nil(照常下发)。
func approvalGate(
	ctx context.Context,
	log *log.Helper,
	repo *data.ApprovalRepo,
	action string,
	refID uint32,
	agentID, opName, payloadJSON string,
	operatorID uint32,
) error {
	if _, ok := approvalRequiredActions()[action]; !ok {
		return nil
	}
	rec, err := repo.Create(ctx, action, refID, agentID, opName, payloadJSON, operatorID)
	if err != nil {
		return err
	}
	log.Warnf("approval gate: operation held for approval (id=%d action=%s agent=%s operator=%d)",
		rec.ID, action, agentID, operatorID)
	return errors.New(http.StatusForbidden, ApprovalRequiredReason,
		"approval #"+strconv.Itoa(int(rec.ID))+" required for "+action)
}

