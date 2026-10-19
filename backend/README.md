# GoWind Ops Backend

GoWind Ops 后端：Go 1.26 + Kratos + Ent 的四服务单体仓库。平台总览见根 [README](../README.md)；开发规范与架构细节的**单一事实源**在 [AGENTS.md](./AGENTS.md)。

## 服务组成

| 服务 | 端口 | 职责 |
|:---|:---|:---|
| core-service | gRPC 随机端口（etcd 发现名 `gowind-ops/core-service`） | 领域资源层：Ent schema/仓储/业务逻辑 + MQTT 控制端 + Asynq worker，持有全部数据 |
| admin-service | REST **:18600** | 管理后台 BFF：REST → gRPC 调 core，**不连数据库**；另挂 WS 与 `/openapi/` 端点 |
| front-service | REST **:18700** | 前台 BFF：结构同 admin，面向 C 端 |
| agent-service | 无入站端口 | 边缘代理：纯 MQTT 出站接入 EMQX，准入/心跳/执行指令，落盘执行类指令过 `AGENT_ALLOWED_DIRS` 校验 |

管理前端（React 19 + antd 6，dev :18999，代理 `/admin` → 127.0.0.1:18600）在 [frontend/admin](../frontend/admin)。

## 目录结构

```
backend/
├── api/
│   ├── protos/                 # .proto 源文件（ops 业务域 + 基座继承域 + admin REST 包装 i_*.proto）
│   └── gen/go/                 # 生成代码（HTTP/gRPC/校验），不手改
├── app/
│   ├── admin/service/          # 管理后台 BFF
│   ├── front/service/          # 前台 BFF
│   ├── core/service/           # 领域资源层（internal/data/ent/schema 是全部表定义）
│   └── agent/service/          # 边缘代理（internal/agent 内按 action 注册指令处理器）
├── pkg/                        # 公共包（signal/topic/serviceid/oss/filetransfer/shellexec/鉴权中间件...）
├── tools/                      # 开发调试工具（pass-enc 密码加密 / mqtt-pub 指令注入）
└── scripts/                    # 部署脚本（Docker、pm2 等）
```

## 管理端 REST 接口速览

前缀均为 `/admin/v1`（JWT 鉴权，登录除外）。除注明"触发类"的特殊动作路由外，各资源均为标准 CRUD（GET 列表/详情、POST 创建、PUT 更新、DELETE 删除）。完整请求/响应结构以 `api/protos/admin/service/v1/i_*.proto` 为准。

### 认证与门户

| 端点 | 说明 |
|:---|:---|
| `POST /login` | 登录（`password` 传 AES-GCM 密文；验证码经 `X-Captcha-Id` / `X-Captcha-Value` 头校验） |
| `POST /refresh-token` · `POST /logout` | 刷新 / 注销 |
| `GET /captcha` · `POST /captcha/verify` | 图形验证码签发 / 校验（`ADMIN_CAPTCHA_DISABLED=true` 可跳过） |
| `GET /oidc/authorize-url` | OIDC 授权跳转地址 |
| `GET /routes` · `GET /perm-codes` · `GET /initial-context` | 前端路由、权限码、初始化上下文 |

### 节点与项目（ops 核心）

| 端点 | 说明 |
|:---|:---|
| `/agent-nodes` | 节点 CRUD（记录由 agent 准入自动创建）；`GET /{id}/metrics` 指标趋势 |
| `POST /agent-nodes/{id}/commands:send` | 下发指令（ping） |
| `POST /agent-nodes/upgrade-upload-url:resolve` · `POST /agent-nodes/upgrade:send` | Agent 远程升级（预签名直传 + 下发自替换指令） |
| `/projects` | 项目 CRUD；`POST /{id}/operations:send` 生命周期操作（START/STOP/RESTART/STATUS） |
| `GET|DELETE /projects/{id}/log` | 运行日志尾部查看 / 清空 |
| `/command-records` | 指令台账（只读审计） |

### 终端与文件

| 端点 | 说明 |
|:---|:---|
| `GET /ws/terminal?ticket=&agentId=` | **WebSocket** 终端（浏览器 WS ↔ BFF ↔ core gRPC 双向流 ↔ MQTT ↔ agent PTY；30 秒一次性票据握手——先 `POST /ws/terminal/tickets` 换票（Bearer token + `ops:node_manage` 权限码 + agentId 绑定），长效 token 不进 URL；操作者身份注入流元数据供审计归因） |
| `/terminal-sessions` · `/terminal-sessions/chunks:search` | 终端会话审计（会话列表 / 分块检索，只读回放；要求 `sys:audit_logs` 权限码） |
| `POST /file/list` · `/file/read` · `/file/write` · `/file/delete` | 文件管理（agent 授权目录白名单内；read 有 1MB 上限，大文件走 pull） |
| `POST /file/pull` · `/file/push-url` · `/file/push` | 大文件三步链：预签名 PUT 中转 → 逐机拉取落盘 |

### 脚本与批量执行

| 端点 | 说明 |
|:---|:---|
| `/scripts` | 脚本 CRUD（内容 ≤256KB）；`POST /{id}/executions:send` 远程执行 |
| `POST /scripts/batch-exec:send` | 批量执行：`script_id + node_ids`（脚本档案）或 `command + agent_ids`（即席命令）二选一 |
| `/script-batches` | 批次详情（逐节点结果矩阵：退出码/时长/输出） |

### 构建与分发

| 端点 | 说明 |
|:---|:---|
| `/repositories` | 源仓库 CRUD（凭据 AES-GCM 加密） |
| `/builds` | 构建配置 CRUD；`POST /{id}/builds:trigger` 触发构建 |
| `/build-histories` | 构建历史；`GET /{id}/artifact-url` 产物预签名下载；`GET /{id}/log[/{offset}]` 日志增量 tail |
| `/distributions` | 分发配置 CRUD（FILE / DOCKER 类型）；`POST /{id}/distributions:trigger` 触发分发 |
| `/distribution-records` | 分发记录（状态机跟踪） |
| `/pipelines` · `POST /pipelines/{id}/trigger` | 流水线（BUILD/SCRIPT/DISTRIBUTION 步骤编排） |
| `/pipeline-runs` · `GET /{id}/log` | 流水线运行记录与日志 |
| `/config-templates` · `POST /config-templates/distribute:send` · `GET /config-templates/distributions:search` | 配置中心：模板 CRUD、占位符渲染下发、下发记录检索 |

### 可观测与治理

| 端点 | 说明 |
|:---|:---|
| `/monitors` | 监控规则（项目存活 / 节点指标阈值） |
| `/alerts` · `POST /alerts/{id}:resolve` | 告警记录 / 手动恢复 |
| `/notification-channels` · `/notification-deliveries` | 通知渠道（webhook/钉钉/飞书/企微/邮件）、逐渠道投递台账 |
| `/backup-tasks` · `POST /backup-tasks/trigger:send` | 备份任务（cron + 手动） |
| `/backup-records` · `GET /{id}/download-url` | 备份记录与下载 |
| `/inspections` · `POST /inspections/run:send` | 巡检报告（聚合快照） |
| `/log-snapshots` · `POST /log-snapshots/capture:send` · `POST /log-snapshots/search:send` | 日志快照采集 / 关键字检索 |
| `/certs` · `POST /certs/{id}/probe:send` | 证书台账 / 立即探测 |
| `/approvals` · `POST /{id}:approve` · `POST /{id}:reject` | 操作审批 |
| `GET /ws/projects/log?token=&projectId=` | **WebSocket** 项目日志实时 tail（可选 `interval` 1000-10000ms） |

### 云管理

| 端点 | 说明 |
|:---|:---|
| `/cloud-accounts` · `POST /{id}/verify` | 多云账号（AK/SK 加密）与在线校验 |
| `/cloud-instances` · `POST /cloud-instances/sync` | 资源盘点（全地域实例同步） |
| `/cloud-costs` · `POST /cloud-costs/import` · `GET /cloud-costs/summary` · `POST /cloud-costs/fetch` | 成本台账：录入 / CSV 导入 / 汇总 / 厂商账单拉取 |
| `/cloud-budgets` · `POST /cloud-budgets/check:send` | 预算与突增检测 |

### 平台化

| 端点 | 说明 |
|:---|:---|
| `/api-keys` | API Key 管理（明文仅创建时返回一次） |
| `/agent-plugins` · `POST /agent-plugins/exec` | 插件指令清单与执行 |

### OpenAPI（免 JWT，`X-Api-Key` 或 `Authorization: Bearer`）

| 端点 | 说明 |
|:---|:---|
| `POST /openapi/v1/builds/{id}/trigger` | 触发构建 |
| `POST /openapi/v1/distributions/{id}/trigger` | 触发分发 |
| `POST /openapi/v1/pipelines/{id}/trigger` | 触发流水线 |
| `POST /openapi/v1/webhooks/git/{buildId}?branch=` | Git webhook（GitHub/GitLab/Gitea 通用 ref 解析，按分支过滤） |

### 基座继承（系统与运维）

`/users` · `/roles` · `/menus` · `/org-units` · `/tenants` · `/positions` · `/permissions`（+ `sync:perms`） · `/permission-groups` · `/login-policies` · `/apis`（接口权限，+ `sync`/`walk-route`） · `/dict/types` · `/dict/entries` · `/dict/langs`，以及审计四件套 `/login-audit-logs` · `/operation-audit-logs` · `/api-audit-logs` · `/data-access-audit-logs`（+ `/permission-audit-logs` · `/policy-evaluation-logs`）。

## MQTT 信令与 agent 指令

主题常量一律从 `pkg/topic` 引用、载荷结构从 `pkg/signal` 引用，禁止裸拼主题字符串。核心主题：

| 主题 | 方向 | 用途 |
|:---|:---|:---|
| `ops/agent/{agentId}/auth/{request,response}` | 双向 | 准入（TOFU + SHA-256 token 哈希） |
| `ops/agent/{agentId}/heartbeat` | agent→core | 30s 心跳（含 gopsutil 指标） |
| `ops/agent/{agentId}/command` | core→agent | 指令下发（`request_id` UUID 幂等键） |
| `ops/agent/{agentId}/command/result` | agent→core | 指令回执（台账对账） |
| `ops/agent/{agentId}/file/{request,response,complete}` | 双向 | 大文件中转信令（内容走预签名 URL） |
| `ops/agent/{agentId}/terminal/{sessionId}` | 双向 | 终端字节流 |

指令 action（小写下划线，agent 侧 `RegisterHandler` 注册）：`ping`、`project_start/stop/restart/status`、`script_exec`（含批量/即席）、`project_log`、`file_list/read/write/delete/pull/push`、`terminal_open`、`backup_run`、`dist_apply`、`docker_deploy`、`agent_upgrade`、`plugin_list`、`plugin_exec` 等。约定细节见 [AGENTS.md](./AGENTS.md) 的「MQTT 信令规范」。

## 快速开始

```bash
# 1. 依赖服务（PG wind_ops / Redis / MinIO / etcd / EMQX / Jaeger）
docker compose -f docker-compose.libs.yaml up -d

# 2. 编译并启动（顺序：core → agent → admin/front；先清 HTTP_PROXY 再起，见下）
make build
env -u HTTP_PROXY -u HTTPS_PROXY ./bin/core-server -c app/core/service/configs
AGENT_ALLOWED_DIRS='D:\ops' ./bin/agent-server -c app/agent/service/configs
env -u HTTP_PROXY -u HTTPS_PROXY ./bin/admin-server -c app/admin/service/configs

# 3. 管理前端
cd ../frontend/admin && pnpm install && pnpm dev   # http://localhost:18999
```

- 登录 `admin / admin`，密码需 AES-GCM 加密（`go run ./tools/pass-enc admin`），开发可设 `ADMIN_CAPTCHA_DISABLED=true` 跳过验证码。
- agent 首启生成 `auth.token` 经 MQTT 注册（TOFU），日志出现 `enrolled/authenticated` 即接入；项目目录必须在 `AGENT_ALLOWED_DIRS` 内。
- 冒烟顺序与本地验证要点（清代理、重建 exe、按 PID 杀进程等）见 [AGENTS.md](./AGENTS.md) 与根 README。

## 代码生成与门禁

```bash
make ent          # 改 ent/schema 后生成
make api          # buf 生成 protobuf Go 代码
make ts           # 生成管理前端 TS 客户端（frontend/admin/src/api/generated）
make gen          # ent + api + openapi 一键生成
make register ENTITY=xxx   # 新 CRUD 模块装配注入（wiring.go 锚点行，幂等）
make lint && make vet && make test   # 提交前门禁，必须全绿
```

新增 gRPC 方法时，**admin/front 门面的嵌入接口必须同步补转发**（漏了编译不报错、调到才 500）；新增触发类 POST 路由 id 走 body、不带 path 参数（kratos oneof 双写 400）。

## 开发调试工具

```bash
go run ./tools/pass-enc admin   # 登录密码加密（AES-GCM + base64）
go run ./tools/mqtt-pub -username core -password core-pass -topic ops/agent/<agentId>/command \
  -payload '{"request_id":"t1","action":"ping"}'   # 手动注入 MQTT 指令
```
