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

### 2.2 构建流水线（Build Pipeline）✅ 已完成
**实现**：ops_build_pipelines + ops_pipeline_runs 两表；步骤为有序列表（BUILD/SCRIPT/DISTRIBUTION 三类，v1 线性即退化 DAG，按序执行失败即中止，未执行步骤标 SKIPPED）。触发建 run 后异步推进：构建轮询历史终态、脚本按指令台账轮询退出码（输出截断入日志）、分发轮询全部目标记录终态（TriggerDistributionResponse 补 record_ids）。运行日志增量落库，前端与构建日志同款 2s delta tail。执行 goroutine 脱离请求生命周期，崩溃遗留 RUNNING 由启动清扫定案。

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

### 5.1 OpenAPI 与 CI/CD 集成 ✅ 已完成
**实现**：ops_api_keys 表（明文 opsk_<48hex> 仅创建时返一次，库存 SHA-256+前缀；状态/过期/最近使用）；core ApiKeyService CRUD+Validate（异步回写最近使用）。admin BFF `/openapi/` 直注册 mux（绕 JWT 中间件），handler 内验 `X-Api-Key`/`Authorization: Bearer` 后转发：`POST /openapi/v1/builds|distributions|pipelines/{id}/trigger` 与 `POST /openapi/v1/webhooks/git/{buildId}?branch=`（解析 GitHub/GitLab/Gitea 通用 ref 字段，按分支过滤命中即触发）。前端密钥管理页含明文一次性展示与 curl 用法模板。

### 5.2 环境与工作空间 ✅ 已完成(v1 轻量方案)
**实现**：agent_node/project 新增 `env` 标签（default/dev/staging/prod），节点页/项目页 env 列按环境着色 + 搜索筛选（走 PagingRequest 结构化 Filter 服务端生效）+ 表单选择；节点页顺带补了此前缺失的编辑入口（改名/改环境）。跨资源级联约束（如分发目标 env 匹配校验）留待 v2。

### 5.3 插件化指令扩展 ✅ 已完成
**实现**：agent 启动扫描 AGENT_PLUGINS_DIR（缺省 ./plugins），脚本文件名主干=action 名（plugin_<stem> 白名单校验），按扩展名选解释器（.bat/.cmd→cmd、.ps1→powershell、.sh→bash、.py→python、.exe→直连 argv），另注册 plugin_list 清单指令；执行 RunArgv 直连+超时杀树（缺省 60s 上限 600s），非零退出码作为回执返回而非传输错误。core PluginService 同步代理清单/执行（action 白名单）；前端插件指令页（清单+执行抽屉+结果弹窗）。与基座嵌入式脚本引擎（gopher-lua/goja，服务端业务钩子）定位不同——运维插件跑在目标机 OS 上，不需要解释器沙箱。

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
