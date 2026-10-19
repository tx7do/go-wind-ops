# AGENTS.md — go-wind-ops 后端开发指南

> 本文件是 backend 子项目的开发规范单一事实源，供开发者与 AI 编码工具使用。管理前端见 `frontend/admin`（React 19 + antd 6 + Vite 6，裁剪自 cms admin 前端；TS API 由后端 `cd api && buf generate --template buf.admin.typescript.gen.yaml` 生成到 `frontend/admin/src/api/generated`；dev 代理 `/admin` → 127.0.0.1:18600；页面加法五件套：api/hooks/页面/router/modules/locales，参照 `pages/app/ops/agent-node`）。

## 项目概览

**go-wind-ops** 是一个多节点运维平台，以 go-wind-cms 为基座精简而来。受管机器上运行 **agent**，经 **MQTT（EMQX）** 与 core 保持信令长连接；大文件经 **MinIO/S3 预签名 URL** 直传直拉，不经过 MQTT。

**核心技术栈**：
- **框架**: go-kratos v2.9.2（HTTP + gRPC）
- **语言**: Go 1.26
- **ORM**: entgo.io/ent v0.14（schema 即代码，privacy/entql/sql/modifier/sql/upsert/sql/lock 特性）
- **依赖注入**: 手写装配（各服务 `cmd/server/wiring.go`，无 DI 框架，`make register` 辅助登记新模块）
- **数据库**: PostgreSQL（主）/ MySQL，驱动 pgx/v5
- **缓存/队列**: Redis（go-redis/v9）、Asynq 异步任务
- **鉴权**: kratos-authn（JWT HS256）+ kratos-authz（Casbin / OPA）
- **对象存储**: MinIO（`pkg/oss` 封装，预签名 URL 签发）
- **信令**: MQTT（EMQX broker，`tx7do/kratos-transport/transport/mqtt` + `kratos-bootstrap/transport/mqtt`，QoS 1，JSON codec）
- **服务发现**: etcd（agent 例外：不注册、纯出站）
- **可观测性**: OpenTelemetry + Jaeger
- **日志**: zap（经 kratos-bootstrap 封装为 `log.Helper`）

**基础设施全家桶**: `tx7do` 系列库（go-utils / go-crud / kratos-bootstrap / kratos-transport / kratos-authn/authz）是项目基建主力，修改集成点前先理解其封装。

## 四服务架构（关键心智模型）

```
用户/浏览器
  ├─ admin-service  管理后台 BFF（REST :18600，gRPC 客户端 → core；不连数据库）
  └─ front-service  前台 BFF（REST :18700，gRPC 客户端 → core；不连数据库）
        │ gRPC + etcd 服务发现（discovery:///gowind-ops/core-service）
        ▼
    core-service    领域资源层（gRPC 随机端口 + Asynq worker + MQTT 控制端）
        │              ├─ 持有全部 ent schema / 仓储 / 业务逻辑
        │              ├─ MQTT $share/core 组共享订阅 agent 上行（auth/heartbeat/command_result）
        │              └─ 下行指令 publish ops/agent/{agentId}/command
        ▼ MQTT(EMQX)            ▲ 预签名 URL
    agent-service   边缘代理（纯 MQTT 出站；无数据库、无服务发现、无 REST）
                       ├─ 启动加载/生成本机 token（configs/auth.token, 0600）
                       ├─ OnConnect 发准入请求 → core 校验 SHA-256 哈希（TOFU 首连注册）
                       ├─ 30s 心跳（内含 gopsutil 指标快照）
                       └─ 订阅 command 主题，按 action 分发（内置 ping；结果回传 command/result）
```

- 服务 ID 常量: `pkg/serviceid`（core-service / admin-service / front-service / agent-service；发现名 `gowind-ops/<id>`）
- **agent 的进程工作目录下需可写 `auth.token`**（可用 `AGENT_AUTH_TOKEN_FILE` 覆盖路径；不要放进 configs 目录，配置加载器会扫描该目录）。
- **调试工具**: `go run ./tools/mqtt-pub -topic ops/agent/<id>/command -payload '{...}'` 手动向 agent 注入指令。

## MQTT 信令规范

主题常量与 Make helper 一律从 `pkg/topic` 引用，**禁止裸拼主题字符串**：

| 主题 | 方向 | 载荷（`pkg/signal`） |
|---|---|---|
| `ops/agent/{agentId}/auth/request` | agent→core | `AgentAuthRequest{agent_id, token, info}` |
| `ops/agent/{agentId}/auth/response` | core→agent | `AgentAuthResponse{ok, reason}` |
| `ops/agent/{agentId}/heartbeat` | agent→core | `Heartbeat{ts, version, metrics}` |
| `ops/agent/{agentId}/command` | core→agent | `CommandEnvelope{request_id, action, payload}` |
| `ops/agent/{agentId}/command/result` | agent→core | `CommandResult{request_id, success, error, data}` |
| `ops/agent/{agentId}/file/{request,response,complete}` | 双向 | `FileTransfer*`（URL 走预签名，内容不过 MQTT） |

约定：QoS 1 + JSON 序列化；`request_id`（UUID）为幂等键，回执原样带回；core 上行订阅全部走 `$share/core/` 共享订阅（多副本负载均衡）；指令 action 用小写下划线枚举（现有 `ping`），新增动作在 agent 侧 `RegisterHandler` 注册。agent 未认证前每 5 秒重发准入请求（规避与 core 的启动顺序竞争）。

准入模型（当前）：SHA-256(token) 与 `ops_agent_nodes.token_hash` 一致放行；agent 首次出现按 TOFU 落库。

**Broker 级认证 + ACL（已启用，compose 默认开）**：EMQX 以 HTTP 认证/授权回调 core 的最小 REST（`core/internal/server/mqtt_auth_server.go`，:9500）：
- `/api/v1/mqtt/auth`——username=`core` 比对 core 配置的 MQTT 账密；username=agentId 校验 SHA-256(口令) 与 token_hash（**agent 以本机 token 作为 MQTT 口令连接**，首次 CONNECT 按 TOFU 注册，与信令层准入同表同 token）。
- `/api/v1/mqtt/acl`——core 放行 `ops/` 全部；agent 仅 `ops/agent/{自身用户名}/#`（`$share` 前缀剥壳后判定）；未命中一律 deny。
- compose 用 `EMQX_AUTHENTICATION__1__*` / `EMQX_AUTHORIZATION__SOURCES__1__*` 环境变量装配（BODY 模板里 `$$` 是 compose 转义）；EMQX 在容器、core 在宿主机时走 `host.docker.internal:9500`（注意放行防火墙）。
- TLS：两端 server.yaml 的 `server.mqtt.tls`（`ssl://` 端点 + `insecure_skip_verify` 或 `file.ca_path`），dev 自签可 skip，生产配 CA。**遗留升级位**：STS 短令牌轮换。

**权限码点查防线（2026-10-20）**：全站 authz 引擎为 noop（admin/front `data.NewAuthorizer()→noop.State{}`，角色→API 策略无数据通道），中间件层鉴权空转——真实防线是：
- admin BFF 门面的 **PermissionGuard 权限码点查**（`internal/service/permguard.go`，wiring 一次性注入）：31 个触发/敏感入口在方法头校验权限码（脚本执行/构建/分发/流水线 trigger、commands:send、升级收发、项目 Operate/ClearLog、文件域全部、备份触发与记录下载、配置下发、api-key 变更、审批裁决、用户增删改、角色增删改）。**码表限用已播种权限码**：ops:node_manage/project_manage/script_manage/build_manage/distribute + sys:platform_admin/sys:tenant_manager；新码须先入库播种再上门。
- 两个 WS 端点 + 终端审计接口：握手/方法期点查（`internal/authzcheck`）。
- core `SendCommand` 仅放行 ping（通用透传面收敛为白名单）。
- 用户关系型字段（角色/组织/岗位）仅当 update_mask 显式包含才生效（`user_repo.Update`，兼容驼峰/下划线键）；front BFF 自助资料更新按字段白名单重建请求。
- **真 authz 引擎已装配（2026-10-20,`AUTHZ_MODE` 三档,默认 shadow 影子模式）**：casbin 真实评估"角色→API"策略——数据通道在 `data/authz_engine.go`（角色列表 × 逐角色 `ListPermissionResources([API])` × API 表,method 统一转大写——OpenAPI 入库是小写、请求侧是大写,不转全误报）,每 5 分钟热载。`shadow`=拒绝只打 `[authz-shadow] would DENY` 日志不拦截;`enforce`=真拦截（策略加载失败 fail-closed 全拒）;`noop`=旧行为。**切 enforce 的前提**：在角色管理里配齐"角色→API"授权数据,并观察 shadow 日志无误杀。点查 PermissionGuard 与真引擎并存（点查是兜底细粒度,引擎是全量面）。- **审批闸门已接线（2026-10-20,`APPROVAL_REQUIRED_ACTIONS` 环境变量,默认空=不启用）**：配置如 `agent_upgrade,script_exec,plugin_exec`——命中动作在派发前建 PENDING 审批单并返回 403（reason=`APPROVAL_REQUIRED`,message 含审批单号）,审批人（Approve,`sys:platform_admin` 门）确认后重下发存储载荷并 MarkExecuted 闭环。仅接单目标动作（agent_upgrade/script_exec/plugin_exec）——批量/多目标与"审批单重放到单一 agent"模型不匹配,其防线是触发面权限码+危险命令确认。闸门实现 `approval_gate.go`,接线点 ScriptService.Execute/UpgradeAgent/PluginService.Exec。仓库凭据 CRUD→`ops:build_manage`。
- **遗留大工程**：API key 作用域/限流/归因、admin/app JWT 密钥分离、front BFF 真 authz（front 仍 noop,但其路由仅登录/个人资料）。

**配置面与身份面收口（2026-10-20 第三批）**：权限码门从"触发面"扩展到"配置面"——**定义执行内容的 CRUD 与触发入口同码**：脚本/流水线 CRUD→`ops:script_manage`、分发 CRUD→`ops:distribute`、构建 CRUD→`ops:build_manage`、配置模板 CRUD→`ops:project_manage`（防"改内容等他人触发"的内容投毒链）；补三个漏门：插件执行/清单（`ops:node_manage`）、日志快照 Capture/Delete、巡检 Run（`ops:project_manage`）；监控 CRUD→`ops:project_manage`。身份面：菜单/权限点 CRUD+SyncPermissions、租户 CRUD、API 表同步→`sys:platform_admin`，组织单元 CRUD→`sys:tenant_manager`（**启动自同步已抽内部方法绕过公开入口的门**，见 api_service/permission_service 的 init）。通知渠道 CRUD→`sys:platform_admin` + webhook URL 基础校验（仅 http/https,拒环回/元数据地址）。项目日志 WS 与终端 WS 统一走一次性票据（kind=project-log,绑定 projectId）。

**目标机器执行面加固（2026-10-20 第二批）**：
- 终端握手改 **30 秒一次性票据**（`/admin/v1/ws/terminal/tickets` 换票 → WS `?ticket=` GETDEL 消费）：长效 accessToken 不再进握手 URL；签发期做 token+`ops:node_manage`+agentId 绑定校验。
- file_write/push 写出前**符号链接复核**（目标 Lstat 拒链接 + 父目录 EvalSymlinks 后重过 checkDir）；插件 .bat/.cmd 参数含 cmd 元字符（`"%&|<>^!`）直接拒绝；zip 条目补 Windows 盘符绝对路径检查。
- 预签名直传直拉**出站主机白名单**：`AGENT_DOWNLOAD_ALLOWED_HOSTS`（env,逗号分隔 host），在 `pkg/filetransfer` 唯一出口收口；未配置不限制（dev），生产必须配置为 MinIO host。
- 项目 stop 的 pid 文件兜底杀树为**设计性遗留**：项目启停命令本就是任意 shell、终端本就是完整 shell，pid 投毒不构成超出设计能力的提升；如需收紧要做进程属主追踪（跨平台成本高）。

大文件传输：core 经 `pkg/oss.MinIOClient` 签发预签名 URL → 信令下发 → agent 经 `pkg/filetransfer`（`UploadFileToPresignedURL`/`DownloadToFile`，流式+SHA-256 对账）直传直拉。

## 节点管理域（ops）

- 契约：`api/protos/ops/service/v1/agent_node.proto`（core gRPC）+ `api/protos/admin/service/v1/i_agent_node.proto`（admin REST 包装）；改动后 `cd api && buf generate` 重新生成（工具链：buf + protoc + 全套本地 protoc-gen-* 插件，`make init` 可安装）
- core 实现：`app/core/service/internal/service/agent_node_service.go`（gRPC）+ `agent_service.go`（MQTT 信令）；仓储 `internal/data/agent_node_repo.go`（标准 CRUD 走 entCrud，信令链路为手写窄接口）
- admin 门面：`app/admin/service/internal/service/agent_node_service.go`，REST 前缀 `/admin/v1/agent-nodes`
- 节点记录由 agent 准入自动创建，运维侧仅可改 name/labels（gRPC 层强制剥离其余字段）

## 项目管理域（ops）与指令台账

- 契约：`api/protos/ops/service/v1/project.proto`（ProjectService：CRUD + OperateProject）与 `command_record.proto`（只读台账查询）；admin 包装 `i_project.proto`/`i_command_record.proto`
- REST：`/admin/v1/projects`（CRUD + `POST {id}/operations:send` 生命周期操作）、`/admin/v1/command-records`（审计台账）
- core：`project_service.go`（Operate 组装 `signal.ProjectOperationPayload` 下发）、`agent_service.go`（PublishCommand 创建 PENDING 台账行 → MQTT 发布；HandleCommandResult 按 request_id 回写终态，project_* 指令同时回填项目 last_pid/last_status）；扫描任务每 30s 顺带把超时(10min)未回执的台账定案 FAILED
- agent：`internal/agent/project_handlers.go`——项目目录必须在 `AGENT_ALLOWED_DIRS`（平台路径列表分隔符分隔）内，否则拒绝；start 分离进程+pid 文件(`<dir>/.ops-project.pid`)、stop 优先停止命令否则杀 pid 树、status 优先状态命令否则按 pid 存活
- 指令动作：`ping`/`project_start`/`project_stop`/`project_restart`/`project_status`/`script_exec`（`pkg/signal` 常量）；新增动作在 agent 侧 `RegisterHandler` 注册

## 脚本管理域（ops）

- 契约：`api/protos/ops/service/v1/script.proto`（ScriptService：CRUD + ExecuteScript）；admin 包装 `i_script.proto`，REST `/admin/v1/scripts`（+ `POST {id}/executions:send`）
- core：`script_service.go`——内容 ≤256KB（超出应走 MinIO，见 ScriptContentLimit）、超时默认 60s 上限 600s；脚本内容组装进 `signal.ScriptExecPayload` 经 MQTT 下发
- agent：`script_handlers.go`——内容落地临时文件（.cmd/.sh）以平台 shell 执行；**超时必须走 runShell（killTree 杀进程树）**：Windows 上 exec.CommandContext 只杀 cmd.exe 外壳，孙进程继承输出管道会导致管道 EOF 永不到来、handler 卡死
- 执行结果（输出截断 64KB/退出码/时长）记入指令台账；台账 SUCCESS/FAILED 表示"回执是否成功"，脚本业务成败看 result 里的 exit_code

## 构建域（ops,v1 在 core 机器构建）

- 契约：`api/protos/ops/service/v1/{repository,build,build_history}.proto`；admin REST `/admin/v1/repositories`、`/admin/v1/builds`（`POST {id}/builds:trigger`）、`/admin/v1/build-histories`（`{id}/artifact-url` 产物预签名下载）
- 执行器：`build_service.go` runBuild(goroutine)——git clone(argv 直传,本地盘符路径自动转 file:/// URL;`D:/...` 直写会被 git 当 scp 主机名)→ 构建命令(shellexec.Run,超时杀树)→ 产物 glob 收集(多文件打包 zip)→ MinIO `artifacts` 桶
- 状态机：PENDING→RUNNING→SUCCESS/FAILED;同配置存在未终态历史时触发返回 409;core 启动清扫把遗留 RUNNING/PENDING 定案 FAILED
- 工作空间:`OPS_BUILD_WORKSPACE`(默认 ./data/builds),构建结束即删除;日志截断 512KB 落库
- **坑**:ent 枚举与 proto 枚举按"值名"映射(EnumTypeConverter),两端正名必须一致——同包顶层枚举值冲突时把枚举嵌进 message(如 BuildHistory.Status、Distribution.AfterOpt)而不是加前缀

## 分发域(ops,OutGiving v1)

- 契约:`api/protos/ops/service/v1/distribution.proto`;admin REST `/admin/v1/distributions`(`POST {id}/distributions:trigger`,可指定 historyId)、`/admin/v1/distribution-records`
- 触发链:分发配置→目标项目→节点→最新成功构建历史→签发产物预签名 URL→`dist_apply` 指令→分发记录(PENDING,记 command_request_id)
- 回执:agent 下载产物(HTTP 直拉,不过 MQTT)→ zip 解包(防 zip slip)/单文件复制进项目目录(clearOld 可选,白名单校验)→ afterOpt 联动重启/启动(项目命令随载荷下发)→ 回执按 command_request_id 回写分发记录终态;超时由 30s 扫描定案
- **坑**:回执终态化必须区分指令成败——`dist_apply` 失败回执也要把分发记录定案 FAILED(曾因只处理 Success 分支导致失败记录停留 PENDING)

## 运行日志与同步指令

- 项目进程输出落盘 `<项目目录>/.ops-project.log`(追加写,启动/停止有分隔行);**进程 Stdout 直接绑文件句柄,禁止接内存 buffer**——长驻进程会把 buffer 撑爆
- 日志查询是**同步指令**:core `PublishCommandSync` 注册等待者→MQTT 下发→`HandleCommandResult` 投递回执→调用方 10s 超时;迟到回执仍由台账链路对账。查询类指令(小输出)用它,执行类指令用异步 `PublishCommand`
- REST:`GET /admin/v1/projects/{id}/log`(尾部,默认 64KB/上限 256KB)、`DELETE .../log`(清空)
- Windows 控制台程序输出多为 GBK:日志按**原样字节**存储与传输,不做转码(增强项:按节点编码转换)

## 开发调试工具

- `go run ./tools/mqtt-pub -topic ops/agent/<agentId>/command -payload '{"request_id":"t1","action":"ping"}'`：手动注入 MQTT 指令
- `go run ./tools/pass-enc admin`：按前端约定（AES-GCM + DefaultAESKey + base64）加密密码——登录接口的 `password` 字段传的是密文，不是明文
- 登录：`POST /admin/v1/login {"grant_type":"password","username":"admin","password":"<pass-enc 输出>"}`；验证码默认开启，开发可设 `ADMIN_CAPTCHA_DISABLED=true` 跳过

## 分层与装配约定

- **core**: `internal/service`（业务）→ `internal/data`（仓储，entCrud 泛型仓储或直用 ent client）→ `internal/data/ent/schema`（ent schema，表名前缀 `sys_`/`ops_`）
- **admin/front**: BFF 门面不连库；`internal/data` 是 core 的 gRPC 客户端构造，`internal/service` 是薄门面
- **wiring.go**: 手写装配，严格单向分层；新增模块按锚点行登记（`register:repo` / `register:client` / `register:service` / `register:param` / `register:route`），漏接由编译器报错；`make register ENTITY=xxx` 工具可自动注入
- **中间件顺序**（admin/front REST）: logging → auth.Server → authz.Server → ent.Server（租户 Viewer）。auth 必须在 ent 之前，颠倒会导致租户隔离失效
- **新 MQTT 订阅**: core 侧在 `internal/server/mqtt_server.go` 注册（$share 共享订阅）；agent 侧在 `app/agent/service/internal/server/mqtt_server.go` 注册

## 数据库

- PostgreSQL，ent 自动建表（`client.NewEntClient` 中 `Schema.Create`，`data.database.migrate: true` 时）
- 表名用 `entsql.Annotation` 显式声明；业务表前缀 `ops_`
- 初始数据播种在代码里：`pkg/constants/default_data.go`（admin/admin 管理员、默认 RBAC、基础菜单、语言）；空表才播种
- 无独立 migrations 目录；改 schema 后跑 `make ent` 重新生成

## 配置

- 每服务一套 `configs/*.yaml`（server/data/oss/registry/logger/trace；core 另有 authenticator.yaml；agent 仅 server/logger）
- `${jwt_signing_key:dev_default}` 语法支持 env 插值；敏感项走环境变量（`ENCRYPTION_KEY`、`MQTT_*`）
- MQTT 段在 server.yaml 的 `server.mqtt`（endpoint/codec/username/password/clean_session/auto_reconnect/resume_subs）
- 生产可用 `cfgexp` 导出到 etcd（`make export`），服务经 registry/etcd 空导入拉取

## 构建与部署

```bash
make build        # 编译全部服务（app/*/*/Makefile, include app.mk）
make ent          # ent 代码生成（ent CLI: go install entgo.io/ent/cmd/ent@latest）
make api          # buf 生成 protobuf（业务轮恢复 proto 工作流时用）
make compose-up   # docker compose 拉起 PG/Redis/MinIO/etcd/EMQX/Jaeger + 四服务
docker compose -f docker-compose.libs.yaml up -d   # 仅中间件
```

- 本地依赖：PostgreSQL(5432, postgres/*Abcd123456, 库名 wind_ops)、Redis(6379, *Abcd123456)、MinIO(9000/9001, root/*Abcd123456)、etcd(2379)、EMQX(1883/18083, admin/public)、Jaeger(16686)
- MQTT 凭据：core 的 server.yaml 默认 `core/core-pass`（EMQX 认证回调比对，生产务必改掉）；**agent 不再配静态账密**——进程以自身身份（username=agentId、password=本机 auth.token）连接，EMQX 认证/ACL 由 compose 环境变量启用并回调 core :9500 webhook。调试注入指令用 `tools/mqtt-pub`（须带 `-username core -password core-pass`）
- 冒烟顺序：起 libs → 起 core → 起 agent（配置 `AGENT_ALLOWED_DIRS`，日志出现 enrolled/authenticated）→ 起 admin（可设 `ADMIN_CAPTCHA_DISABLED=true`）→ `pass-enc` 加密密码登录 → `GET /admin/v1/agent-nodes` 应见节点 → 建项目 → `operations:send` START/STATUS/STOP，项目 lastStatus 随之变化，进程被真实拉起/终止 → `GET /admin/v1/command-records` 台账含每条指令的终态与结果；目录越权的操作在台账中为 FAILED

## 代码风格

- 注释、日志、错误信息用中文；对外 API 字段用 proto 风格小写下划线
- 仓储/service 构造函数统一 `NewXxx(ctx *bootstrap.Context, ...)` 签名
- cleanup 资源创建后立即注册进 wiring 的 cleanups（LIFO）
- 指标/状态字段命名与 `pkg/signal` 保持一致，避免两套词表
