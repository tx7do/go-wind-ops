# 贡献指南

感谢你对 GoWind Ops（go-wind-ops）的关注！本文档说明如何向本项目提交代码、文档与问题反馈。在开始之前，请先阅读 [README.md](./README.md) 了解项目定位与技术栈。

参与本项目即代表你同意在讨论、Issue 与 PR 中保持友善与专业。

## 快速上手

1. **Fork 并克隆仓库**

   ```bash
   git clone https://github.com/<your-name>/go-wind-ops.git
   cd go-wind-ops
   git remote add upstream https://github.com/<org>/go-wind-ops.git
   ```

2. **搭建开发环境**

   - 依赖服务一键起：`cd backend && docker compose -f docker-compose.libs.yaml up -d`（PostgreSQL / Redis / MinIO / etcd / EMQX / Jaeger）
   - 首次安装工具链：`cd backend && make init`（protoc 插件 + buf / ent / golangci-lint / gow）
   - 后端本地验证与冒烟顺序：见 [backend/AGENTS.md](./backend/AGENTS.md)

3. **创建分支**

   请基于最新的 `main` 创建特性分支，不要直接在 `main` 上开发：

   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feat/your-feature
   ```

## 开发约定

### 代码生成

本项目大量依赖代码生成，**请不要手工编辑生成产物**：

| 产物 | 生成命令 | 说明 |
|------|---------|------|
| Go API / HTTP / gRPC / 校验代码 | `make api`（或 `cd api && buf generate`） | 由 `backend/api/protos/` 下的 proto 生成 |
| OpenAPI 文档 | `make openapi` | admin / app 两份 OpenAPI 产物 |
| TypeScript HTTP 客户端 | `cd api && buf generate --template buf.admin.typescript.gen.yaml` | 生成到 `frontend/admin/src/api/generated/`（`make ts` 含全部模板） |
| Ent ORM 代码 | `make ent` | 由 `app/core/service/internal/data/ent/schema/` 下的 schema 生成 |
| 新模块接入装配 | `make register ENTITY=xxx [SVC=core] [PKG=opsV1]` | 把仓储/服务构造行与路由注册注入手写的 `wiring.go` 和 `rest_server.go`（靠锚点行定位，幂等可重跑） |

在 `backend/` 根目录执行上述 `make` 目标。Windows 无原生 make，可在 Git Bash / MSYS 环境运行；工具链未装时先 `make init`。

修改 `.proto` 或 `ent/schema` 后请重新生成并**连同产物一并提交**。

> `make register` 只覆盖 CRUD 生成器的标准形态。依赖更多的模块请手工补那几行装配（wiring 锚点：`register:repo` / `register:client` / `register:service` / `register:param` / `register:route`）。

### 输入校验

字段约束请在 proto 中用 protoc-gen-validate 的 `(validate.rules)` 声明（配套 `import "validate/validate.proto";`），避免在 service 层手写判空。生成器会产出对应的 `*.pb.validate.go`。**注意**：REST 链路没有统一的 validate 中间件，规则主要在 gRPC 校验中间件生效——给关键入参加完约束后，请实测确认真的会拦，别以为写了规则就自动生效。

### 代码规范

- **Go**：遵循 [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)。提交前在 `backend/` 下跑 `make lint`（golangci-lint）和 `go vet ./...`（注意仓库 Makefile 的 `make vet` 是裸 `go vet`，在 backend 根目录等于没检查）。
- **注释与日志**：注释、日志、错误信息用中文；对外 API 字段用 proto 风格小写下划线。
- **前端**：`frontend/admin` 提交前跑 `pnpm typecheck` + `pnpm lint`。
- **命名与风格**：新代码应与所在文件既有风格保持一致（缩进、注释密度、命名习惯）。

### 提交规范

本项目采用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)，历史提交已统一使用。提交信息格式：

```
<type>(<scope>): <subject>
```

| type | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | 缺陷修复 |
| `refactor` | 重构（不改变外部行为） |
| `perf` | 性能优化 |
| `style` | 代码格式（不影响逻辑） |
| `docs` | 文档 |
| `test` | 补充测试 |
| `build` | 构建系统、依赖 |
| `chore` | 杂项、脚手架 |
| `ci` | CI 配置 |

- `scope` 可选，对应受影响的模块（如 `core`、`admin`、`front`、`agent`、`frontend`）
- `subject` 使用中文或英文均可，简明描述，结尾不加句号
- **示例**：`feat: 构建分发(OutGiving v1)`

破坏性变更在 type 后加 `!`（如 `feat(core)!:`）并在正文说明迁移方式。大改动请添加正文说明动机与影响范围，必要时附 Issue 链接。

## 提交 Pull Request

1. 确保本地通过后端 `make lint` + `go vet ./...` + `make test`，以及改动所在前端（`frontend/admin`）的 `pnpm typecheck` + `pnpm lint`
2. 若改动涉及 proto / ent schema，确认已重新生成并提交产物：`make api && make ent`（改动落在管理前端接口时补跑 TS 模板）
3. MQTT 链路的改动：主题/载荷是否走 `pkg/topic` / `pkg/signal` 常量，指令是否落台账并保证终态
4. PR 标题遵循上述提交规范
5. PR 描述请写清：类型、动机、改动内容、**影响范围**（落在哪个服务/哪一端）、自测情况
6. 一个 PR 只解决一个问题，便于评审与回退

> Windows 上 `go test ./...` 偶发 `fork/exec %TEMP%\...test.exe: Access is denied.`——那是安全策略/杀软拦了刚链接好的测试二进制，**不是你的代码挂了**。绕开 Temp 复验：`go test -c -o <工作区内路径>/x.test.exe ./pkg/x` 后直接跑该 exe，别据此提 Issue。

### 评审标准

- 是否改动生成产物、是否漏跑生成
- 是否破坏多租户隔离（跨租户数据访问需带租户过滤；中间件顺序 auth → authz → ent 不得颠倒）
- MQTT 主题是否裸拼字符串（必须走 `pkg/topic`）；指令链路是否保证台账与业务记录到终态
- 是否引入新的硬编码密钥 / 凭据（应走配置或环境变量，如 `ENCRYPTION_KEY`）
- 是否补齐了相应测试
- 文档是否同步更新

## 反馈问题

- 缺陷请开 Issue，写清：环境（OS / Go / Node 版本）、复现步骤、期望与实际行为、相关日志
- 新功能建议请开 Issue 并说明使用场景与预期效果
- 安全漏洞请按 [SECURITY.md](./SECURITY.md) 流程私下上报，**勿直接开公开 Issue**

## 致谢

你的每一次贡献——无论代码、文档还是问题反馈——都让 GoWind Ops 更好。感谢你的参与。
