# go-wind-ops 功能路线图

> 基于当前已落地的四大业务域（节点/项目/脚本/构建分发）规划深层功能开发。
> 按依赖关系与价值密度分五个阶段，每阶段可独立交付。

---

## Phase 1 · 安全与准入（生产部署前置条件）

### 1.1 EMQX Broker 级认证
**现状**：MQTT 信令层已有 TOFU+token 哈希校验，但 EMQX broker 本身接受任意连接。
**方案**：core 暴露 `/api/v1/mqtt/auth` HTTP 端点（需给 core 加最小 REST listener 或 standalone 认证服务）；EMQX 配置 HTTP 认证数据源指向该端点；agent CONNECT 时 username=agentId / password=token，EMQX 调 core 校验 SHA-256 哈希。
**配套**：docker-compose 配置 EMQX authentication；ACL 规则限制 agent 只能访问 `ops/agent/{自己agentId}/#` 主题。
**工作量**：后端 1 天 + EMQX 配置半天。

### 1.2 运维资源 RBAC
**现状**：用户/RBAC 基建完整（来自 cms），但 ops 域资源（节点/项目/构建/分发）未挂权限码。
**方案**：种子数据补 ops 菜单权限码（`ops:node:read` / `ops:project:deploy` 等）；前端路由 meta.authority 挂权限码；后端 kratos-authz 策略规则覆盖 ops 服务。
**工作量**：后端半天 + 前端半天。

### 1.3 操作审批工作流
**场景**：生产环境分发/脚本执行需要审批人确认后才下发。
**方案**：分发触发后创建审批单（ops_approvals），指定审批人（角色/用户），审批通过后自动执行；前端审批列表页 + 通知。
**依赖**：站内信或 webhook 通知渠道。
**工作量**：后端 2 天 + 前端 1 天。

---

## Phase 2 · 构建与发布增强

### 2.1 构建日志实时 tail ✅ 已完成
**实现**：`GetBuildHistoryLog(id, offset)` 增量拉取（字节偏移安全切分），REST 路由 `/admin/v1/build-histories/{id}/log`；前端日志弹窗运行中每 2s 追加新片段并自动滚动，终态停止。采用增量轮询替代 SSE/WebSocket，免去 BFF 长连接改造与鉴权复杂度。

### 2.2 构建流水线（Build Pipeline）
**场景**：多模块项目需要按依赖顺序构建（先后端后前端），或构建后自动分发。
**方案**：ops_build_pipeline 表定义 DAG 步骤（每步指向 build_id 或 script_id），调度器按拓扑序执行，失败即中止。
**工作量**：后端 2 天 + 前端 1 天。

### 2.3 产物版本管理与保留策略 ✅ 已完成
**实现**：对象键含构建号天然有版本；构建配置 `artifact_retention`（保留最近 N 个成功构建产物，0=不限），调度器每小时清扫超额对象（删 MinIO + 清记录指针）；构建历史页已支持产物下载与回滚分发（回滚=用旧构建号产物重新触发分发）。

### 2.4 Docker 镜像构建 ✅ 已完成
**实现**：新增 `docker_deploy` 指令（agent 侧 pull → 强删同名旧容器 → `run -d`，RunArgv 直连 argv）；分发配置新增 `deploy_type`（FILE/DOCKER），DOCKER 类型填镜像 tag（支持 `{buildNo}` 占位替换为实际构建号）+ 容器名/端口/环境/卷/重启策略/网络，触发时跳过产物预签名，回执复用分发记录定案链路。

---

## Phase 3 · 可观测性

### 3.1 WebSocket 日志实时 tail
**现状**：运行日志为 5s 轮询。
**方案**：admin BFF 暴露 WebSocket 端点 `/admin/v1/ws/projects/{id}/log`；BFF 经 MQTT 同步指令增量拉取（带 offset），推送到浏览器。前端 LogDrawer 改 WebSocket 消费。
**工作量**：后端 1.5 天 + 前端 1 天。

### 3.2 节点监控大盘
**现状**：心跳指标落库但无可视化。
**方案**：心跳指标历史入库（ops_node_metrics：node_id/timestamp/cpu/mem/disk）；前端用 ECharts 画 CPU/内存/磁盘趋势图（可选自动刷新）；大盘首页展示节点拓扑概览（在线/离线/告警数/最近分发）。
**依赖**：echarts 依赖已在前端 package.json。
**工作量**：后端 1 天 + 前端 1.5 天。

### 3.3 告警通知渠道扩展
**现状**：告警仅 webhook POST JSON。
**方案**：复用 cms 的通知域骨架（已精简，需重建精简版）：邮件（SMTP）/钉钉机器人/企业微信机器人/飞书机器人；ops_monitors.webhook_url 扩展为多渠道配置（JSON 数组）；告警级别可路由到不同渠道。
**工作量**：后端 2 天 + 前端 1 天。

---

## Phase 4 · 终端与文件

### 4.1 Web 终端
**场景**：浏览器直接打开受管机器的 shell 终端。
**方案**：admin BFF 暴露 WebSocket 端点；经 MQTT 发 `terminal_open` 指令给 agent，agent 起子进程（bash/cmd），Stdin/Stdout 桥接到 MQTT 双向主题 `ops/agent/{id}/terminal/{sessionId}`；BFF 做 WebSocket↔MQTT 双向桥接。安全：会话超时/操作审计（终端输入落库）。
**依赖**：MQTT QoS 1 延迟可接受（非高频交互场景）。
**工作量**：后端+agent 2 天 + 前端（xterm.js）1 天。

### 4.2 文件管理器 ✅ 已完成
**实现**：agent `file_list`/`file_read`/`file_write`/`file_delete` 指令（AGENT_ALLOWED_DIRS 白名单内，读上限 1MB）；core `FileService` 经 `PublishCommandSync` 同步代理（15s 回执等待）；REST `/admin/v1/file/*`；前端文件管理页（机器选择 + 目录浏览 + 面包屑 + 在线编辑/新建/删除，unicode 安全 base64）。大文件走 MinIO 预签名 URL 的上传/下载 UI 暂缓。

---

## Phase 5 · 平台化

### 5.1 OpenAPI 与 CI/CD 集成
**场景**：外部系统（Jenkins/GitLab CI/GitHub Actions）通过 API 触发构建/分发。
**方案**：ops 域关键操作暴露 OpenAPI 端点（带 API Key 认证）；GitLab CI 调 `POST /api/v1/builds/{id}:trigger` 即可；Webhook 接收器（Git push → 自动触发关联构建）。
**工作量**：后端 1 天。

### 5.2 环境与工作空间
**场景**：dev/staging/prod 环境隔离，同名项目在不同环境独立管理。
**方案**：复用已有租户体系做环境隔离（每环境一个租户）；或引入轻量 `env` 标签字段（更简单，推荐 v1）。
**工作量**：后端 1 天 + 前端半天。

### 5.3 插件化指令扩展
**场景**：业务团队自定义 agent 指令（如数据库备份、SSL 证书续期）。
**方案**：agent 侧 `RegisterHandler` 已有注册机制；增加从 `plugins/` 目录动态加载脚本文件映射为自定义 action；core 侧增加 action 注册表管理页面。
**工作量**：后端+agent 1.5 天 + 前端 1 天。

---

## 依赖关系

```
Phase 1 (安全) ──────────── 生产部署前置
    │
Phase 2 (构建增强) ──────── 依赖 Phase 1 的 RBAC(操作权限)
    │
Phase 3 (可观测性) ──────── 独立,可并行
    │
Phase 4 (终端/文件) ─────── 依赖 Phase 1 的 ACL(文件访问控制)
    │
Phase 5 (平台化) ────────── 依赖 Phase 1-3 稳定
```

## 工作量估算汇总

| 阶段 | 后端 | 前端 | 合计 |
|---|---|---|---|
| Phase 1 安全与准入 | 3.5 天 | 1 天 | 4.5 天 |
| Phase 2 构建增强 | 6 天 | 2.5 天 | 8.5 天 |
| Phase 3 可观测性 | 4.5 天 | 3.5 天 | 8 天 |
| Phase 4 终端与文件 | 4 天 | 3 天 | 7 天 |
| Phase 5 平台化 | 3.5 天 | 1 天 | 4.5 天 |
| **合计** | **21.5 天** | **11 天** | **32.5 天** |

---

## 技术决策记录

| 决策 | 结论 | 原因 |
|---|---|---|
| 构建执行位置 | core 机器(经典模式) | agent 机器不需装构建工具链;后续可扩展节点构建 |
| 定时调度 | 进程内 ticker + cron 解析 + CAS 锚点 | 避免 asynq cron 注册/注销复杂度;CAS 保证多副本安全 |
| 日志实时方案 | 先轮询+offset,后 WebSocket | 轮询已可用;WebSocket 需 BFF 做桥接,放在 Phase 3 |
| 多目标分发 | JSON 数组而非关联表 | 目标数少(通常 <10),避免额外 join;后续量大再拆表 |
| 监控检查 | MQTT 同步指令探测 | 复用已验证的指令链路,不引入 Prometheus 等重依赖 |
| 脚本执行 | 临时文件 + 平台 shell | 最通用;后续可加 Docker 容器执行隔离 |
| 枚举映射 | ent/proto 值名必须一致 | EnumTypeConverter 按名映射;冲突时嵌套进 message |
| Windows 超时 | killTree 杀整棵进程树 | exec.CommandContext 只杀外壳;孙进程继承管道致 EOF 永不到来 |
