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

---

## Phase 6 · 第一梯队增强(2026-10-19)✅ 已完成

### 6.1 节点指标阈值告警 ✅
**实现**:Monitor 扩展 target_type(PROJECT/NODE_METRIC)+ node_id/metric_type(CPU/MEMORY/DISK)/threshold_percent/consecutive_count(防抖)/notify_on_recovery;告警判定直接读心跳快照(ops_agent_nodes 行上字段),连续 N 周期 ≥ 阈值才告警(consecutive_hits 运行态计数),恢复自动 resolve + 落 RECOVERY 记录并通知;Alert 增加 node_id。告警级别节点规则为 WARNING、项目规则 ERROR。

### 6.2 多节点批量脚本执行 ✅
**实现**:ops_script_batches 批次表(script 名快照/目标集/成败计数/RUNNING→FINISHED)+ command_record.batch_id 关联;ExecuteScriptBatch 先建批次再逐节点下发(PublishCommand 后 AttachBatch 回填),单节点失败计 failed 不影响其余;回执路径(HANDLE 回执 + 超时扫描)双路对账批次计数,全部终态推 FINISHED;BFF `POST /admin/v1/scripts/batch-exec:send` + `GET /admin/v1/script-batches/{id}`(详情含逐节点结果矩阵:退出码/时长/输出截断 4KB);脚本页"批量执行"多选节点入口 + 独立批次列表页(RUNNING 3s 轮询)。

### 6.3 备份任务 ✅
**实现**:ops_backup_tasks(name/node/source_dir/cron/retention)+ ops_backup_records(bucket/object_key/size/sha256/PENDING→SUCCESS/FAILED,command_request_id 唯一);新指令 backup_run:agent 校验授权目录 → zipDir 递归打包(跳过非常规文件)→ 预签名 PUT 直传 MinIO backups 桶 → 回执带大小/SHA-256/文件数;对象键 backups/{taskId}/{uuid}.zip;保留策略回执后即清(保留最近 N 份成功,删对象+删记录);定时走 CAS 锚点(空锚点先落锚不补跑);BFF 全套 REST(trigger 路由**不带 path 参数**——body 与 path 双写同一字段会触发 kratos oneof CODEC 400);下载走预签名 GET。

### 6.4 项目变更时间线 ✅(纯前端)
**实现**:项目页时间线抽屉聚合 ListDistributionRecords + ListAlerts(按 projectId EQ 过滤,各取 50 条倒序合并),AntD Timeline 按状态着色;指令台账暂无 project 关联,不参与聚合(后续如需可在 command_record 加 project_id)。

### 冒烟修复的三个通用 bug
- `pkg/filetransfer.UploadFileToPresignedURL`:body 经 TeeReader 包装后 Go http 探测不到长度走 chunked,MinIO 411 Length Required → 显式 `req.ContentLength`(顺带修复既有 file_pull 大文件通道)。
- proto int64 经 JSON 是字符串(sizeBytes 等),前端展示需 `Number()` 归一。
- BFF REST `post: .../{id}/...` + `body: "*"`:生成客户端会把 id 同时塞 path 和 body,kratos 报 `field already set for oneof "_id"`;新路由触发类 POST 不要带 path 参数。

## Phase 7 · 第二梯队增强(2026-10-19)✅ 已完成

### 7.1 告警治理 ✅
**实现**:项目监控恢复语义与节点规则统一(RUNNING 且有未恢复告警 → resolve + RECOVERY 记录 + 恢复通知);Monitor 加 repeat_interval_minutes(重复告警最小间隔,距上次告警不足则跳过,恢复后重新告警同样受限);Alert 加 source(MONITOR/BUDGET/CERT)且 monitor_id 放宽为"非 MONITOR 来源存域对象 ID"(预算/证书复用 HasUnresolvedAlert/ResolveAllByMonitor 去重恢复);通知渠道 EMAIL 真发送(net/smtp,465 隐式 TLS/其余 STARTTLS,channel 加 email_to 收件人列表)+ 统一 SendAlertToChannels 逐渠道落投递台账(ops_notification_deliveries 此前只有 schema 无 repo/实现,NotificationDeliveryService 已实现并暴露 GET /admin/v1/notification-deliveries,告警页行内"投递记录"抽屉)。

### 7.2 云成本预算与突增检测 ✅
**实现**:ops_cloud_budgets(账号(0=全部合计)+月份唯一,amount/currency/threshold_percent/anomaly_ratio);调度器 30 分钟节拍 RunBudgetCheck:当月台账成本 ≥ 预算×阈值% 或 > 前 3 月均值×倍数 → 落 BUDGET 告警 + 渠道外呼(notified/anomaly_notified 防重位,手动"立即检查"重置后重算);前端云管理组"云成本预算"页(CRUD+立即检查)。

### 7.3 证书到期台账 ✅
**实现**:ops_certs(name/host/port/issuer/not_after/days_before_expire/probe_cron);core 直连 TLS 探测(tls.Dialer+InsecureSkipVerify 只取对端叶子证书,回填签发者/有效期,失败落 last_error);调度探测以 last_probed_at 为锚点(cron 跨过触发点即探);到期检查:剩余 ≤ 提前天数 → CERT 告警+外呼,续期后剩余回升 → 自动 resolve + RECOVERY;前端治理组"证书台账"页(CRUD+立即探测+到期倒计时着色)。

### 7.4 Agent 远程升级 ✅
**实现**:浏览器经 GET /admin/v1/agent-nodes/upgrade-upload-url:resolve 拿预签名 PUT 直传 agent-uploads/ 前缀 → POST upgrade:send(校验前缀防越权,预签名 GET 下发 agent_upgrade 指令);agent 下载到临时文件 → **回执先发**(台账 SUCCESS)→ 3 秒后退出,接力脚本(Windows bat/Unix sh,平台分离的 SysProcAttr)等进程退出 → 旧二进制改名腾位 → 新二进制落回原路径 → 原参数重启 → 清理;实际版本以重启后心跳上报为准。真机验证:31MB 二进制上传→下载→自替换→新 PID→重新认证,全链路通过。

### 冒烟补记
- 手动 API 冒烟建了 smoke-account/baidu-www/预算 #1 等 smoke 数据;webhook 渠道 smoke-webhook 指向必失败地址用于验证投递 FAILED 落库。
- 登录接口 access_token 字段是 access_token(snake),前端 axios 封装已适配;冒烟脚本直调时注意。

## Phase 8 · 第三梯队(2026-10-19)✅ 已完成

### 8.1 终端会话审计回放 ✅
**实现**:ops_terminal_sessions+ops_terminal_chunks(base64 保真存储,GBK 回显不错字);TerminalRecorder 每会话一实例,输入/输出经 1024 深队列异步批量落库(800ms 冲刷,单会话 8MB 上限超出置 truncated),录制失败只记日志绝不影响终端;审计 RPC 挂 TerminalService(ListSessions/GetSession/ListChunks,分块以原始字节返回);BFF 新增 i_terminal_audit.proto(REST /admin/v1/terminal-sessions*);前端终端页"会话审计"抽屉(时长/流量/分块统计)+xterm 只读回放(OUTPUT 原字节回灌、INPUT 以灰显 [cmd] 行标注)。**注意:生成 TS 客户端把 proto bytes 产出为 base64 字符串,回放/消费侧必须 atob 解码后再喂字节型 API**。本提交一并入库同文件的终端 resize 通道 WIP(WS RESIZE 帧/mqtt resize 主题/agent 处理/真 PTY 提示)。

### 8.2 巡检报告 ✅
**实现**:ops_inspections,RUN→异步聚合(节点在线/CPU/内存/磁盘/版本、项目运行态、未恢复告警、30 天内到期证书)生成 JSON 快照存 summary,概览数字冗余成列便于列表展示;前端治理组"巡检报告"页(发起+报告抽屉渲染三张明细表)。

### 8.3 日志快照检索 ✅
**实现**:ops_log_snapshots,同步指令(project_log,256KB)拉项目日志尾部落库;跨项目 ILIKE 关键字检索返回命中摘要(首命中行 ±1 上下文,400 字符截断)+命中行数;前端可观测组"日志快照"页(采集/查看/检索/删除)。v1 仅手动采集,定时采集留待后续。

### 8.4 配置文件中心 ✅
**实现**:ops_config_templates(按 env)+ops_config_distributions(回执终态);下发=内容 minio PutObject 直传 config-templates/ 前缀 → 预签名 GET → file_push 指令(agent 复用,授权目录校验),file_push 回执钩子对账下发记录(文件管理器的 file_push 无记录静默跳过);前端持续交付组"配置中心"页(模板编辑抽屉/下发弹窗/下发记录抽屉)。

### 冒烟实录
- 终端:WS 会话敲 echo/dir → 19 分块/30B↑/3.8KB↓ → 回放含命令回显与 dir 真实输出(字节级保真)。
- 巡检:RUN→FINISHED(1/1 节点、0/1 项目、告警 1)。
- 日志快照:项目补 startCommand(ping)启动产生日志 → 采集 435B → 检索 'ping' 命中。
- 配置:模板下发 smoke 库 repo-demo/app.conf → 记录 SUCCESS。
- 三新页 + 终端审计抽屉 UI 走查全通过。

### 环境注意
- 菜单种子仅在空表时播种(ops_menu_ids 86-106);**已有库需在菜单管理里手动补**"批量执行/备份任务"两个页面(前端路由不受影响,页面直接可访问)。
- 本机 smoke 环境的 agent 有未知看门狗会重拉进程,授权目录已 `setx AGENT_ALLOWED_DIRS`(用户级)兜底。
