// authz_engine.go 真 authz 引擎装配(casbin)+影子模式:
//
//	AUTHZ_MODE=noop     恒放行(旧行为,逃生开关)
//	AUTHZ_MODE=shadow   默认。真实评估"角色→API"策略,但仅记日志不拦截
//	AUTHZ_MODE=enforce  真实拦截
//
// 策略数据通道:角色列表 × 逐角色 ListPermissionResources([API]) × API 表
// (id→path/method),生成 casbin 规则 p, roleCode, path, method, *。
// 每 5 分钟重载;启动首次加载失败时策略为空(enforce 下即全拒,fail-closed)。
//
// 匹配口径:资源 = kratos PathTemplate(gorilla `{id}` 花括号格式,与 OpenAPI
// 文档一致);动作 = HTTP 方法。**OpenAPI 同步入库的 method 是小写,请求侧
// 是大写,provider 统一转大写**,否则全部误报。
package data

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	casbinEngine "github.com/tx7do/kratos-authz/engine/casbin"
	authzEngine "github.com/tx7do/kratos-authz/engine"

	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"

	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"

	"go-wind-ops/pkg/authorizer"
)

const (
	authzModeNoop    = "noop"
	authzModeShadow  = "shadow"
	authzModeEnforce = "enforce"

	authzReloadInterval = 5 * time.Minute
)

func authzModeFromEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AUTHZ_MODE"))) {
	case authzModeNoop:
		return authzModeNoop
	case authzModeEnforce:
		return authzModeEnforce
	default:
		return authzModeShadow
	}
}

// policyProvider 从 core 装配"角色→API"策略数据。
type policyProvider struct {
	roleClient permissionV1.RoleServiceClient
	permClient permissionV1.PermissionServiceClient
	apiClient  permissionV1.ApiServiceClient
}

// ProvidePolicies 生成 角色码 → [(path, method)] 策略集。
func (p *policyProvider) ProvidePolicies(ctx context.Context) (authorizer.PermissionDataMap, error) {
	noPaging := &paginationV1.PagingRequest{NoPaging: trans.Ptr(true)}
	roles, err := p.roleClient.List(ctx, noPaging)
	if err != nil {
		return nil, err
	}
	apis, err := p.apiClient.List(ctx, noPaging)
	if err != nil {
		return nil, err
	}

	// API id → (path, method 大写)
	apiByID := make(map[uint32]authorizer.PermissionData, len(apis.GetItems()))
	for _, api := range apis.GetItems() {
		if api.GetId() == 0 || api.GetPath() == "" {
			continue
		}
		apiByID[api.GetId()] = authorizer.PermissionData{
			Path:   api.GetPath(),
			Method: strings.ToUpper(api.GetMethod()),
		}
	}

	out := make(authorizer.PermissionDataMap, len(roles.GetItems()))
	for _, role := range roles.GetItems() {
		if role.GetId() == 0 || role.GetCode() == "" {
			continue
		}
		resources, err := p.permClient.ListPermissionResources(ctx, &permissionV1.ListPermissionResourcesRequest{
			RoleIds:       []uint32{role.GetId()},
			ResourceTypes: []permissionV1.ListPermissionResourcesRequest_ResourceType{permissionV1.ListPermissionResourcesRequest_API},
		})
		if err != nil {
			return nil, err
		}
		seen := make(map[authorizer.PermissionData]struct{})
		datas := make(authorizer.PermissionDataArray, 0, 8)
		for _, res := range resources.GetResources() {
			for _, apiID := range res.GetIds() {
				d, ok := apiByID[apiID]
				if !ok {
					continue
				}
				if _, dup := seen[d]; dup {
					continue
				}
				seen[d] = struct{}{}
				datas = append(datas, d)
			}
		}
		out[role.GetCode()] = datas
	}
	return out, nil
}

// casbinRulesOf PermissionDataMap → casbin p 规则(sub, obj, act, dom)。
func casbinRulesOf(data authorizer.PermissionDataMap) []casbinEngine.PolicyRule {
	var rules []casbinEngine.PolicyRule
	for roleCode, arr := range data {
		for _, api := range arr {
			rules = append(rules, casbinEngine.PolicyRule{
				PType: "p", V0: roleCode, V1: api.Path, V2: api.Method, V3: "*",
			})
		}
	}
	return rules
}

// ── 包装引擎 ──

// AuthzEngine 组合 casbin 真引擎与模式开关,实现 authzEngine.Engine。
type AuthzEngine struct {
	inner    *casbinEngine.State
	provider *policyProvider
	mode     string
	log      *log.Helper
}

var _ authzEngine.Engine = (*AuthzEngine)(nil)

// NewAuthzEngine 装配真引擎并启动周期重载(启动时同步首载)。
func NewAuthzEngine(
	ctx *bootstrap.Context,
	roleClient permissionV1.RoleServiceClient,
	permClient permissionV1.PermissionServiceClient,
	apiClient permissionV1.ApiServiceClient,
) authzEngine.Engine {
	mode := authzModeFromEnv()
	if mode == authzModeNoop {
		return noopEngine{}
	}

	e := &AuthzEngine{
		inner:    mustCasbin(ctx),
		provider: &policyProvider{roleClient: roleClient, permClient: permClient, apiClient: apiClient},
		mode:     mode,
		log:      ctx.NewLoggerHelper("authz-engine/data/admin-service"),
	}
	if e.inner == nil {
		e.log.Errorf("casbin engine init failed, fall back to noop (AUTHZ_MODE=%s)", mode)
		return noopEngine{}
	}

	// 首载(失败即空策略:shadow 只刷日志,enforce 全拒 fail-closed)
	if err := e.reload(context.Background()); err != nil {
		e.log.Errorf("authz policy initial load failed: %v", err)
	}

	// 周期重载
	go func() {
		ticker := time.NewTicker(authzReloadInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Context().Done():
				return
			case <-ticker.C:
				if err := e.reload(context.Background()); err != nil {
					e.log.Errorf("authz policy reload failed: %v", err)
				}
			}
		}
	}()

	e.log.Infof("authz engine started (mode=%s)", e.mode)
	return e
}

func mustCasbin(ctx *bootstrap.Context) *casbinEngine.State {
	inner, err := casbinEngine.NewEngine(ctx.Context())
	if err != nil {
		return nil
	}
	return inner
}

// reload 拉取策略并热更新。
func (e *AuthzEngine) reload(ctx context.Context) error {
	data, err := e.provider.ProvidePolicies(ctx)
	if err != nil {
		return err
	}
	if err = e.inner.SetPolicies(ctx, authzEngine.PolicyMap{"policies": casbinRulesOf(data)}, nil); err != nil {
		return err
	}
	e.log.Infof("authz policies reloaded (%d roles)", len(data))
	return nil
}

// IsAuthorized 真实评估;shadow 模式下拒绝仅记日志并放行。
func (e *AuthzEngine) IsAuthorized(ctx context.Context, subject authzEngine.Subject, action authzEngine.Action, resource authzEngine.Resource, project authzEngine.Project) (bool, error) {
	if e.mode == authzModeNoop {
		return true, nil
	}
	allowed, err := e.inner.IsAuthorized(ctx, subject, action, resource, project)
	if err == nil && allowed {
		return true, nil
	}
	if e.mode == authzModeShadow {
		e.log.Warnf("[authz-shadow] would DENY: role=%s %s %s (err=%v) — grant it via role→API authorization or switch AUTHZ_MODE",
			subject, action, resource, err)
		return true, nil
	}
	return allowed, err
}

func (e *AuthzEngine) Name() string { return "casbin-authz(" + e.mode + ")" }

func (e *AuthzEngine) ProjectsAuthorized(ctx context.Context, subjects authzEngine.Subjects, action authzEngine.Action, resource authzEngine.Resource, projects authzEngine.Projects) (authzEngine.Projects, error) {
	return e.inner.ProjectsAuthorized(ctx, subjects, action, resource, projects)
}

func (e *AuthzEngine) FilterAuthorizedPairs(ctx context.Context, subjects authzEngine.Subjects, pairs authzEngine.Pairs) (authzEngine.Pairs, error) {
	return e.inner.FilterAuthorizedPairs(ctx, subjects, pairs)
}

func (e *AuthzEngine) FilterAuthorizedProjects(ctx context.Context, subjects authzEngine.Subjects) (authzEngine.Projects, error) {
	return e.inner.FilterAuthorizedProjects(ctx, subjects)
}

func (e *AuthzEngine) SetPolicies(ctx context.Context, policies authzEngine.PolicyMap, roles authzEngine.RoleMap) error {
	return e.inner.SetPolicies(ctx, policies, roles)
}

// noopEngine 恒放行(AUTHZ_MODE=noop 或引擎装配失败的兜底)。
type noopEngine struct{ authzEngine.Engine }

func (noopEngine) Name() string { return "noop" }
func (noopEngine) IsAuthorized(context.Context, authzEngine.Subject, authzEngine.Action, authzEngine.Resource, authzEngine.Project) (bool, error) {
	return true, nil
}
func (noopEngine) ProjectsAuthorized(context.Context, authzEngine.Subjects, authzEngine.Action, authzEngine.Resource, authzEngine.Projects) (authzEngine.Projects, error) {
	return authzEngine.Projects{}, nil
}
func (noopEngine) FilterAuthorizedPairs(context.Context, authzEngine.Subjects, authzEngine.Pairs) (authzEngine.Pairs, error) {
	return authzEngine.Pairs{}, nil
}
func (noopEngine) FilterAuthorizedProjects(context.Context, authzEngine.Subjects) (authzEngine.Projects, error) {
	return authzEngine.Projects{}, nil
}
func (noopEngine) SetPolicies(context.Context, authzEngine.PolicyMap, authzEngine.RoleMap) error {
	return nil
}
