# AGENTS.md — go-wind-ops Monorepo 开发指南（AI Agent 入口）

本文件是 monorepo 总入口：定位布局、指路各端文档、声明全仓铁律。**深入开发前先读本文件与对应端的相关文档**。后端规范的单一事实源是 [backend/AGENTS.md](./backend/AGENTS.md)——四服务架构、MQTT 信令规范、各业务域细节、数据库/配置/冒烟顺序都在那里，本文件只做入口与全仓约定。

## 仓库布局

多节点运维平台：一个 Go 后端（core/admin/front/agent 四服务）+ 一个管理前端（cms admin 同源裁剪）。业务领域参考 Jpom，信令与大文件传输架构参考 cloud-system，基座为 go-wind-cms。

```
backend/                        Go 1.26 + Kratos + Ent（DI 手写装配 wiring.go）
├── api/
│   ├── protos/                 .proto 源文件（ops 域：agent_node/project/command_record/script/
│   │                           repository/build/build_history/distribution + 基座继承域）
│   └── gen/go/                 生成代码（HTTP/gRPC/校验），不手改
├── app/
│   ├── admin/service/          管理后台 BFF（REST :6600；不连库，gRPC → core）
│   ├── front/service/          前台 BFF（REST :6700；不连库，gRPC → core）
│   ├── core/service/           领域资源层（全部 ent schema/仓储/业务逻辑 + Asynq worker + MQTT 控制端）
│   └── agent/service/          边缘代理（纯 MQTT 出站；无库/无服务发现/无 REST）
├── pkg/                        公共包（signal/topic/serviceid/oss/filetransfer/鉴权中间件/constants...）
└── scripts/                    部署与依赖脚本（Docker 等）
frontend/
└── admin/                      管理前端：React 19 + antd 6 + ProComponents + Vite 6（dev :15999，pnpm）
```

**关键心智模型**：agent 部署在受管机器，纯 MQTT 出站接入 EMQX；core 经 `$share/core` 共享订阅收上行、按 `ops/agent/{agentId}/command` 下行；大文件走 MinIO 预签名 URL 直传直拉，不过 MQTT。指令以 `request_id`（UUID）为幂等键，全部落台账对账。

## 门禁（提交前必须全绿）

| 范围 | 命令（在各自目录下） |
|---|---|
| backend | `make lint` + `go test ./...` |
| frontend/admin | `pnpm typecheck` + `pnpm lint` |

**门禁出现任何新报错，一律当作自己引入的 bug 修复**，不存在"可忽略的既有错误"。改完代码先跑门禁再声称完成。

> 别照抄 `make vet`：Makefile 里那条是裸 `go vet`，而 `backend/` 根目录没有 `.go` 文件，跑完只吐一句 `no Go files`——什么都没检查，却不像失败。要真的过一遍，写 `go vet ./...`。

## 全仓铁律

1. **不吞错**：任何 catch 至少二选一——带出**原始错误对象**打日志，或重新抛出。用户可见的通知 ≠ 日志。合法裸 catch 仅限纯本地 best-effort 兜底且注释写明原因。
2. **生成产物不手改**：`api/gen/go/`、ent 生成代码、`frontend/admin/src/api/generated/` 都是产物。改 `.proto` 或 `ent/schema` 后必须重新生成并**连同产物一并提交**（命令见 [CONTRIBUTING.md](./CONTRIBUTING.md)）。
3. **CRUD 写请求体必须包 `{ data: {...} }`**（请求消息的 `data` 字段），更新配合 `update_mask` 做部分更新。列表搜索条件一律 contains（模糊匹配），ID 类字段不进模糊搜索。
4. **MQTT 主题与载荷一律从 `pkg/topic` / `pkg/signal` 引用，禁止裸拼主题字符串**；`request_id` 是幂等键，回执原样带回；core 上行订阅必须走 `$share/` 共享订阅；新增指令动作在 agent 侧 `RegisterHandler` 注册并落台账。
5. **中间件顺序**（admin/front REST）：logging → auth.Server → authz.Server → ent.Server（租户 Viewer）。auth 必须在 ent 之前，颠倒会导致租户隔离失效。
6. **换代码必须重建对应 exe 再重启**：只重启不重建 = 旧二进制静默服务（ent 新表不建/新 RPC Unimplemented/新字段运行时 nil）。
7. **不吞错之外的第二条命：台账是事实**。执行类指令用异步 `PublishCommand`、查询类小输出用 `PublishCommandSync`；超时由 30s 扫描定案，任何新指令链路都必须保证"回执无论成败，台账与业务记录都到终态"（分发域曾因只处理 Success 分支导致失败记录停留 PENDING）。

## 后端任务（Makefile，在 `backend/` 下执行）

| 任务 | 命令 |
|---|---|
| 运行测试 | `make test` |
| Lint | `make lint`（golangci-lint） |
| Ent 生成 | `make ent` |
| Proto / API Go 代码 | `make api`（或 `cd api && buf generate`） |
| OpenAPI 文档 | `make openapi` |
| 管理前端 TS 客户端 | `cd api && buf generate --template buf.admin.typescript.gen.yaml` |
| 一键全部生成 | `make gen`（ent + api + openapi） |
| 新 CRUD 模块装配 | `make register ENTITY=xxx [SVC=core] [PKG=opsV1]`（注入手写 wiring.go 与 rest_server.go，幂等可重跑；依赖更多的模块请手工补装配） |
| 构建所有服务 | `make build` |

- Windows 无原生 make，可用 Git Bash / MSYS make；工具链一键安装：`make init`（protoc 插件 + buf / ent / golangci-lint / gow）。
- 改 ent schema 后先 `make ent`；改 proto 后 `make api` + 按需 TS 模板，**产物连同提交**。

## 本地验证要点

- **依赖服务**：`cd backend && docker compose -f docker-compose.libs.yaml up -d`（PostgreSQL 5432 / Redis 6379 / MinIO 9000+9001 / etcd 2379 / EMQX 1883+18083 / Jaeger 16686）。
- **端口**：admin REST `:6600`、front REST `:6700`；前端 dev `:15999`（代理 `/admin` → 127.0.0.1:6600）。
- **登录账号**：服务启动时自动播种——管理员 `admin / admin`（`pkg/constants/default_data.go`），**不要**找 SQL 种子脚本。登录密码按前端约定 AES-GCM 加密传输（`go run ./tools/pass-enc admin` 生成密文）；验证码开发可设 `ADMIN_CAPTCHA_DISABLED=true` 跳过。
- **agent 联调**：必须配置 `AGENT_ALLOWED_DIRS`（项目目录须在其中）；`auth.token` 在 agent 工作目录生成（`AGENT_AUTH_TOKEN_FILE` 可覆盖路径，不要放进 configs 目录）；冒烟顺序（起 libs → core → agent → admin → pass-enc 登录 → 建项目 → operations:send）见 [backend/AGENTS.md](./backend/AGENTS.md)。
- **调试工具**：`go run ./tools/mqtt-pub -topic ops/agent/<id>/command -payload '{...}'` 手动向 agent 注入指令；MQTT 凭据默认 `core/core-pass`、`agent/agent-pass` 是占位（EMQX 未开认证时可任意连接）。
- **敏感环境变量**：`ENCRYPTION_KEY`（repository 密码等凭据加密落库，未设置降级明文并告警）、`MQTT_*`；配置支持 `${jwt_signing_key:dev_default}` 插值。
- **Windows 测试坑**：`go test ./...` 偶发 `fork/exec %TEMP%\...test.exe: Access is denied.`——Windows 对刚链接好的测试二进制执行被拦（疑似杀软实时扫描），属环境问题、**不是代码失败**。复验办法：`go test -c -o <工作区内路径>/x.test.exe ./pkg/x` 后直接跑该 exe。

## 文档索引

- 根 README（三语）：[README.md](./README.md) / [README.en-US.md](./README.en-US.md) / [README.ja-JP.md](./README.ja-JP.md)
- 后端规范单一事实源：[backend/AGENTS.md](./backend/AGENTS.md)（四服务架构 / MQTT 信令规范 / 节点·项目·脚本·构建·分发各域 / 运行日志 / 数据库 / 配置 / 构建部署 / 代码风格）
- 后端速览：[backend/README.md](./backend/README.md)（快速开始与管理端 REST 接口速览）
- 变更历史：[CHANGELOG.md](./CHANGELOG.md)；贡献流程：[CONTRIBUTING.md](./CONTRIBUTING.md)；安全策略：[SECURITY.md](./SECURITY.md)
