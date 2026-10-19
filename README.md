<div align="center">

# GoWind Ops

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Kratos](https://img.shields.io/badge/Kratos-2.9-00ADD8?logo=go&logoColor=white)](https://go-kratos.dev/)
[![React](https://img.shields.io/badge/React-19.x-61DAFB?logo=react&logoColor=white)](https://react.dev/)

[English](./README.en-US.md) | **中文** | [日本語](./README.ja-JP.md)

</div>

---

GoWind Ops 是一款开箱即用的多节点运维平台（Golang 全栈实现）。业务领域参考 [Jpom](https://gitee.com/dromara/Jpom)，信令与大文件传输架构参考 cloud-system，以 [go-wind-cms](../go-wind-cms) 为基座精简改造——继承基座的组织权限、审计日志、数据字典等系统能力，聚焦把"管机器、跑项目、发脚本、构建分发"做扎实。

**核心亮点：**

- **四服务架构** — core（领域资源层）/ admin / front（BFF）/ agent（边缘代理），职责单向分层，BFF 不连库
- **MQTT 信令长连接** — agent 纯出站接入 EMQX（无入站端口），`$share` 共享订阅多副本负载均衡，QoS 1 + request_id 幂等对账
- **大文件不过信令** — MinIO 预签名 URL 直传直拉，流式读写 + SHA-256 对账
- **指令全程审计** — 每条指令落台账（PENDING → 终态 / 超时定案），执行结果、时长与失败原因可追溯
- **构建分发闭环** — Git 仓库 → 构建（产物入 MinIO）→ 分发到目标项目目录 → 联动重启，全程状态机跟踪
- **基座同源管理前端** — React 19 + Ant Design 6 + ProComponents，与 go-wind-cms admin 同源裁剪，登录/权限/审计/字典开箱即用

## 技术栈

### 后端

| 层级     | 技术                                                                                   | 说明          |
|:-------|:-------------------------------------------------------------------------------------|:------------|
| 语言     | [Go 1.26+](https://go.dev/)                                                          | 高性能编译型语言    |
| 框架     | [go-kratos](https://go-kratos.dev/)                                                  | B站开源微服务框架   |
| 依赖注入   | 手写装配（wiring.go）                                                                  | 无 DI 框架，`make register` 辅助登记 |
| ORM    | [Ent](https://entgo.io/)                                                             | Go 实体框架     |
| 数据库    | [PostgreSQL](https://www.postgresql.org/) / [MySQL](https://www.mysql.com/)          | 关系型数据库      |
| 缓存/队列  | [Redis](https://redis.io/) + [Asynq](https://github.com/hibiken/asynq)               | 缓存与异步任务     |
| 对象存储   | [MinIO](https://min.io/)                                                             | 兼容 S3 的对象存储，预签名 URL 签发 |
| 信令     | [MQTT](https://mqtt.org/)（[EMQX](https://www.emqx.io/)）                              | agent 准入/心跳/指令，QoS 1 + JSON |
| 服务注册   | [Etcd](https://etcd.io/)                                                             | 服务发现（agent 例外：纯出站不注册） |
| 链路追踪   | [Jaeger](https://www.jaegertracing.io/) + [OpenTelemetry](https://opentelemetry.io/) | 分布式可观测      |
| API 定义 | [Protobuf](https://protobuf.dev/) + [buf.build](https://buf.build/)                  | 接口契约优先      |
| 鉴权     | kratos-authn（JWT）/ kratos-authz（Casbin / OPA）                                        | 策略驱动鉴权      |

### 管理前端

| 技术                                            | 说明         |
|:----------------------------------------------|:-----------|
| [React 19](https://react.dev/)                | 渐进式前端框架    |
| [TypeScript](https://www.typescriptlang.org/) | 类型安全       |
| [Ant Design 6](https://ant.design/)           | 企业级 UI 组件库 |
| [ProComponents](https://procomponents.ant.design/) | 企业级中后台组件 |
| [Vite 6](https://vite.dev/)                   | 构建工具（dev :18999） |

## 核心功能

### 节点与项目

| 功能    | 说明                                      |
|:------|:----------------------------------------|
| 节点管理  | agent 机器指纹准入（TOFU 首连注册、未认证自动重试）、30s 心跳上报指标、在线状态、改名/打标签、下发 ping 指令 |
| 项目管理  | 项目 = 节点 + 目录 + 启动/停止命令；生命周期操作 START/STOP/RESTART/STATUS，agent 侧授权目录校验（`AGENT_ALLOWED_DIRS`）+ pid 文件管理 |
| 指令台账  | 全部指令审计落库（回执结果/退出码/时长/失败原因），超时自动定案 FAILED |
| 运行日志  | 项目进程输出落盘 `<项目目录>/.ops-project.log`，日志尾部查看（默认 64KB/上限 256KB）与清空 |

### 脚本与构建分发

| 功能    | 说明                                          |
|:------|:--------------------------------------------|
| 脚本管理  | 脚本 CRUD（内容 ≤256KB），远程执行：内容经 MQTT 下发、临时文件按平台 shell 执行、输出/退出码/时长落台账、超时杀进程树 |
| 源仓库管理 | HTTPS Git / 本地路径，密码 AES-GCM 加密落库、回显脱敏       |
| 构建    | 构建配置（仓库/分支/构建命令/产物路径）+ 异步触发（构建号自增、并发去重、中断定案），产物 glob 收集打包入 MinIO `artifacts` 桶，预签名下载 |
| 分发    | 构建产物分发到目标项目目录（zip 解包防 zip slip、clearOld 可选、分发后联动重启/启动），分发记录状态机全程跟踪 |

### 系统与运维（基座继承）

| 功能           | 说明                                    |
|:-------------|:--------------------------------------|
| 组织与权限        | 用户/角色/菜单/部门/接口权限管理，按钮级权限管控（go-wind-cms 基座继承） |
| 审计与字典        | 登录日志、操作日志、数据字典、语言管理等系统能力              |

## 项目结构

```
go-wind-ops/
├── backend/                        # 后端服务
│   ├── api/                        # Protobuf API 定义 & 生成代码
│   │   ├── protos/                 # .proto 源文件（ops 业务域 + 基座继承域）
│   │   └── gen/                    # 生成代码 (Go / TypeScript / OpenAPI)，不手改
│   ├── app/
│   │   ├── admin/service/          # 管理后台 BFF (REST :18600，gRPC → core，不连库)
│   │   ├── front/service/          # 前台 BFF (REST :18700，gRPC → core，不连库)
│   │   ├── core/service/           # 领域资源层 (Ent schema/仓储/业务逻辑 + MQTT 控制端 + Asynq worker)
│   │   └── agent/service/          # 边缘代理 (纯 MQTT 出站；无库/无服务发现/无 REST)
│   ├── pkg/                        # 公共包 (signal/topic/serviceid/oss/filetransfer/鉴权中间件...)
│   └── scripts/                    # 部署脚本 (Docker 等)
└── frontend/
    └── admin/                      # 管理前端 (React 19 + Ant Design 6 + ProComponents + Vite 6)
```

## 快速开始

### 环境要求

- Go 1.26+
- Docker & Docker Compose
- Node.js 20+ & pnpm
- buf / protoc 工具链（改 proto 时需要，`cd backend && make init` 一键安装）

### 1. 启动依赖服务

```bash
cd backend
docker compose -f docker-compose.libs.yaml up -d   # PG/Redis/MinIO/etcd/EMQX/Jaeger
```

### 2. 启动后端服务

```bash
cd backend
go build -o bin/core ./app/core/service/cmd/server && ./bin/core -c app/core/service/configs
go build -o bin/agent ./app/agent/service/cmd/server && ./bin/agent -c app/agent/service/configs
```

agent 首次启动会在工作目录生成 `auth.token` 并经 MQTT 向 core 注册（TOFU，未认证前每 5 秒重试），随后每 30 秒心跳上报指标。受管机器上运行 agent 前需配置授权目录（环境变量 `AGENT_ALLOWED_DIRS`，平台路径列表分隔符分隔），项目目录必须在授权目录内，否则 agent 拒绝执行。

### 3. 启动管理前端

```bash
cd frontend/admin
pnpm install
pnpm dev        # http://localhost:18999，代理 /admin → 127.0.0.1:18600
```

默认账号 `admin / admin`（验证码开发环境可设 `ADMIN_CAPTCHA_DISABLED=true` 跳过；登录密码需按前端约定 AES-GCM 加密，见 `backend/tools/pass-enc`）。

### 常用命令

```bash
cd backend

# 生成 Ent ORM 代码
make ent

# 生成 Protobuf API Go 代码
make api

# 生成管理前端 TypeScript 客户端
cd api && buf generate --template buf.admin.typescript.gen.yaml

# 一键生成全部代码 (ent + api + openapi)
make gen

# 构建所有服务
make build

# 运行测试
make test
```

> 管理端 REST 接口速览（节点/项目/脚本/仓库/构建/分发/台账全部端点）见 [backend/README.md](./backend/README.md)；后端开发规范与架构细节见 [backend/AGENTS.md](./backend/AGENTS.md)。

## 开发路线

- [x] 基座落地：服务四分（core/front/admin/agent）、CMS 业务域精简、MQTT 信令与 MinIO 传输基础设施
- [x] 节点管理：proto 化 REST 接口（列表/详情/改名打标签/删除/下发指令 ping）、准入重试机制、指令结果对账
- [x] 项目管理 + 指令台账：项目 CRUD 与生命周期操作（启动/停止/重启/状态，agent 侧授权目录校验 + pid 文件管理），指令审计落库与超时定案
- [x] 脚本管理：脚本 CRUD + 远程执行（内容经 MQTT 下发、临时文件执行、输出/退出码/时长落台账、超时杀进程树）
- [x] 构建模块 v1：源仓库管理（凭据加密）、构建配置与触发（构建号自增/并发去重/重启中断定案）、构建历史与产物入 MinIO、预签名下载
- [x] 运行日志：项目进程输出落盘（`<项目目录>/.ops-project.log`，修复内存泄漏），日志尾部查看（同步指令等回执）与清空
- [x] 管理前端基座：cms admin 前端拷贝裁剪（登录/布局/权限/审计/字典/组织骨架 + TS API 重生成），首个 ops 页面（节点管理：列表/ping 下发/删除）
- [x] 前端业务页七连：节点/项目（含日志查看/生命周期操作）/脚本/仓库/构建/分发/指令台账
- [x] 构建分发 v1：分发配置（clearOld/分发后动作）与触发、zip 解包防 zip slip、分发记录状态机
- [x] 多目标分发：分发配置支持多项目目标，逐目标触发分发记录
- [x] 监控告警（Monitor v1）：监控规则 CRUD + cron 调度检查 + 告警记录 + webhook 通知 + 异常自动重启
- [x] 定时构建/脚本：cron 表达式调度（CAS 锚点去重），构建在 core 触发、脚本下发到目标节点
- [ ] 准入加固：EMQX webhook 认证/ACL、STS 短令牌轮换
- [x] 定时构建：cron 表达式调度
- [x] 构建日志实时 tail(GetBuildHistoryLog 增量拉取 + 前端 2s 追加)
- [ ] 日志增强：WebSocket 实时 tail、日志文件下载/备份、控制台编码转换（Windows GBK 输出按原样字节存储）

## 参与贡献

我们欢迎各种形式的贡献，包括但不限于：

- 提交 [Issue](../../issues) 报告 Bug 或提出功能建议
- 提交 [Pull Request](../../pulls) 修复问题或添加新功能
- 完善文档
- 分享使用经验

提交前请阅读 [CONTRIBUTING.md](./CONTRIBUTING.md) 了解开发约定与门禁。

## 开源许可

本项目基于 [MIT License](./LICENSE) 开源。

## 致谢

- [go-wind-cms](../go-wind-cms) — 项目基座（Kratos + Ent + tx7do 全家桶与 admin 前端均裁剪自基座）
- [Jpom](https://gitee.com/dromara/Jpom) — 业务领域模型参考
- cloud-system — MQTT 信令与大文件传输架构参考
