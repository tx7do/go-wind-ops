<div align="center">

# GoWind Ops

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Kratos](https://img.shields.io/badge/Kratos-2.9-00ADD8?logo=go&logoColor=white)](https://go-kratos.dev/)
[![React](https://img.shields.io/badge/React-19.x-61DAFB?logo=react&logoColor=white)](https://react.dev/)

**English** | [中文](./README.md) | [日本語](./README.ja-JP.md)

</div>

---

GoWind Ops is a batteries-included multi-node operations platform built with Golang. Its business domain is modeled after [Jpom](https://gitee.com/dromara/Jpom); its signaling and large-file transfer architecture reference cloud-system; and the whole project is trimmed down from the [go-wind-cms](../go-wind-cms) base — inheriting the base's organization/permission, audit log and data dictionary capabilities while focusing on managing machines, running projects, shipping scripts, and build & distribution.

**Highlights:**

- **Four-service architecture** — core (domain resource layer) / admin / front (BFF) / agent (edge proxy), strictly layered, BFFs never touch the database
- **MQTT signaling over long-lived connections** — agents connect outbound only (no inbound ports) via EMQX, `$share` shared subscriptions for multi-replica load balancing, QoS 1 with request_id idempotent reconciliation
- **Large files never travel through signaling** — direct upload/download via MinIO presigned URLs, streaming I/O with SHA-256 verification
- **Full command audit trail** — every command is recorded (PENDING → terminal state / timeout verdict) with results, duration and failure reasons traceable
- **Closed-loop build & distribution** — Git repository → build (artifacts into MinIO) → distribute to target project directories → chained restarts, tracked by state machines end to end
- **Admin frontend from the same base** — React 19 + Ant Design 6 + ProComponents, trimmed from the go-wind-cms admin; login/RBAC/audit/dictionary work out of the box

## Tech Stack

### Backend

| Layer | Technology | Notes |
|:------|:-----------|:------|
| Language | [Go 1.26+](https://go.dev/) | High-performance compiled language |
| Framework | [go-kratos](https://go-kratos.dev/) | Bilibili's open-source microservice framework |
| DI | Hand-written wiring (wiring.go) | No DI framework; `make register` assists registration |
| ORM | [Ent](https://entgo.io/) | Entity framework for Go |
| Database | [PostgreSQL](https://www.postgresql.org/) / [MySQL](https://www.mysql.com/) | Relational databases |
| Cache/Queue | [Redis](https://redis.io/) + [Asynq](https://github.com/hibiken/asynq) | Caching and async tasks |
| Object Storage | [MinIO](https://min.io/) | S3-compatible storage, presigned URL issuing |
| Signaling | [MQTT](https://mqtt.org/) ([EMQX](https://www.emqx.io/)) | Agent admission/heartbeat/commands, QoS 1 + JSON |
| Service Registry | [Etcd](https://etcd.io/) | Service discovery (agents are the exception: outbound only, never registered) |
| Tracing | [Jaeger](https://www.jaegertracing.io/) + [OpenTelemetry](https://opentelemetry.io/) | Distributed observability |
| API Definition | [Protobuf](https://protobuf.dev/) + [buf.build](https://buf.build/) | Contract-first APIs |
| Auth | kratos-authn (JWT) / kratos-authz (Casbin / OPA) | Policy-driven authorization |

### Admin Frontend

| Technology | Notes |
|:-----------|:------|
| [React 19](https://react.dev/) | Progressive frontend framework |
| [TypeScript](https://www.typescriptlang.org/) | Type safety |
| [Ant Design 6](https://ant.design/) | Enterprise-grade UI component library |
| [ProComponents](https://procomponents.ant.design/) | Enterprise mid/back-office components |
| [Vite 6](https://vite.dev/) | Build tooling (dev :15999) |

## Core Features

### Nodes & Projects

| Feature | Description |
|:--------|:------------|
| Node Management | Agent fingerprint admission (TOFU first-connection registration, automatic retry until authenticated), 30s heartbeat metrics, online status, rename/labeling, ping commands |
| Project Management | Project = node + directory + start/stop commands; lifecycle operations START/STOP/RESTART/STATUS, agent-side allowed-directory checks (`AGENT_ALLOWED_DIRS`) + pid file management |
| Command Ledger | Every command audited (result/exit code/duration/failure reason), timeouts auto-verdicted FAILED |
| Runtime Logs | Project process output persisted to `<project dir>/.ops-project.log`; tail view (64KB default / 256KB cap) and clearing |

### Scripts, Build & Distribution

| Feature | Description |
|:--------|:------------|
| Script Management | Script CRUD (content ≤256KB); remote execution: content delivered over MQTT, executed via platform shell from a temp file, output/exit code/duration recorded to the ledger, process tree killed on timeout |
| Source Repositories | HTTPS Git / local paths; passwords AES-GCM encrypted at rest, masked on read |
| Builds | Build config (repo/branch/command/artifact path) + async trigger (auto-incrementing build number, concurrency dedup, interrupted-run verdicts); artifacts globbed, packed and pushed to the MinIO `artifacts` bucket; presigned downloads |
| Distribution | Build artifacts distributed into target project directories (zip unpacking with zip-slip protection, optional clearOld, chained restart/start after distribution); distribution records tracked by state machine |

### System & Operations (inherited from the base)

| Feature | Description |
|:--------|:------------|
| Organization & RBAC | Users/roles/menus/departments/API permissions with button-level control (inherited from go-wind-cms) |
| Audit & Dictionary | Login logs, operation logs, data dictionaries, language management and other system capabilities |

## Project Structure

```
go-wind-ops/
├── backend/                        # Backend services
│   ├── api/                        # Protobuf API definitions & generated code
│   │   ├── protos/                 # .proto sources (ops domains + inherited base domains)
│   │   └── gen/                    # Generated code (Go / TypeScript / OpenAPI), do not edit
│   ├── app/
│   │   ├── admin/service/          # Admin BFF (REST :6600, gRPC → core, no DB)
│   │   ├── front/service/          # Front BFF (REST :6700, gRPC → core, no DB)
│   │   ├── core/service/           # Domain resource layer (Ent schema/repos/logic + MQTT controller + Asynq worker)
│   │   └── agent/service/          # Edge proxy (outbound MQTT only; no DB / discovery / REST)
│   ├── pkg/                        # Shared packages (signal/topic/serviceid/oss/filetransfer/auth middleware...)
│   └── scripts/                    # Deployment scripts (Docker etc.)
└── frontend/
    └── admin/                      # Admin frontend (React 19 + Ant Design 6 + ProComponents + Vite 6)
```

## Quick Start

### Requirements

- Go 1.26+
- Docker & Docker Compose
- Node.js 20+ & pnpm
- buf / protoc toolchain (only when changing protos; `cd backend && make init` installs everything)

### 1. Start dependency services

```bash
cd backend
docker compose -f docker-compose.libs.yaml up -d   # PG/Redis/MinIO/etcd/EMQX/Jaeger
```

### 2. Start backend services

```bash
cd backend
go build -o bin/core ./app/core/service/cmd/server && ./bin/core -c app/core/service/configs
go build -o bin/agent ./app/agent/service/cmd/server && ./bin/agent -c app/agent/service/configs
```

On first start the agent generates `auth.token` in its working directory and registers with core over MQTT (TOFU, retrying every 5 seconds until authenticated), then sends heartbeat metrics every 30 seconds. Before running an agent on a managed machine, configure its allowed directories (env `AGENT_ALLOWED_DIRS`, separated by the platform path-list separator); project directories must live inside them or the agent refuses to execute.

### 3. Start the admin frontend

```bash
cd frontend/admin
pnpm install
pnpm dev        # http://localhost:15999, proxies /admin → 127.0.0.1:6600
```

Default account `admin / admin` (captcha can be skipped in development with `ADMIN_CAPTCHA_DISABLED=true`; the login password must be AES-GCM encrypted per the frontend convention — see `backend/tools/pass-enc`).

### Common Commands

```bash
cd backend

# Generate Ent ORM code
make ent

# Generate Protobuf API Go code
make api

# Generate the admin TypeScript client
cd api && buf generate --template buf.admin.typescript.gen.yaml

# Generate everything (ent + api + openapi)
make gen

# Build all services
make build

# Run tests
make test
```

> For a quick tour of all admin REST endpoints (nodes/projects/scripts/repositories/builds/distributions/ledger), see [backend/README.md](./backend/README.md); backend development conventions and architecture details live in [backend/AGENTS.md](./backend/AGENTS.md).

## Roadmap

- [x] Base landing: four-service split (core/front/admin/agent), CMS domain slimming, MQTT signaling & MinIO transfer infrastructure
- [x] Node management: proto-backed REST (list/detail/rename/label/delete/ping), admission retry, command result reconciliation
- [x] Projects + command ledger: project CRUD and lifecycle operations (start/stop/restart/status with agent-side allowed-directory checks + pid files), command auditing with timeout verdicts
- [x] Script management: script CRUD + remote execution (content over MQTT, temp-file execution, output/exit code/duration to the ledger, process-tree kill on timeout)
- [x] Build module v1: source repository management (encrypted credentials), build config & trigger (auto build numbers / concurrency dedup / interrupted-run verdicts), build history & artifacts into MinIO, presigned downloads
- [x] Runtime logs: project process output to disk (`<project dir>/.ops-project.log`, memory leak fixed), tail view (synchronous command with receipt) & clearing
- [x] Admin frontend base: cms admin frontend trimmed (login/layout/RBAC/audit/dictionary skeleton + regenerated TS API), first ops page (node management: list/ping/delete)
- [x] Seven business pages: nodes / projects (log viewer & lifecycle actions) / scripts / repositories / builds / distributions / command ledger
- [x] Distribution v1: distribution config (clearOld / post-distribution actions) & trigger, zip-slip-protected unpacking, distribution record state machine
- [ ] Business modules: multi-target distribution (node groups), monitoring & alerting (Monitor)
- [ ] Admission hardening: EMQX webhook auth/ACL, STS short-token rotation
- [ ] Build enhancements: scheduled builds (cron), live build log tail
- [ ] Log enhancements: WebSocket live tail, log download/backup, console encoding conversion (Windows GBK output stored as raw bytes)

## Contributing

All kinds of contributions are welcome, including but not limited to:

- Filing [Issues](../../issues) for bugs or feature suggestions
- Opening [Pull Requests](../../pulls) to fix problems or add features
- Improving documentation
- Sharing usage experience

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for development conventions and gates before submitting.

## License

This project is licensed under the [MIT License](./LICENSE).

## Acknowledgements

- [go-wind-cms](../go-wind-cms) — the project base (Kratos + Ent + the tx7do toolkit and the admin frontend are all trimmed from it)
- [Jpom](https://gitee.com/dromara/Jpom) — reference for the business domain model
- cloud-system — reference for MQTT signaling and large-file transfer architecture
