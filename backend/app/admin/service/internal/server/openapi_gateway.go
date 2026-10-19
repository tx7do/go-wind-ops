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
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// OpenApiGateway OpenAPI 网关处理器。
type OpenApiGateway struct {
	apiKeyClient   opsV1.ApiKeyServiceClient
	buildClient    opsV1.BuildServiceClient
	distClient     opsV1.DistributionServiceClient
	pipelineClient opsV1.PipelineServiceClient
}

// NewOpenApiGateway 构造网关。
func NewOpenApiGateway(
	apiKeyClient opsV1.ApiKeyServiceClient,
	buildClient opsV1.BuildServiceClient,
	distClient opsV1.DistributionServiceClient,
	pipelineClient opsV1.PipelineServiceClient,
) *OpenApiGateway {
	return &OpenApiGateway{
		apiKeyClient:   apiKeyClient,
		buildClient:    buildClient,
		distClient:     distClient,
		pipelineClient: pipelineClient,
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
	if _, err := g.apiKeyClient.ValidateApiKey(r.Context(), &opsV1.ValidateApiKeyRequest{Key: &key}); err != nil {
		writeKratosError(w, err)
		return
	}

	// ── 路由 ──
	path := strings.TrimPrefix(r.URL.Path, "/openapi/v1/")
	parts := strings.Split(path, "/")
	// 期望 {kind}/{id}/trigger 或 webhooks/git/{buildId}
	if len(parts) == 3 && parts[2] == "trigger" {
		id, perr := strconv.ParseUint(parts[1], 10, 32)
		if perr != nil {
			writeJsonError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id")
			return
		}
		g.trigger(w, r, parts[0], uint32(id))
		return
	}
	if len(parts) == 3 && parts[0] == "webhooks" && parts[1] == "git" {
		id, perr := strconv.ParseUint(parts[2], 10, 32)
		if perr != nil {
			writeJsonError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid build id")
			return
		}
		g.gitWebhook(w, r, uint32(id))
		return
	}
	writeJsonError(w, http.StatusNotFound, "NOT_FOUND", "unknown endpoint")
}

// trigger 触发构建/分发/流水线。
func (g *OpenApiGateway) trigger(w http.ResponseWriter, r *http.Request, kind string, id uint32) {
	var (
		resp any
		err  error
	)
	switch kind {
	case "builds":
		resp, err = g.buildClient.TriggerBuild(r.Context(), &opsV1.TriggerBuildRequest{Id: id})
		if err == nil {
			writeJsonProto(w, http.StatusOK, resp.(*opsV1.TriggerBuildResponse))
		}
	case "distributions":
		resp, err = g.distClient.TriggerDistribution(r.Context(), &opsV1.TriggerDistributionRequest{Id: id})
		if err == nil {
			writeJsonProto(w, http.StatusOK, resp.(*opsV1.TriggerDistributionResponse))
		}
	case "pipelines":
		resp, err = g.pipelineClient.TriggerPipeline(r.Context(), &opsV1.TriggerPipelineRequest{Id: id})
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
func (g *OpenApiGateway) gitWebhook(w http.ResponseWriter, r *http.Request, buildId uint32) {
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

	resp, err := g.buildClient.TriggerBuild(r.Context(), &opsV1.TriggerBuildRequest{Id: buildId})
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
