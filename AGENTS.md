# AGENTS.md — go-wind-ops Monorepo 开发指南（AI Agent 入口）

本文件是 monorepo 总入口：定位布局、指路各端文档、声明全仓铁律。**深入开发前先读本文件与对应端的相关文档**（后端规范单一事实源在 [backend/AGENTS.md](./backend/AGENTS.md)）。

## 仓库布局

GoWind Ops：一个 Go 后端（core / admin / front / agent 四服务）+ 一个管理前端的**多节点运维平台**。以 go-wind-cms 为基座精简而来（组织权限、审计日志、数据字典等系统能力开箱即用）。

```
backend/                        Go 1.26 + Kratos + Ent（DI 手写装配 wiring.go，已弃用 Wire）
├── api/
│   ├── protos/                 .proto 源文件（ops 业务域 + 基座继承域 + admin REST 包装 i_*.proto）
│   └── gen/go/                 生成代码（HTTP/gRPC/校验），不手改
├── app/
│   ├── admin/service/          管理后台 BFF（REST :18600，gRPC → core，不连数据库）
│   ├── front/service/          前台 BFF（REST :18700，gRPC → core，不连数据库）
│   ├── core/service/           领域资源层（Ent schema/仓储/业务逻辑 + MQTT 控制端 + Asynq worker）
│   └── agent/service/          边缘代理（纯 MQTT 出站；无库/无服务发现/无 REST）
├── pkg/                        公共包（signal/topic/serviceid/oss/filetransfer/鉴权中间件...）
├── tools/                      开发调试工具（pass-enc 密码加密 / mqtt-pub 指令注入）
└── scripts/                    部署与依赖脚本（Docker 等）
frontend/
└── admin/                      管理前端：React 19 + antd 6 + ProComponents + Vite 6（dev :18999，代理 /admin → 127.0.0.1:18600）
```

四服务单向分层：BFF（admin/front）不连库、经 gRPC + etcd 服务发现调 core；agent 与 core 之间走 **MQTT（EMQX）信令长连接**（agent 纯出站、无入站端口，`$share` 共享订阅多副本负载均衡），**大文件经 MinIO 预签名 URL 直传直拉、不过信令**。架构心智模型、MQTT 信令规范与各业务域细节见 [backend/AGENTS.md](./backend/AGENTS.md)。

## 门禁（提交前必须全绿）

| 范围 | 命令（在各自目录下） |
|---|---|
| backend | `make lint` + `go vet ./...` + `go test ./...` |
| frontend/admin | `pnpm typecheck` + `pnpm lint` + `pnpm check:i18n` |

**门禁出现任何新报错，一律当作自己引入的 bug 修复**，不存在"可忽略的既有错误"。改完代码先跑门禁再声称完成。

`pnpm check:i18n` 是仓库固化的缺键扫描器（`scripts/check-i18n-keys.mjs`），**新增页面/新增文案必跑**——历史教训是 i18n 键"时点清零会腐烂"，同日新页面就又漏键。

## 全仓铁律

1. **生成产物不手改**：`backend/api/gen/go/`、`frontend/admin/src/api/generated/`、ent 生成代码都是产物。改 `.proto` 或 `ent/schema` 后必须重新生成并**连同产物一并提交**（命令见下节与 [CONTRIBUTING.md](./CONTRIBUTING.md)）。
2. **不吞错**：任何 catch 至少二选一——带出**原始错误对象**打日志，或重新抛出。用户可见的通知 ≠ 日志（只有翻译文案）。历史教训：认证链路静默吞错让排查耗时数日。
3. **proto bytes 在 TS 客户端是 base64 字符串**：生成客户端把 proto `bytes` 字段产出为 base64 string，消费侧（终端回放、日志 GBK 保真等）必须 `atob` 解码后再喂字节型 API；反向上传字节先编码。这是高频坑。
4. **触发类 POST 路由不带 path 参数**：`post: .../{id}/...` + `body: "*"` 会把 id 同时塞 path 和 body，kratos 报 oneof `field already set` 400。v1 已把 24 个 ops request 的 optional id 治理为普通字段，但**新增触发类路由仍须守规矩：id 走 body**。
5. **BFF 门面漏转发 = 编译过但运行时 500**：core 加 gRPC 方法后，admin/front 门面的嵌入接口必须同步补方法转发，漏了编译器不报错、调到才 panic 500。新增 RPC 时把两端门面当同一提交的一部分。
6. **MQTT 主题与载荷只用常量**：主题常量从 `pkg/topic` 引用、载荷结构从 `pkg/signal` 引用，**禁止裸拼主题字符串**；新增指令动作在 agent 侧 `RegisterHandler` 注册；`request_id`（UUID）为幂等键，回执原样带回。
7. **CRUD 表单一律 Drawer**：管理前端 create/update 一律走 DrawerForm（照 `CloudAccountDrawer` 抄），Modal 只留查看/信息/执行确认类轻交互（全站约定）。Drawer 里塞 ListTable 三件事：容器 `height:100%` + hook 传 `open` + 内部勿用 rAF。
8. **i18n 键成对落库**：前端新页面五件套（api / hooks / 页面 / router/modules / locales）里 locales 是其中一件，zh-CN 与 en-US 键集必须同步，验收以 `pnpm check:i18n` 全绿为准。
9. **agent 侧安全边界**：一切落盘/执行类指令（项目操作/脚本/文件/备份/插件/升级）必须过 `AGENT_ALLOWED_DIRS` 授权目录校验。新增会碰文件系统的指令动作，先想这层再动手。
10. **中间件顺序不能动**：admin/front REST 链 logging → auth.Server → authz.Server → ent.Server（租户 Viewer）。auth 必须在 ent 之前，颠倒会导致租户隔离失效。
11. **ent 枚举与 proto 枚举按"值名"映射**（EnumTypeConverter）：两端正名必须一致；同包顶层枚举值名冲突时把枚举嵌进 message（如 `BuildHistory.Status`），而不是加前缀。

## 代码生成与常用命令

在 `backend/` 下执行（根 Makefile include `app.mk`，Windows 下在 Git Bash / WSL 跑 make 即可）：

```bash
make ent          # ent 代码生成（改 internal/data/ent/schema 后必跑）
make api          # buf 生成 protobuf Go 代码
make ts           # 管理前端 TypeScript 客户端（等价：cd api && buf generate --template buf.admin.typescript.gen.yaml）
make gen          # ent + api + openapi 一键生成
make register ENTITY=xxx   # 新 CRUD 模块装配注入（wiring.go / rest_server.go 锚点行注入，幂等可重跑）
make build        # 编译全部服务
make lint / make vet / make test   # 门禁
make compose-up   # docker compose 拉起中间件 + 四服务
```

- 依赖注入是**手写装配**（各服务 `cmd/server/wiring.go`），已弃用 Wire——不要运行 wire 命令，也不要在新代码里引入 wire 依赖。
- 分层与装配约定（`register:*` 锚点行、cleanup 注册、构造函数签名 `NewXxx(ctx *bootstrap.Context, ...)`）见 [backend/AGENTS.md](./backend/AGENTS.md)。
- proto 工具链（buf + protoc + 插件）一键安装：`make init`。

## 本地验证要点

- **依赖服务**：`docker compose -f docker-compose.libs.yaml up -d`（在 `backend/` 下）起 PG / Redis / MinIO / etcd / EMQX / Jaeger。数据库名 **wind_ops**（2026-10-19 由 gwc 在线改名，DSN/compose 已全量同步）。
- **端口**：admin REST `:18600`、front REST `:18700`、core gRPC 随机端口（etcd 发现名 `gowind-ops/core-service`）、前端 dev `:18999`。
- **登录**：`admin / admin`；登录接口的 `password` 字段传 **AES-GCM 密文**（`go run ./tools/pass-enc admin` 生成），不是明文；图形验证码开发可设 `ADMIN_CAPTCHA_DISABLED=true` 跳过。
- **冒烟顺序**：libs → core → agent（配 `AGENT_ALLOWED_DIRS`，日志出现 enrolled/authenticated）→ admin → 登录 → 建项目 → 下发操作 → 查指令台账。完整冒烟清单见 [backend/AGENTS.md](./backend/AGENTS.md)。
- **起服务前清 HTTP_PROXY**：带代理环境变量启动会让 grpc-go 走 7890 之类代理拨 core，报 503 connection error——先 `env -u HTTP_PROXY ...` 清掉再起。
- **换代码必须重建对应 exe 再重启**：只重启不重建 = 旧二进制静默服务（ent 新表不建、新 RPC Unimplemented）。判定：`ls --time-style=full-iso` 对时间线，或 `go version -m xxx.exe`。
- **禁止按映像名 taskkill**（Windows）：会误杀同映像的无关进程；按 PID 或完整命令行过滤杀。
- **`go test ./...` 偶发 `fork/exec ...test.exe: Access is denied.`**（Windows 对刚链接好的测试二进制执行被拦，疑似杀软实时扫描）：环境问题、**不是代码失败**。复验：`go test -c -o <工作区内路径>/x.test.exe ./pkg/x` 后直接跑该 exe。
- **Windows 控制台 GBK**：项目日志按**原样字节**存储与传输（回执 base64 保真），前端 UTF-8 严格解码失败回落 GBK；新增日志链路别在中间做转码。
- **菜单种子仅在空表时播种**：新增前端页面后，已有库需在「菜单管理」手动补菜单（前端路由不受影响，页面直接可访问）。

## 文档索引

- 根 README（三语）：[README.md](./README.md) / [README.en-US.md](./README.en-US.md) / [README.ja-JP.md](./README.ja-JP.md)
- 后端规范单一事实源：[backend/AGENTS.md](./backend/AGENTS.md)（四服务架构、MQTT 信令规范、各业务域实现要点、坑清单）
- 后端 README：[backend/README.md](./backend/README.md)（REST 接口速览、快速开始）
- 脚本与工作流：[backend/scripts/WORKFLOWS_AND_BEST_PRACTICES.md](./backend/scripts/WORKFLOWS_AND_BEST_PRACTICES.md)
- 功能路线图：[ROADMAP.md](./ROADMAP.md)（Phase 1-10 已完成项的实现口径与技术决策记录）
- 贡献指南：[CONTRIBUTING.md](./CONTRIBUTING.md)；安全上报：[SECURITY.md](./SECURITY.md)；变更记录：[CHANGELOG.md](./CHANGELOG.md)
