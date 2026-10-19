<div align="center">

# GoWind Ops

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Kratos](https://img.shields.io/badge/Kratos-2.9-00ADD8?logo=go&logoColor=white)](https://go-kratos.dev/)
[![React](https://img.shields.io/badge/React-19.x-61DAFB?logo=react&logoColor=white)](https://react.dev/)

**English** | [中文](./README.md) | [日本語](./README.ja-JP.md)

</div>

---

GoWind Ops is a batteries-included, multi-node operations platform built with a full Golang stack. Organization/permissions, audit logs, and data dictionaries come out of the box — the platform focuses on doing four things well: **managing machines, running projects, shipping scripts, and building & distributing artifacts**.

**Highlights:**

- **Four-service architecture** — core (domain resource layer) / admin / front (BFF) / agent (edge proxy), strictly one-way layering; BFFs never touch the database
- **MQTT signaling over long-lived connections** — agents connect purely outbound via EMQX (no inbound ports); `$share` shared subscriptions balance load across replicas; QoS 1 + `request_id` idempotent reconciliation
- **Large files never travel through signaling** — MinIO presigned URLs for direct upload/download with streaming I/O and SHA-256 verification
- **Every command is audited** — each instruction lands in a command ledger (PENDING → terminal state / timeout verdict) with results, duration, and failure reasons fully traceable
- **Build-and-distribute pipeline** — Git repository → build (artifacts into MinIO) → distribute to target project directories / Docker deployment → linked restarts, orchestrated by pipelines and tracked end to end by state machines
- **Web terminal & file manager** — open a shell on managed machines straight from the browser (full session auditing + replay); browse/edit/batch-upload files over presigned URLs
- **Observability & governance** — monitoring & alerting (multi-channel notifications), cloud cost, certificate ledger, backups, inspections, log snapshots, and operation approvals out of the box
- **Admin frontend** — React 19 + Ant Design 6 + ProComponents, with login/permissions/audit/dictionary ready out of the box

## Screenshots

| | |
|:---|:---|
| **Ops dashboard** | **Node management** |
| ![Ops dashboard](docs/images/dashboard.png) | ![Node management](docs/images/agent-nodes.png) |
| **Project management** | **Web terminal** |
| ![Project management](docs/images/projects.png) | ![Web terminal](docs/images/terminal.png) |
| **File manager** | **Batch commands** |
| ![File manager](docs/images/file-manager.png) | ![Batch commands](docs/images/batch-commands.png) |
| **Command ledger** | **Alert records** |
| ![Command ledger](docs/images/command-records.png) | ![Alert records](docs/images/alerts.png) |
| **Build history** | |
| ![Build history](docs/images/build-history.png) | |

## Tech Stack

### Backend

| Layer | Technology | Notes |
|:------|:-----------|:------|
| Language | [Go 1.26+](https://go.dev/) | High-performance compiled language |
| Framework | [go-kratos](https://go-kratos.dev/) | Bilibili's open-source microservice framework |
| DI | Hand-written wiring (wiring.go) | No DI framework; `make register` assists with registration |
| ORM | [Ent](https://entgo.io/) | Go entity framework |
| Database | [PostgreSQL](https://www.postgresql.org/) / [MySQL](https://www.mysql.com/) | Relational databases |
| Cache / Queue | [Redis](https://redis.io/) + [Asynq](https://github.com/hibiken/asynq) | Caching and async tasks |
| Object storage | [MinIO](https://min.io/) | S3-compatible storage, presigned URL issuance |
| Signaling | [MQTT](https://mqtt.org/) ([EMQX](https://www.emqx.io/)) | Agent admission/heartbeat/commands, QoS 1 + JSON |
| Service discovery | [Etcd](https://etcd.io/) | (agents excepted: outbound-only, never registered) |
| Tracing | [Jaeger](https://www.jaegertracing.io/) + [OpenTelemetry](https://opentelemetry.io/) | Distributed observability |
| API definition | [Protobuf](https://protobuf.dev/) + [buf.build](https://buf.build/) | Contract-first APIs |
| Auth | kratos-authn (JWT) / kratos-authz (Casbin / OPA) | Policy-driven authorization |

### Admin Frontend

| Technology | Notes |
|:-----------|:------|
| [React 19](https://react.dev/) | Progressive frontend framework |
| [TypeScript](https://www.typescriptlang.org/) | Type safety |
| [Ant Design 6](https://ant.design/) | Enterprise UI component library |
| [ProComponents](https://procomponents.ant.design/) | Enterprise admin components |
| [Vite 6](https://vite.dev/) | Build tool (dev :18999) |

## Core Features

### Nodes & Projects

| Feature | Description |
|:--------|:------------|
| Node management | Agent fingerprint admission (TOFU first-connect registration with automatic retry while unauthenticated), 30s heartbeat metrics reporting, online status, rename/labels/env tags, CPU/memory/disk trend charts, ping commands |
| Project management | Project = node + directory + start/stop commands; lifecycle operations START/STOP/RESTART/STATUS with agent-side authorized-directory checks (`AGENT_ALLOWED_DIRS`) + pid file management; project change timeline (distribution records + alerts aggregated) |
| Command ledger | Every command is audited (result / exit code / duration / failure reason); timeouts are auto-verdicted as FAILED |
| Runtime logs | Process output persisted to `<project dir>/.ops-project.log`; WebSocket live tail (incremental fetch, GBK-faithful — no mojibake for Windows console output), tail viewing and clearing |

### Terminal & Files

| Feature | Description |
|:--------|:------------|
| Web terminal | Open a shell on managed machines from the browser (xterm.js over a WS↔MQTT bridge); auto-reconnect/clipboard/font scaling/clear; full session I/O audited, byte-faithful read-only replay |
| File manager | Directory tree + breadcrumb browsing, online edit/create/delete, drag-and-drop upload; selecting a machine jumps to its default working directory; large files travel over MinIO presigned URLs |
| Batch commands | Run the same ad-hoc command on multiple machines at once with a live per-machine matrix of status/exit code/duration/output; results are kept in batch records |
| Batch upload | Send the same set of files to the same directory on multiple machines — each file is PUT once, then pulled by every target machine, saving N-1 times the upload bandwidth |

### Scripts & Build/Distribution

| Feature | Description |
|:--------|:------------|
| Script management | Script CRUD (content ≤256KB), remote execution: content delivered over MQTT, executed via platform shell through a temp file, output/exit code/duration recorded in the ledger, process-tree kill on timeout; multi-node batch execution (per-node result matrix in batch detail) |
| Repository management | HTTPS Git / local paths; passwords AES-GCM encrypted at rest, redacted on read |
| Builds | Build configs + async triggers (auto-incrementing build numbers, concurrency dedup, interrupted-run verdicts); artifacts into the MinIO `artifacts` bucket; live build log tail; artifact retention policies and rollback distribution |
| Distribution | Artifacts distributed to target project directories (zip-slip-safe extraction, optional clearOld, linked restart/start after distribution), multi-target runs tracked per record; staged rollouts (N machines per batch, next batch only after the previous fully succeeds, remaining batches marked SKIPPED on failure); DOCKER deploy type (image tag supports the `{buildNo}` placeholder; agent pulls → replaces the container → `run -d`) |
| Build pipelines | Ordered BUILD / SCRIPT / DISTRIBUTION steps; fail-fast with SKIPPED marking for steps never run; incremental run-log tail |
| Scheduled jobs | Cron-scheduled builds/scripts (CAS anchor dedup, first-seen anchors without catch-up runs) |
| Config center | Per-environment config templates rendered with `{{env}}/{{node.*}}/{{project.*}}/{{date}}` placeholders and pushed out; delivery records reconciled to terminal states |

### Observability & Governance

| Feature | Description |
|:--------|:------------|
| Monitoring & alerting | Four target types: project liveness / node metric thresholds (CPU/memory/disk) / node offline / process guard (alert when a named process disappears); consecutive-period debounce, auto-resolve with RECOVERY records; auto-restart for failed projects |
| Notification channels | Webhook / DingTalk / Feishu / WeCom / email channel management; alerts fan out per channel with a per-channel delivery ledger |
| Certificate ledger | Direct TLS probing fills issuer/validity; expiry alerts with auto-resolve after renewal |
| Backup jobs | Directory zip archives pushed straight to the MinIO `backups` bucket with retention-based cleanup; cron schedules + manual triggers, presigned backup downloads |
| Inspection reports | One-click aggregation (node liveness/resource levels/project states/unresolved alerts/expiring certs) into a JSON snapshot with detail tables and export |
| Log snapshots | Project log tails persisted for cross-project keyword search returning hit excerpts; scheduled capture supported (`LOG_SNAPSHOT_INTERVAL_MINUTES`) |
| Ops dashboard | Node/project/build/alert overview + node resource gauge grid |
| Operation approvals | Sensitive operations (distribution/scripts) create approval tickets first; commands are dispatched automatically once approved |

### Cloud Management

| Feature | Description |
|:------|:----------------------------------------|
| Multi-cloud accounts | AK/SK ledger for Alibaba Cloud / Tencent Cloud / Huawei Cloud / AWS (AES-GCM encrypted at rest, redacted on read), validated online against vendor APIs with native request signing |
| Resource inventory | Full-region cloud instance sync (Alibaba ECS / Tencent CVM / AWS EC2), automatically reconciled against agent node IPs |
| Cost ledger | Per-account monthly cost entry / CSV bulk import / monthly trend charts with multi-currency display |
| Bill fetching | Pull monthly totals from vendor billing APIs (Alibaba BSS / Tencent billing / Huawei Cloud BSS / AWS CE) into the ledger in one click |
| Budgets & spikes | Monthly budget thresholds + spike detection against the trailing 3-month average, alerting on breach, with manual "check now" |

### Platform

| Feature | Description |
|:--------|:------------|
| OpenAPI | API Key authentication (plaintext shown once at creation, stored as SHA-256) + build/distribution/pipeline trigger endpoints + Git webhooks (GitHub/GitLab/Gitea common ref parsing with branch filtering) |
| Plugin commands | Scripts under the agent's `plugins/` directory map to custom actions (interpreter picked by extension, process-tree kill on timeout); frontend listing + execution |
| Agent upgrades | Presigned direct upload of a new binary from the browser → agent downloads and self-replaces → helper script restarts with the original arguments (31MB verified end to end on a real machine) |
| Environment tags | Node/project `env` tags (default/dev/staging/prod) with colored columns + server-side filtering |

### System & Operations (inherited from the base)

| Feature | Description |
|:--------|:------------|
| Authentication & sessions | Username/password login (AES-GCM encrypted in transit) + JWT, optional MFA second factor, OIDC enterprise SSO, image captchas, login policy enforcement |
| Organization & permissions | User/role/menu/department/position/tenant management, API permission sync with button-level control, permission groups |
| Audit trail | Six audit log types — login / operation / API call / data access / permission change / policy evaluation — recorded end to end |
| Dictionary & i18n | Data dictionaries (types/entries) and language management; bilingual (zh/en) admin UI with dark theme, multi-tab workspace and `Ctrl+K` command palette |

## Project Structure

```
go-wind-ops/
├── backend/                        # Backend services
│   ├── api/                        # Protobuf API definitions & generated code
│   │   ├── protos/                 # .proto sources (ops domains + inherited base domains)
│   │   └── gen/                    # Generated code (Go / TypeScript / OpenAPI) — do not hand-edit
│   ├── app/
│   │   ├── admin/service/          # Admin BFF (REST :18600, gRPC → core, no database)
│   │   ├── front/service/          # Front BFF (REST :18700, gRPC → core, no database)
│   │   ├── core/service/           # Domain resource layer (Ent schemas/repos/business logic + MQTT control plane + Asynq worker)
│   │   └── agent/service/          # Edge proxy (pure outbound MQTT; no DB / discovery / REST)
│   ├── pkg/                        # Shared packages (signal/topic/serviceid/oss/filetransfer/auth middleware...)
│   └── scripts/                    # Deployment scripts (Docker etc.)
└── frontend/
    └── admin/                      # Admin frontend (React 19 + Ant Design 6 + ProComponents + Vite 6)
```

## Quick Start

### Prerequisites

- Go 1.26+
- Docker & Docker Compose
- Node.js 20+ & pnpm
- buf / protoc toolchain (only needed when changing protos; `cd backend && make init` installs everything)

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

On first startup the agent creates `auth.token` in its working directory and registers with core over MQTT (TOFU; retries every 5 seconds until authenticated), then heartbeats metrics every 30 seconds. Before running an agent on a managed machine, configure authorized directories (environment variable `AGENT_ALLOWED_DIRS`, platform path-list separator delimited); project directories must live inside them or the agent refuses to execute.

### 3. Start the admin frontend

```bash
cd frontend/admin
pnpm install
pnpm dev        # http://localhost:18999, proxies /admin → 127.0.0.1:18600
```

Default account `admin / admin` (set `ADMIN_CAPTCHA_DISABLED=true` to skip the captcha in development; the login password must be AES-GCM encrypted per the frontend convention — see `backend/tools/pass-enc`).

### Common Commands

```bash
cd backend

# Generate Ent ORM code
make ent

# Generate Protobuf API Go code
make api

# Generate the admin frontend TypeScript client
cd api && buf generate --template buf.admin.typescript.gen.yaml

# Generate everything (ent + api + openapi)
make gen

# Build all services
make build

# Run tests
make test
```

> For a quick reference of admin REST endpoints (nodes/projects/scripts/repos/builds/distributions/ledger), see [backend/README.md](./backend/README.md); for backend conventions and architecture details see [backend/AGENTS.md](./backend/AGENTS.md).

## Roadmap

> Implementation notes and technical decisions live in [ROADMAP.md](./ROADMAP.md) (Phases 1-10 all complete).

**Done:**

- [x] Base landing: four-service split (core/admin/front/agent), slimmed system domains, MQTT signaling & MinIO presigned transfer infrastructure, EMQX broker-level authentication + ops-domain RBAC seeding
- [x] Core four domains: nodes (admission/heartbeat metrics/trend charts), projects (lifecycle + authorized-directory checks), scripts (remote + multi-node batch execution), command ledger (audited/timeout verdicts)
- [x] Build-and-distribute loop: repositories (encrypted credentials) → builds (concurrency dedup/live log tail/artifact retention/rollback) → distribution (multi-target/staged rollouts/zip-slip-safe/linked restarts/Docker deployment) → pipelines (three step types)
- [x] Terminal & files: web terminal (auto-reconnect/clipboard/session audit replay), file manager (directory tree/online editing/drag-and-drop upload/large files via presigned URLs), batch commands & batch upload, WebSocket live project-log tail (GBK-faithful)
- [x] Observability & governance: monitoring & alerting (project liveness/node thresholds/node offline/process guard + multi-channel notifications + delivery records), full cloud-cost suite, certificate ledger, backup jobs, inspection reports, log snapshot search, ops dashboard, operation approvals, change timeline
- [x] Platform: OpenAPI (API Keys + Git webhooks), plugin commands, agent remote upgrades (31MB self-replacement verified on a real machine), environment tags, config center (placeholder rendering), cron scheduling
- [x] Admin frontend: base (login/permissions/audit/dictionary) + all business pages + i18n missing-key gate (`pnpm check:i18n`) + typecheck at zero

**In progress / planned:**

- [ ] Admission hardening: full EMQX ACL coverage, STS short-token rotation
- [ ] Environment tags v2: cross-resource cascade validation between distribution targets and project env
- [ ] Associate command-ledger rows with project_id (fold commands into the timeline)
- [ ] Switch log-snapshot core storage to content_b64 (lossless for GBK scenarios)

## Contributing

All kinds of contributions are welcome, including but not limited to:

- Filing [Issues](../../issues) for bugs or feature suggestions
- Opening [Pull Requests](../../pulls) to fix problems or add features
- Improving documentation
- Sharing usage experience

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) first for development conventions and quality gates.

## License

This project is licensed under the [MIT License](./LICENSE).
