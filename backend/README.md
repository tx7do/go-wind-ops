# go-wind-ops

多节点运维平台（Go 实现）。以 [go-wind-cms](../go-wind-cms) 为基座精简改造，业务领域参考 [Jpom](https://gitee.com/dromara/Jpom)，信令与大文件传输架构参考 cloud-system。

## 服务划分

| 服务 | 角色 | 通讯 |
|---|---|---|
| **core-service** | 领域资源层：持有全部数据库 schema/仓储/业务逻辑；MQTT 控制端（agent 准入/心跳/指令下发） | gRPC(etcd 发现) + MQTT + Asynq |
| **admin-service** | 管理后台 BFF（REST :6600） | gRPC → core |
| **front-service** | 前台 BFF（REST :6700） | gRPC → core |
| **agent-service** | 边缘代理，部署在受管机器：机器指纹注册、心跳指标、指令执行、文件直传 | 纯 MQTT 出站（EMQX）+ MinIO 预签名 URL |

## 快速开始

```bash
cd backend
docker compose -f docker-compose.libs.yaml up -d   # PG/Redis/MinIO/etcd/EMQX/Jaeger
go build -o bin/core ./app/core/service/cmd/server && ./bin/core -c app/core/service/configs
go build -o bin/agent ./app/agent/service/cmd/server && ./bin/agent -c app/agent/service/configs
```

agent 首次启动会在工作目录生成 `auth.token` 并经 MQTT 向 core 注册（TOFU，未认证前自动重试），随后每 30 秒心跳上报指标。管理端 REST（默认 admin/admin 登录）：

```
GET    /admin/v1/agent-nodes              # 节点列表(在线状态/指标)
GET    /admin/v1/agent-nodes/{id}         # 节点详情
PUT    /admin/v1/agent-nodes/{id}         # 改名/打标签
DELETE /admin/v1/agent-nodes/{id}         # 删除(在线 agent 重连后按 TOFU 重注册)
POST   /admin/v1/agent-nodes/{id}/commands:send   # 下发指令(ping),返回 request_id

GET    /admin/v1/projects                 # 项目列表
POST   /admin/v1/projects                 # 创建项目(节点/目录/启动停止命令)
PUT    /admin/v1/projects/{id}            # 更新项目
DELETE /admin/v1/projects/{id}            # 删除项目
POST   /admin/v1/projects/{id}/operations:send    # 生命周期操作(START/STOP/RESTART/STATUS)

GET    /admin/v1/command-records          # 指令台账(审计+执行结果)
GET    /admin/v1/command-records/{id}     # 指令详情

GET    /admin/v1/scripts                  # 脚本列表
POST   /admin/v1/scripts                  # 创建脚本(内容 ≤256KB)
PUT    /admin/v1/scripts/{id}             # 更新脚本
DELETE /admin/v1/scripts/{id}             # 删除脚本
POST   /admin/v1/scripts/{id}/executions:send     # 在目标节点执行,输出/退出码见台账
```

agent 需配置授权目录（环境变量 `AGENT_ALLOWED_DIRS`，平台路径列表分隔符分隔），项目目录必须在授权目录内，否则 agent 拒绝执行。

详见 [backend/AGENTS.md](backend/AGENTS.md)。

## 开发路线

- [x] 基座落地：服务四分（core/front/admin/agent）、CMS 业务域精简、MQTT 信令与 MinIO 传输基础设施
- [x] 节点管理：proto 化 REST 接口（列表/详情/改名打标签/删除/下发指令 ping）、准入重试机制、指令结果对账
- [x] 项目管理 + 指令台账：项目 CRUD 与生命周期操作（启动/停止/重启/状态，agent 侧授权目录校验 + pid 文件管理），指令审计落库与超时定案
- [x] 脚本管理：脚本 CRUD + 远程执行（内容经 MQTT 下发、临时文件执行、输出/退出码/时长落台账、超时杀进程树）
- [ ] 业务模块：构建与分发（参考 jpom：Repository→Build→BuildHistory、OutGiving、Monitor）
- [ ] 准入加固：EMQX webhook 认证/ACL、STS 短令牌轮换
- [ ] 管理前端：节点/项目/脚本/台账操作页面
- [ ] 运行日志采集：项目进程输出落盘与 tail 查看（对齐 jpom 控制台日志）
- [ ] 脚本增强：定时执行（asynq cron）、脚本维度执行历史检索

## License

见 [LICENSE](LICENSE)。
