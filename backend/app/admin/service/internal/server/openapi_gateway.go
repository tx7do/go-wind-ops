// openapi_gateway.go OpenAPI 网关:外部系统(CI/CD)凭 API Key 触发
// 构建/分发/流水线,以及 Git push webhook 自动构建。
//
// 直注册 mux(绕过 JWT 中间件),认证由本 handler 完成并转发 core 校验:
//   - 头 X-Api-Key: opsk_... 或 Authorization: Bearer opsk_...
//   - core 校验哈希/状态/过期并异步回写最近使用
//
// 端点:
//
//	POST /openapi/v1/builds/{id}/trigger           触发构建(异步)
//	POST /openapi/v1/distributions/{id}/trigger    触发分发(异步)
//	POST /openapi/v1/pipelines/{id}/trigger        触发流水线(异步)
//	POST /openapi/v1/webhooks/git/{buildId}        Git push webhook
//	     (body 取 ref/refs 字段,query branch 提供时按分支过滤)
package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"os"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"

	appMetadata "go-wind-ops/pkg/metadata"
)

// OpenApiGateway OpenAPI 网关处理器。
type OpenApiGateway struct {
	apiKeyClient   opsV1.ApiKeyServiceClient
	buildClient    opsV1.BuildServiceClient
	distClient     opsV1.DistributionServiceClient
	pipelineClient opsV1.PipelineServiceClient
	promotionClient opsV1.PromotionPipelineServiceClient
	rateLimiter    *apiKeyRateLimiter
}

// NewOpenApiGateway 构造网关。
func NewOpenApiGateway(
	apiKeyClient opsV1.ApiKeyServiceClient,
	buildClient opsV1.BuildServiceClient,
	distClient opsV1.DistributionServiceClient,
	pipelineClient opsV1.PipelineServiceClient,
	promotionClient opsV1.PromotionPipelineServiceClient,
) *OpenApiGateway {
	return &OpenApiGateway{
		apiKeyClient:    apiKeyClient,
		buildClient:     buildClient,
		distClient:      distClient,
		pipelineClient:  pipelineClient,
		promotionClient: promotionClient,
		rateLimiter:     newAPIKeyRateLimiter(),
	}
}

// Prefix 注册前缀(rest_server.go HandlePrefix 用)。
func (g *OpenApiGateway) Prefix() string { return "/openapi/" }

func (g *OpenApiGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJsonError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}

	// ── API Key 认证 ──
	key := r.Header.Get("X-Api-Key")
	if key == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if key == "" || g.apiKeyClient == nil {
		writeJsonError(w, http.StatusUnauthorized, "API_KEY_REQUIRED", "missing X-Api-Key header")
		return
	}
	// 按 key 限流(OPENAPI_RATE_LIMIT 次/分钟,默认 60):防有效 key 无节制刷触发
	if !g.rateLimiter.Allow(key) {
		writeJsonError(w, http.StatusTooManyRequests, "RATE_LIMITED", "api key rate limit exceeded")
		return
	}
	info, verr := g.apiKeyClient.ValidateApiKey(r.Context(), &opsV1.ValidateApiKeyRequest{Key: &key})
	if verr != nil {
		writeKratosError(w, verr)
		return
	}

	// ── 路由(先解析出目标端点类别,作用域闸门按类别放行) ──
	path := strings.TrimPrefix(r.URL.Path, "/openapi/v1/")
	parts := strings.Split(path, "/")
	// 期望 {kind}/{id}/trigger 或 webhooks/git/{buildId}
	var requiredScope opsV1.ApiKey_Scope
	switch {
	case len(parts) == 3 && parts[2] == "trigger":
		switch parts[0] {
		case "builds":
			requiredScope = opsV1.ApiKey_BUILDS
		case "distributions":
			requiredScope = opsV1.ApiKey_DISTRIBUTIONS
		case "pipelines":
			requiredScope = opsV1.ApiKey_PIPELINES
		case "promotions":
			requiredScope = opsV1.ApiKey_PROMOTIONS
		default:
			writeJsonError(w, http.StatusNotFound, "NOT_FOUND", "unknown kind: "+parts[0])
			return
		}
	case len(parts) == 3 && parts[0] == "webhooks" && parts[1] == "git":
		requiredScope = opsV1.ApiKey_WEBHOOKS
	default:
		writeJsonError(w, http.StatusNotFound, "NOT_FOUND", "unknown endpoint")
		return
	}

	// 作用域闸门:密钥未限 scope(空)= 全部;含 ALL 通配 = 全部;
	// 否则必须包含目标端点类别。密钥泄露时把爆炸半径收到签发时勾选的范围。
	if !scopeAllowed(info.GetScopes(), requiredScope) {
		writeJsonError(w, http.StatusForbidden, "SCOPE_DENIED",
			"api key scope does not allow this endpoint")
		return
	}

	// 归因注入:触发的构建/分发/流水线归属密钥创建者的租户(而非平台)。
	// 无归属(历史数据 tenant 0)时保持平台视图,行为不变。
	gctx := r.Context()
	if info.GetTenantId() > 0 {
		gctx, _ = appMetadata.NewContext(gctx, appMetadata.NewUserOperator(
			uint64(info.GetOwnerUserId()), uint64(info.GetTenantId()), 0, identityV1.DataScope_ALL))
	}

	if len(parts) == 3 && parts[2] == "trigger" {
		id, perr := strconv.ParseUint(parts[1], 10, 32)
		if perr != nil {
			writeJsonError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id")
			return
		}
		g.trigger(w, gctx, r, parts[0], uint32(id))
		return
	}
	// 走到此处必为 webhooks/git/{buildId}(requiredScope 解析已排除其余形态)
	id, perr := strconv.ParseUint(parts[2], 10, 32)
	if perr != nil {
		writeJsonError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid build id")
		return
	}
	g.gitWebhook(w, r, gctx, uint32(id))
}

// scopeAllowed 作用域判定:未限定(空)或含通配 ALL 放行,否则须命中目标类别。
func scopeAllowed(scopes []opsV1.ApiKey_Scope, required opsV1.ApiKey_Scope) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, sc := range scopes {
		if sc == opsV1.ApiKey_ALL || sc == required {
			return true
		}
	}
	return false
}

// trigger 触发构建/分发/流水线。
func (g *OpenApiGateway) trigger(w http.ResponseWriter, ctx context.Context, r *http.Request, kind string, id uint32) {
	var (
		resp any
		err  error
	)
	switch kind {
	case "builds":
		resp, err = g.buildClient.TriggerBuild(ctx, &opsV1.TriggerBuildRequest{Id: id})
		if err == nil {
			writeJsonProto(w, http.StatusOK, resp.(*opsV1.TriggerBuildResponse))
		}
	case "distributions":
		resp, err = g.distClient.TriggerDistribution(ctx, &opsV1.TriggerDistributionRequest{Id: id})
		if err == nil {
			writeJsonProto(w, http.StatusOK, resp.(*opsV1.TriggerDistributionResponse))
		}
	case "pipelines":
		resp, err = g.pipelineClient.TriggerPipeline(ctx, &opsV1.TriggerPipelineRequest{Id: id})
		if err == nil {
			writeJsonProto(w, http.StatusOK, resp.(*opsV1.TriggerPipelineResponse))
		}
	default:
		writeJsonError(w, http.StatusNotFound, "NOT_FOUND", "unknown kind: "+kind)
		return
	}
	if err != nil {
		writeKratosError(w, err)
	}
}

// gitWebhook Git push webhook:解析 ref 字段(GitHub/GitLab/Gitea 通用格式
// refs/heads/<branch>),query branch 提供时按分支过滤,命中即触发构建。
// 必须配置 OPENAPI_GIT_WEBHOOK_SECRET(fail-closed):校验 X-Signature:
// sha256=<HMAC-SHA256(body)>(GitHub/Gitea 兼容的签名格式),防止 webhook
// 端点被无密钥方刷触发;未配置时一律 403 拒绝。
func (g *OpenApiGateway) gitWebhook(w http.ResponseWriter, r *http.Request, ctx context.Context, buildId uint32) {
	secret := os.Getenv("OPENAPI_GIT_WEBHOOK_SECRET")
	if secret == "" {
		writeJsonError(w, http.StatusForbidden, "WEBHOOK_SECRET_REQUIRED",
			"OPENAPI_GIT_WEBHOOK_SECRET is not configured; git webhook is disabled")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJsonError(w, http.StatusBadRequest, "BAD_REQUEST", "read body failed")
		return
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expect := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expect), []byte(r.Header.Get("X-Signature"))) != 1 {
		writeJsonError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "webhook signature mismatch")
		return
	}

	// 分支过滤(可选)
	filterBranch := r.URL.Query().Get("branch")
	if filterBranch != "" {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJsonError(w, http.StatusBadRequest, "BAD_REQUEST", "read body failed")
			return
		}
		var payload struct {
			Ref string `json:"ref"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Ref != "" {
			pushed := strings.TrimPrefix(payload.Ref, "refs/heads/")
			if pushed != filterBranch {
				// 非目标分支:静默成功(不触发)
				writeJsonOK(w, map[string]any{"triggered": false, "branch": pushed})
				return
			}
		}
	}

	resp, err := g.buildClient.TriggerBuild(ctx, &opsV1.TriggerBuildRequest{Id: buildId})
	if err != nil {
		writeKratosError(w, err)
		return
	}
	writeJsonOK(w, map[string]any{"triggered": true, "buildNo": resp.GetBuildNo(), "historyId": resp.GetHistoryId()})
}

// ── 响应工具 ──

func writeJsonProto(w http.ResponseWriter, status int, v proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if b, err := protojson.Marshal(v); err == nil {
		_, _ = w.Write(b)
	}
}

func writeJsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJsonError(w http.ResponseWriter, status int, reason, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    status,
		"reason":  reason,
		"message": message,
	})
}

func writeKratosError(w http.ResponseWriter, err error) {
	if ke := errors.FromError(err); ke != nil {
		status := int(ke.Code)
		if status < 100 || status > 599 {
			status = http.StatusInternalServerError
		}
		writeJsonError(w, status, ke.Reason, ke.Message)
		return
	}
	writeJsonError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
}
