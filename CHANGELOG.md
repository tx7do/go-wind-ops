# 更新日志

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 以下条目基于 Git 提交记录回溯整理（自 2026-10-18 起追溯），
> 仅记录对使用者有影响的功能性变更、安全加固与重要缺陷修复。

## [Unreleased]

### 新增

#### 平台基座

- 服务四分：core（领域资源层，持有全部 Ent schema/仓储/业务逻辑 + MQTT 控制端 +
  Asynq worker）、admin / front（BFF，REST :6600/:6700，不连库，gRPC 经 etcd
  发现调 core）、agent（边缘代理，纯 MQTT 出站，无库/无服务发现/无 REST）。
- MQTT 信令基础设施：EMQX + QoS 1 + JSON codec；agent 准入（SHA-256(token)
  校验，TOFU 首连注册，未认证每 5 秒重试）、30s 心跳（内含 gopsutil 指标快照）、
  指令下发与结果回执（`request_id` 幂等对账）；core 上行走 `$share/core` 共享订阅。
- 大文件传输：MinIO 预签名 URL 直传直拉（流式 + SHA-256 对账），内容不过 MQTT。
- 节点管理：proto 化 REST（列表/详情/改名打标签/删除/下发 ping），节点由 agent
  准入自动创建，运维侧仅可改 name/labels；删除后在线 agent 重连按 TOFU 重注册。

#### 项目与脚本

- 项目管理：项目 = 节点 + 目录 + 启动/停止命令；生命周期操作
  START/STOP/RESTART/STATUS；agent 侧授权目录校验（`AGENT_ALLOWED_DIRS`）+
  pid 文件管理（`<项目目录>/.ops-project.pid`）。
- 指令台账：全部指令审计落库（回执结果/退出码/时长/失败原因），30s 扫描将
  超时（10 分钟）未回执的指令定案 FAILED。
- 脚本管理：脚本 CRUD（内容 ≤256KB）+ 远程执行——内容经 MQTT 下发、临时文件
  按平台 shell 执行、输出（截断 64KB）/退出码/时长落台账、超时杀进程树。
- 项目运行日志：进程输出落盘 `<项目目录>/.ops-project.log`（追加写，启停分隔行）；
  日志尾部查看（同步指令等回执，默认 64KB/上限 256KB）与清空。

#### 构建与分发

- 源仓库管理：HTTPS Git / 本地路径，密码 AES-GCM 加密落库（`ENCRYPTION_KEY`）、
  回显脱敏。
- 构建模块 v1：构建配置（仓库/分支/构建命令/产物路径）+ 异步触发（构建号自增、
  同配置未终态历史 409 去重、core 重启将遗留 RUNNING/PENDING 定案 FAILED）；
  git clone（argv 直传、本地盘符转 file:/// URL）→ 构建命令（超时杀树）→ 产物
  glob 收集（多文件打包 zip）→ MinIO `artifacts` 桶；构建历史（日志截断 512KB
  落库）与产物预签名下载。
- 分发 v1：分发配置（构建 → 项目，clearOld / 分发后动作）+ 触发（可指定构建
  历史）；agent 经预签名 URL 下载产物 → zip 解包（防 zip slip）/单文件复制进
  项目目录（clearOld 白名单校验）→ 分发后动作联动重启/启动；分发记录状态机，
  超时由 30s 扫描定案。

#### 管理前端

- 前端基座：cms admin 前端拷贝裁剪（登录/布局/权限/审计/字典/组织骨架），
  TS API 由后端 buf 模板生成到 `frontend/admin/src/api/generated`。
- 业务页七连：节点管理（列表/ping 下发/删除）、项目（CRUD/生命周期操作/运行
  日志查看与清空）、脚本（CRUD/远程执行）、仓库、构建配置与历史（触发/产物
  下载）、分发配置与记录（触发）、指令台账（审计与结果追溯）。

### 修复

- 项目运行日志内存泄漏：长驻进程的 Stdout 由内存 buffer 改为直接绑定文件句柄，
  避免输出把进程内存撑爆。
- 分发记录失败回执未终态：`dist_apply` 失败回执此前只处理 Success 分支，导致
  失败记录停留 PENDING；现按指令成败区分定案 FAILED/SUCCESS。
