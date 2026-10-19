# 更新日志

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 以下内容归入 [1.0.0]（首个正式版本）。
> 条目基于 Git 提交记录回溯整理（自 2026-10-18 项目起步至 2026-10-19），
> 仅记录对使用者有影响的功能性变更、安全加固与重要缺陷修复。

## [Unreleased]

以 go-wind-cms 为基座精简出多节点运维平台骨架后，按 ROADMAP 十个阶段推进：
从节点/项目/脚本/构建分发四大核心业务域，到安全准入、可观测性、终端与文件、
平台化（OpenAPI/插件），再到三个梯队的治理增强与 v1 收尾治理。

### 新增

#### 节点与项目

- 四服务架构落地：core（领域资源层 + MQTT 控制端 + Asynq worker）/ admin（管理 BFF :18600）/ front（前台 BFF :18700）/ agent（边缘代理，纯 MQTT 出站），服务发现走 etcd，BFF 不连库。
- 节点管理：agent 机器指纹准入（TOFU 首连注册、未认证自动重试）、30s 心跳上报指标、在线状态、改名/打标签/环境标签、下发 ping 指令。
- 项目管理：项目 = 节点 + 目录 + 启动/停止命令；START/STOP/RESTART/STATUS 生命周期操作，agent 侧授权目录校验（`AGENT_ALLOWED_DIRS`）+ pid 文件管理。
- 指令台账：全部指令审计落库（回执结果/退出码/时长/失败原因），超时自动定案 FAILED；`request_id` 幂等对账。
- 项目运行日志：进程输出落盘 `<项目目录>/.ops-project.log`，日志尾部查看（默认 64KB/上限 256KB）与清空；同步指令链路（`PublishCommandSync`）支撑查询类指令。
- 项目变更时间线：分发记录 + 告警按项目聚合的时间线抽屉。

#### 脚本与构建分发

- 脚本管理：脚本 CRUD（内容 ≤256KB）+ 远程执行——内容经 MQTT 下发、临时文件按平台 shell 执行、输出/退出码/时长落台账、超时杀进程树。
- 多节点批量脚本执行：批次表 + 逐节点下发（单点失败不影响其余），批次详情含逐节点结果矩阵（退出码/时长/输出）。
- 源仓库管理：HTTPS Git / 本地路径，密码 AES-GCM 加密落库、回显脱敏。
- 构建 v1：构建配置（仓库/分支/构建命令/产物路径）+ 异步触发（构建号自增、并发去重、中断定案），产物入 MinIO `artifacts` 桶，预签名下载。
- 构建分发（OutGiving v1）：产物分发到目标项目目录（zip 解包防 zip slip、clearOld 可选、分发后联动重启/启动），分发记录状态机全程跟踪；多目标分发逐目标出记录。
- 构建日志增量 tail：`GetBuildHistoryLog(id, offset)` + 前端 2s 追加；构建日志增量落库，RUNNING 构建自动刷新。
- 构建流水线：BUILD/SCRIPT/DISTRIBUTION 三类步骤有序编排，失败即中止、未执行标 SKIPPED，运行日志与构建日志同款 tail。
- 产物版本保留：构建配置保留最近 N 个成功产物，调度器定时清扫超额对象；支持回滚分发（用旧构建号产物重新触发）。
- Docker 镜像部署：`docker_deploy` 指令（pull → 替换容器 → run -d），分发配置 DOCKER 类型（镜像 tag 支持 `{buildNo}` 占位）。
- 定时构建/脚本：cron 表达式调度（CAS 锚点去重，首见落锚不补跑）。
- 配置文件中心：配置模板（按环境 + `{{env}}/{{node.*}}/{{project.*}}/{{date}}` 占位符渲染）→ MinIO 中转 → `file_push` 指令下发 → 下发记录终态对账。

#### 终端与文件

- Web 终端：浏览器 → admin BFF WebSocket → core gRPC 双向流 → MQTT → agent 子进程；自动重连/剪贴板/字号缩放/清屏；终端输入输出全量审计落库，只读回放（字节级保真，GBK 不错字）。
- 文件管理器：agent `file_list/read/write/delete` 指令（授权目录白名单内），目录树 + 面包屑浏览/在线编辑/新建/删除；选中机器自动进入默认工作目录；大文件走 MinIO 预签名 URL 中转（解除 1MB 限制）；终端页拖入与整页拖入上传。
- 批量命令与批量上传（多台机器即席操作）：批量命令页机器多选 + 命令 + 单台超时，逐机状态/退出码/耗时/输出实时矩阵；文件管理器批量上传——文件只 PUT 一次、各目标机分别拉取落盘，省 N-1 倍带宽。
- 项目日志实时 tail：WebSocket 端点 + 2s 增量拉取，回执 base64 原始字节，前端 UTF-8 严格解码失败回落 GBK（Windows 控制台中文不再乱码）。

#### 可观测性与治理

- 监控告警 v1：监控规则 CRUD + cron 调度检查 + 告警记录 + webhook/钉钉/飞书/企微/邮件通知渠道 + 异常自动重启；告警逐渠道落投递台账。
- 节点指标：心跳指标历史入库，ECharts CPU/内存/磁盘趋势图；节点指标阈值告警（连续 N 周期防抖、恢复自动 resolve + RECOVERY 记录）。
- 云成本：多云账号管理（AK/SK 加密台账 + 在线校验）→ 云资源盘点（全地域实例同步 + agent 节点 IP 对账）→ 成本台账（账号×月份录入/CSV 导入/月度趋势）→ 厂商账单 API 拉取（阿里/腾讯/华为/亚马逊四家）→ 云成本预算（阈值 + 突增检测告警）。
- 证书到期台账：TLS 直连探测（签发者/有效期回填），到期告警 + 续期自动恢复。
- 备份任务：目录 zip 打包 → 预签名 PUT 直传 MinIO `backups` 桶，保留策略自动清理超额备份；cron 定时 + 手动触发。
- 巡检报告：一键聚合（节点在线/资源水位/项目运行态/未恢复告警/到期证书）生成 JSON 快照，抽屉渲染明细表，支持导出。
- 日志快照检索：拉取项目日志尾部落库，跨项目 ILIKE 关键字检索返回命中摘要；支持定时采集（`LOG_SNAPSHOT_INTERVAL_MINUTES`）。
- 运维大盘：节点/项目/构建/告警概览 + 渐变横幅 + 节点资源仪表网格。

#### 平台化

- OpenAPI 与 CI/CD 集成：API Key 认证（明文仅创建时展示一次，库存 SHA-256）+ 构建/分发/流水线触发端点 + Git webhook（GitHub/GitLab/Gitea 通用 ref 解析、按分支过滤）；前端密钥管理页含 curl 用法模板。
- 插件化指令扩展：agent `plugins/` 目录脚本映射自定义 action（按扩展名选解释器、超时杀树），`plugin_list` 清单指令；前端插件指令页。
- 操作审批工作流：分发/脚本等敏感操作先建审批单，审批通过自动重新下发指令。
- 环境标签隔离 v1：节点/项目 `env` 标签（default/dev/staging/prod），列表着色 + 服务端筛选。
- Agent 远程升级：浏览器预签名直传新二进制 → `agent_upgrade` 指令 → agent 回执后接力脚本自替换重启（真机验证 31MB 全链路通过）。

#### 管理前端

- 管理前端基座（自 cms admin 裁剪）：登录/布局/权限/审计/字典/组织 + TS API 生成链。
- 业务页七连：节点/项目（含日志查看/生命周期操作）/脚本/仓库/构建/分发/指令台账；后续补齐批量执行/备份/插件/密钥/审批/通知渠道/巡检/配置中心/证书/云成本等全部业务页。
- i18n 缺键扫描器固化进仓库（`pnpm check:i18n`），新页面验收必跑。

### 变更

- 品牌独立：与上游 go-wind-cms 端口体系完全错开（18xxx），清除 CMS 品牌残留（Redis 前缀 `gwc:` → `wind_ops:`、pm2 默认名、CORS 旧域名等）。
- 数据库名 `gwc` 改为 `wind_ops`（在线 ALTER + DSN/compose/文档全量同步）。
- 运维菜单整理为六个一级功能组，菜单名统一四字风格。
- 全站 CRUD 表单 Drawer 化收尾（改密码/Excel 导入/API Key 组件化）。

### 修复

- **定时调度三连修**：新建任务 cron 锚点为 NULL 导致永不触发；ScriptRepo/BuildRepo 的 Create/Update 从未写入 cron 字段（创建带定时的任务静默丢失）；UpdateScript 内容校验拦截部分更新。
- **oneof 双写 400 全域治理**：24 个 ops request 的 optional id 改普通字段——路径变量与 body 双写同一字段触发 kratos `field already set` 400，此前所有带 id 的 body 请求全部失败。
- 文件管理路径规范化：裸盘符（如 `D:`）报错/静默失败可见。
- Web 终端 Windows Ctrl+C：改经 AttachConsole + GenerateConsoleCtrlEvent 显式投递。
- 终端会话审计抽屉表格高度塌缩；看板硬编码色值换主题 token。
- 预签名 PUT 缺 ContentLength 走 chunked 被 MinIO 拒（411）——顺带修复既有 file_pull 大文件通道。
- MQTT clientID 撞车、ops 权限种子缺失等冒烟实测修复。
- admin 前端 typecheck 历史遗留 34 处清零。

### 安全

- EMQX broker 级认证：core 暴露认证端点对接 EMQX HTTP 认证数据源（校验 agent token SHA-256），agent 只能访问自身主题（ACL）。
- ops 业务域 RBAC：节点/项目/脚本/构建/分发等资源权限码播种，前端路由挂权限码。
- 云厂商 AK/SK、Git 仓库凭据 AES-GCM 加密落库、回显脱敏。
- 瞬态对象清理：files/config-templates 中转前缀下超 24h 的对象定时清扫。

## [1.0.0] - 计划中

当前 main 分支即 1.0.0 候选，待准入加固收尾（EMQX ACL 全量覆盖、STS 短令牌轮换）后正式打 tag。

[Unreleased]: https://github.com/tx7do/go-wind-ops/compare/main...HEAD
