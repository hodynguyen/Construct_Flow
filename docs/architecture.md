# ConstructFlow — Architecture

## Problem Statement

Build a multi-tenant SaaS task management system for the construction industry:

1. **Multi-tenant** — Multiple companies share the system, data fully isolated between tenants
2. **Project & Task management** — Clear workflow (todo → in_progress → done/blocked)
3. **Role-based access control** — Admin, Manager, Worker with different permissions
4. **Async notifications** — Task assignment or status change triggers automatic notifications
5. **Audit trail** — Every operation is logged for traceability
6. **Full-text search** — Search tasks, projects by keyword
7. **Async reporting** — Generate reports without blocking the user
8. **File management** — Upload attachments to tasks/projects
9. **Security** — JWT RS256 authentication, RBAC authorization, rate limiting
10. **Observability** — Distributed tracing across the entire system

---

## System Overview

```
                              ┌──────────────────────────────────────┐
                              │         gw-gateway :8080             │
                              │                                      │
  Client ──── REST/JSON ────▶ │  Rate Limit ──▶ JWT Auth ──▶ RBAC    │
  (Browser/Mobile)            │       │            │           │     │
                              │       ▼            ▼           ▼     │
                              │              Gin Router              │
                              └──────┬───┬───┬───┬───┬───────────────┘
                                     │   │   │   │   │
                          gRPC calls │   │   │   │   │
                    ┌────────────────┘   │   │   │   └──────────────┐
                    ▼                    ▼   │   ▼                  ▼
            ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
            │ user-service │  │ task-service │  │ notification │  │ file-service │
            │    :50053    │  │    :50051    │  │   -service   │  │    :50054    │
            └──────┬───────┘  └──┬───┬───┬───┘  │    :50052    │  └──────┬───────┘
                   │             │   │   │      └──────┬───────┘         │
                   │             │   │   │             │                 │
                   ▼             ▼   │   ▼             ▼                 ▼
              PostgreSQL   PostgreSQL│  Redis      PostgreSQL        MinIO (S3)
                                     │  (lock)     Redis (idempotency)
                                     ▼
                                 RabbitMQ
                                     │
                      ┌──────────────┼──────────────┬────────────────┐
                      ▼              ▼              ▼                ▼
              notification-    audit-service   search-service   scheduler-
              service :50052     :50056           :50057        service :50058
                  │                 │                │               │
                  ▼                 ▼                ▼               ▼
              PostgreSQL       PostgreSQL      Elasticsearch      Redis
              Redis (idemp.)   (partitioned)                     (cron lock)
```

---

## Services

| Service | Port | Responsibility | Infrastructure |
|---------|------|---------------|----------------|
| gw-gateway | :8080 | Rate limit, JWT auth, RBAC, REST routing | Redis |
| user-service | :50053 | Register, login, JWT signing (RS256 private key) | PostgreSQL |
| task-service | :50051 | Project/task CRUD, assignment, state machine, event publishing | PostgreSQL, Redis, RabbitMQ |
| notification-service | :50052 | Consume events, idempotency check, notification store | PostgreSQL, Redis, RabbitMQ |
| file-service | :50054 | File upload, S3 presigned URLs, tiered storage | PostgreSQL, MinIO |
| report-service | :50055 | Async report jobs, status polling | PostgreSQL, RabbitMQ, MinIO |
| audit-service | :50056 | Append-only event log, partitioned table | PostgreSQL, RabbitMQ |
| search-service | :50057 | Full-text search, event-driven indexing | Elasticsearch, RabbitMQ |
| scheduler-service | :50058 | Distributed cron, deadline watch | Redis, RabbitMQ |

---

## Request Flow: Assign Task (most complex)

```
Client
  │
  │  POST /api/v1/tasks/:id/assign  { "assigned_to": "<user_id>" }
  │
  ▼
┌─────────────────────── gw-gateway ───────────────────────┐
│ 1. Rate Limit     →  Redis INCR ratelimit:<ip>           │
│ 2. JWT Auth       →  Verify RS256 with public key        │
│                      Extract: user_id, company_id, role  │
│ 3. RBAC           →  Casbin(role, company_id,            │
│                              "/tasks/assign", "write")   │
│ 4. Route          →  gRPC call to task-service           │
└──────────────────────────┬───────────────────────────────┘
                           │
                           ▼
┌─────────────────── task-service ─────────────────────────┐
│ 5. Redis SET NX "lock:task:assign:<taskID>" TTL=5s       │
│    └── Already locked? → return 409 Conflict             │
│                                                          │
│ 6. PostgreSQL: SELECT task WHERE id=? AND company_id=?   │
│    └── Not found? → return 404                           │
│                                                          │
│ 7. PostgreSQL: SELECT user WHERE id=? AND company_id=?   │
│    └── Not found? → return 400 "user not found"          │
│    └── Different company? → rejected (tenant isolation)  │
│                                                          │
│ 8. PostgreSQL: UPDATE task SET assigned_to=?             │
│                                                          │
│ 9. RabbitMQ: Publish "task.assigned" event               │
│    └── Publish fails? → log error, task still assigned   │
│                                                          │
│ 10. defer Redis DEL "lock:task:assign:<taskID>"          │
└──────────────────────────┬───────────────────────────────┘
                           │
                           ▼
┌─────────────────── RabbitMQ fan-out ─────────────────────┐
│ Exchange: constructflow.events (direct)                  │
│ Routing key: task.assigned                               │
│                                                          │
│  ├──▶ notification-service                               │
│  │    └── Redis SET NX event:<id> → INSERT notification  │
│  ├──▶ audit-service                                      │
│  │    └── INSERT audit_logs (partitioned)                │
│  ├──▶ search-service                                     │
│  │    └── Update Elasticsearch index                     │
│  └──▶ scheduler-service                                  │
│       └── Track deadline if due_date set                 │
└──────────────────────────────────────────────────────────┘
```

---

## Task State Machine

```
                 ┌──────────┐
  Create Task ──▶│   todo   │
                 └────┬─────┘
                      │
                      ▼
              ┌───────────────┐
        ┌─────│  in_progress  │────┐
        │     └───────────────┘    │
        │            ▲             │
        │            │             ▼
        │     ┌──────┴──────┐  ┌────────┐
        │     │   blocked   │  │  done  │
        │     └─────────────┘  └───┬────┘
        │                          │
        └──────────────────────────┘
           done → in_progress
          (manager/admin only)
```

| From | To | Allowed Roles |
|------|----|--------------|
| todo | in_progress | All |
| in_progress | done | All |
| in_progress | blocked | All |
| blocked | in_progress | All |
| done | in_progress | Manager, Admin only |

---

## Clean Architecture (per service)

```
┌─────────────────────────────────────────────────────┐
│                   Outer Layer                       │
│                                                     │
│  gRPC Controller ──▶ Use Case ──▶ Domain Interfaces │
│  (api/grpc/)         (use-case/)    (domain/)       │
│                                        ▲            │
│                                        │ implements │
│                          Repository ───┘            │
│                          (repository/sql/)          │
│                          Service ──────┘            │
│                          (service/)                 │
└─────────────────────────────────────────────────────┘

Inner layer (domain/ + use-case/): ZERO external imports
Outer layer: gRPC, GORM, Redis, RabbitMQ implementations
```

Directory layout:

```
apps/task-service/
├── main.go                      # Dependency injection
├── bootstrap/                   # Config, DB, Redis, RabbitMQ init
├── api/grpc/controller/         # gRPC handlers
├── domain/                      # Interfaces only (zero imports)
│   ├── task_repository.go
│   ├── project_repository.go
│   ├── user_repository.go
│   ├── event_publisher.go
│   └── lock_client.go
├── entity/
│   ├── model/                   # GORM structs
│   └── dto/                     # Request/Response + mappers
├── use-case/
│   ├── create_task/             # + _test.go
│   ├── assign_task/             # + _test.go
│   ├── update_task_status/      # + _test.go
│   └── create_project/
├── repository/sql/              # GORM implementations
├── service/                     # Redis lock, RabbitMQ publisher
└── common/                      # Errors, pagination
```

---

## Multi-Tenancy

```
JWT Claims { user_id, company_id, role }
       │
       ▼
Gateway extracts company_id
       │
       ▼
gRPC Metadata carries company_id
       │
       ▼
Every Repository: WHERE company_id = ?
```

- Every table has `company_id` column
- Every query scopes to `WHERE company_id = ?`
- No code path exists to query without tenant scope
- Cross-tenant access is structurally impossible

---

## Security

```
┌──────────────┐                    ┌──────────────┐
│ user-service │                    │  gw-gateway  │
│              │                    │              │
│ Private Key ─┼── sign JWT ──────▶ │ Public Key   │
│ (RS256)      │   {user_id,        │ (verify only)│
│              │    company_id,     │              │
│ bcrypt       │    role}           │ Casbin RBAC  │
│ cost=12      │                    │ Rate Limit   │
└──────────────┘                    └──────────────┘
```

| Layer | Implementation |
|-------|---------------|
| Authentication | JWT RS256 — private key signs (user-service), public key verifies (gateway) |
| Authorization | Casbin RBAC — domain-scoped `(role, company_id, resource, action)` |
| Password hashing | bcrypt cost=12 (~280ms per hash) |
| Rate limiting | Redis INCR per client IP, configurable RPM |
| Tenant isolation | `company_id` on every table + every query |

Gateway compromise cannot forge tokens — only public key is exposed.

---

## Database

### Tables

| Table | Service | Notes |
|-------|---------|-------|
| companies | user-service | Tenant root |
| users | user-service | `UNIQUE(email, company_id)` — same email, different companies = OK |
| projects | task-service | `company_id` scoped |
| tasks | task-service | Partial indexes, soft delete |
| notifications | notification-service | Per-user, read/unread tracking |
| files | file-service | S3 key reference |
| report_jobs | report-service | Async job status tracking |
| audit_logs | audit-service | Range partitioned by month |

### Key Indexes

```sql
-- Composite: list tasks in a project
CREATE INDEX idx_tasks_company_project ON tasks(company_id, project_id);

-- Partial: only active tasks — skip ~80% completed rows
CREATE INDEX idx_tasks_due_date ON tasks(due_date)
  WHERE status != 'done' AND deleted_at IS NULL;

-- Partial: priority sorting for backlog
CREATE INDEX idx_tasks_priority ON tasks(priority)
  WHERE status = 'todo';

-- Partial: skip soft-deleted rows
CREATE INDEX idx_tasks_deleted ON tasks(company_id)
  WHERE deleted_at IS NULL;
```

### Audit Log Partitioning

```sql
CREATE TABLE audit_logs (...) PARTITION BY RANGE (occurred_at);

-- Monthly partitions
CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
  FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

-- Retention: DROP TABLE audit_logs_2019_01 (instant, no expensive DELETE)
```

---

## Event-Driven Architecture

### Topology

```
task-service ──publish──▶ constructflow.events (direct exchange)
                              │
                              ├── routing key: task.assigned
                              │   └──▶ task_assigned_queue ──▶ notification, audit, search, scheduler
                              │
                              └── routing key: task.status_changed
                                  └──▶ task_status_changed_queue ──▶ notification, audit, search
```

### Event Envelope

```json
{
  "event_id":   "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "task.assigned",
  "timestamp":  "2026-03-09T10:00:00Z",
  "payload": {
    "task_id":     "...",
    "task_title":  "Pour concrete floor 3",
    "project_id":  "...",
    "company_id":  "...",
    "assigned_to": "...",
    "assigned_by": "..."
  }
}
```

### Reliability

| Pattern | Implementation |
|---------|---------------|
| Idempotency | `SET NX event:<event_id>` in Redis before processing |
| Dead letter queue | Failed messages (3 retries, backoff 1s/2s/4s) → `<queue>.dlq` |
| Non-fatal publish | Task persisted even if RabbitMQ publish fails |
| Persistent delivery | `DeliveryMode: Persistent` — messages survive broker restart |

---

## Extended Services — Deep Dive

### file-service (:50054) — File Management

```
Client                        file-service                    MinIO (S3)
  │                               │                              │
  │  POST /files/upload           │                              │
  │  { project_id, filename }     │                              │
  │──────────────────────────────▶│                              │
  │                               │  Generate presigned PUT URL  │
  │                               │─────────────────────────────▶│
  │    { upload_url, file_id }    │                              │
  │◀──────────────────────────────│                              │
  │                               │                              │
  │  PUT upload_url (binary)      │                              │
  │─────────────────────────────────────────────────────────────▶│
  │                               │                              │
  │  POST /files/confirm          │                              │
  │  { file_id }                  │                              │
  │──────────────────────────────▶│  INSERT files (S3 key ref)   │
  │    200 OK                     │                              │
  │◀──────────────────────────────│                              │
```

- **Presigned URL pattern**: Client uploads directly to S3/MinIO — file never passes through the service, reducing bandwidth and latency
- **Tiered storage**: Standard → Standard-IA (30 days) → Glacier (90 days) via S3 lifecycle rules
- **Metadata stored in PostgreSQL**: file_id, project_id, company_id, s3_key, content_type, size
- **MinIO locally** → **S3 in production** (same API, endpoint swap via env var)

---

### report-service (:50055) — Async Report Generation

```
Client                       report-service                 MinIO (S3)
  │                               │                            │
  │  POST /reports                │                            │
  │  { type: "project_summary" }  │                            │
  │──────────────────────────────▶│                            │
  │ { job_id, status: "pending" } │                            │
  │◀──────────────────────────────│                            │
  │                               │                            │
  │                               │  Background worker:        │
  │                               │  Query PostgreSQL (OLAP)   │
  │                               │  Generate CSV/PDF          │
  │                               │  Upload to S3              │
  │                               │  UPDATE job status="done"  │
  │                               │───────────────────────────▶│
  │  GET /reports/:job_id         │                            │
  │──────────────────────────────▶│                            │
  │    { status: "done",          │                            │
  │      download_url: "..." }    │                            │
  │◀──────────────────────────────│                            │
```

- **Polling pattern**: Client submits job → gets job_id → polls status until done → download
- **Non-blocking**: Heavy OLAP queries run in background worker, don't block API
- **Production**: Read from DB read replica to avoid impacting write performance

---

### audit-service (:50056) — Append-Only Event Log

```
RabbitMQ ──▶ audit-service ──▶ PostgreSQL (partitioned)

audit_logs table:
  PARTITION BY RANGE (occurred_at)
  ├── audit_logs_2026_01  (January)
  ├── audit_logs_2026_02  (February)
  ├── ...
  └── audit_logs_2026_12  (December)
```

- **Append-only**: No UPDATE or DELETE — immutable audit trail
- **Range partitioning by month**: Query planner auto-prunes partitions for date-range queries
- **7-year retention**: `DROP TABLE audit_logs_2019_01` — instant, no expensive DELETE scan
- **Consumes all events**: task.assigned, task.status_changed — records who did what, when

---

### search-service (:50057) — Full-Text Search

```
RabbitMQ ──▶ search-service ──▶ Elasticsearch

Indexes:
  ├── tasks      (title, description, status, assignee)
  ├── projects   (name, description)
  └── files      (filename, content_type)

Query: GET /search?q=concrete&type=tasks
  → Elasticsearch multi-index query
  → Return ranked results with highlights
```

- **CQRS pattern**: Write path (PostgreSQL) is separate from read/search path (Elasticsearch)
- **Event-driven indexing**: Every task/project change triggers re-index via RabbitMQ
- **Multi-index queries**: Search across tasks, projects, and files in a single request
- **Elasticsearch locally** → **Amazon OpenSearch in production**

---

### scheduler-service (:50058) — Distributed Cron

```
scheduler-service
  │
  ├── Every 1 min: Check overdue tasks
  │   ├── Redis SET NX "cron:deadline_check" TTL=55s (distributed lock)
  │   │   └── Only ONE instance runs the check across all replicas
  │   ├── Query tasks WHERE due_date < NOW() AND status != 'done'
  │   └── Publish "task.overdue" event → RabbitMQ
  │
  └── File lifecycle trigger
      └── Move files between S3 storage tiers based on age
```

- **Distributed cron**: Redis SET NX prevents duplicate execution across multiple instances
- **Leader election via lock**: First instance to acquire lock runs the job, others skip

---

## Observability — Elastic APM

```
All 9 services ──── traces ────▶ APM Server :8200 ──▶ Elasticsearch ──▶ Kibana :5601

Instrumentation:
  ├── Gateway:     apmgin middleware (every HTTP request)
  ├── gRPC:        apmgrpc interceptors (every RPC call)
  ├── RabbitMQ:    custom spans (message publish + consume)
  └── PostgreSQL:  GORM plugin (every DB query)
```

- **Distributed tracing**: A single request from client → gateway → task-service → RabbitMQ → notification-service is traced end-to-end with a single trace ID
- **Service map**: Kibana auto-generates a visual service dependency map from trace data
- **Latency breakdown**: See exactly how much time is spent in each service, DB query, or Redis call
- **Error tracking**: Failed requests, panics, and slow queries are captured automatically
- **Access**: Kibana at `http://localhost:5601` (elastic/changeme)

---

## API Documentation — Swagger / OpenAPI

- Auto-generated from Go annotations using `swaggo`
- Live at `http://localhost:8080/swagger/index.html`
- 13 REST endpoints documented with request/response schemas
- Try-it-out: Execute API calls directly from the browser

```go
// Example annotation on handler:
// @Summary Assign a task to a worker (manager only)
// @Tags tasks
// @Security BearerAuth
// @Router /api/v1/tasks/{id}/assign [post]
```

---

## Protocol Buffers

```
proto/
├── task_service/v1/
│   └── task_service.proto       # Project + Task RPCs
├── user_service/v1/
│   └── user_service.proto       # Register, Login RPCs
└── notification_service/v1/
    └── notification_service.proto  # Notification RPCs

gen/go/proto/                    # Generated Go stubs (buf generate)
```

- **Strongly-typed contracts**: Proto files define the API contract between services — breaking changes are caught at compile time
- **Code generation**: `buf generate` from repo root regenerates all Go stubs
- **~10x lower serialization overhead** vs JSON — critical for high-frequency internal calls

---

## DevOps

### Docker Compose — 16 Containers

```
Infrastructure (7):  postgres, redis, rabbitmq, minio,
                     elasticsearch, kibana, apm-server
Application (9):     gw-gateway, user-service, task-service,
                     notification-service, file-service,
                     report-service, audit-service,
                     search-service, scheduler-service
```

- Health checks on all infrastructure containers
- `depends_on` with `condition: service_healthy` ensures correct startup order
- Named volumes for data persistence across restarts

### Kubernetes — 10 Manifests

```
k8s/
├── namespace.yaml
├── postgres.yaml          # StatefulSet + PVC
├── redis.yaml
├── rabbitmq.yaml
├── user-service.yaml      # Deployment + Service
├── task-service.yaml
├── notification-service.yaml
├── gateway.yaml           # Deployment + Service (NodePort/LoadBalancer)
├── configmap.yaml         # Shared env vars
└── secrets.yaml           # DB passwords, JWT keys
```

- Ready to deploy to EKS with only endpoint changes
- Horizontal pod autoscaling for stateless services

### GitHub Actions CI/CD

```yaml
# .github/workflows/ci.yml
on: push
jobs:
  lint:   golangci-lint run ./...
  test:   go test -race ./...
```

- Runs on every push
- `golangci-lint` catches code quality issues
- `-race` flag detects data races in concurrent code

---

## Testing

| Suite | Service | Key Coverage |
|-------|---------|-------------|
| assign_task | task-service | Distributed lock, concurrent assignment, cross-tenant rejection |
| update_task_status | task-service | All valid/invalid state machine transitions, role-based restrictions |
| create_task | task-service | Validation, project ownership, cross-tenant check |
| login | user-service | Credential validation, JWT issuance, error masking |
| register | user-service | Email uniqueness per company, company create/join flow |

- **gomock**: All domain interfaces mocked — tests don't touch real DB/Redis/RabbitMQ
- **Table-driven**: Each test case is a row in a table — easy to add new cases
- **`-race` flag**: Detects data races at test time
- **Fast**: All tests run in < 1s (no I/O, all mocked)

---

## Infrastructure: Local → AWS Production

| Local (Docker Compose) | AWS Production |
|------------------------|---------------|
| PostgreSQL 16 | Amazon Aurora MySQL |
| Redis 7 | ElastiCache |
| RabbitMQ 3 | Amazon MQ |
| MinIO | S3 |
| Elasticsearch 8 | Amazon OpenSearch |
| Docker Compose | EKS (Kubernetes) |
| Elastic APM | AWS X-Ray / OpenSearch APM |

All infrastructure addresses come from environment variables — zero cloud-provider lock-in.

---

## Known Limitations — Demo vs Production

This project is built as a demo to showcase architecture, patterns, and Go proficiency. Below are the gaps that would need to be addressed before running in production.

### Infrastructure

| Area | Current (Demo) | Production |
|------|---------------|------------|
| Object storage | MinIO (local container) | AWS S3 (endpoint swap via env var — code unchanged) |
| Database | Single PostgreSQL instance | Aurora MySQL with read replicas (report-service OLAP queries on replica) |
| Cache | Single Redis instance | ElastiCache cluster with Redis Sentinel/Cluster for HA |
| Message broker | Single RabbitMQ instance | Amazon MQ or RabbitMQ cluster (quorum queues for durability) |
| Search | Single Elasticsearch node | Amazon OpenSearch (multi-node, dedicated master) |
| Deployment | Docker Compose on local | EKS cluster with HPA, PDB, resource limits |
| Secrets | Hardcoded in docker-compose env | AWS Secrets Manager / K8s external-secrets-operator |
| TLS | None (plain HTTP/gRPC) | TLS termination at ALB/Ingress, mTLS between services |

### Application

| Area | Current (Demo) | Production Fix |
|------|---------------|---------------|
| **Casbin RBAC policy** | CSV file, loaded at startup | `gorm-adapter` — store policies in PostgreSQL, add management API for runtime CRUD |
| **JWT refresh token** | No refresh token — access token only | Add `POST /auth/refresh` with rotating refresh tokens, short-lived access tokens (15min) |
| **Dual-write risk** | DB write + RabbitMQ publish in same handler — publish failure = lost event | Transactional outbox pattern: write `outbox` table in same DB tx, background worker polls and publishes |
| **Redis lock TTL race** | `defer Del(lockKey)` may delete another goroutine's lock if TTL expires first | Lua script atomic check-and-delete: only delete if value matches unique lock token |
| **Rate limiting algorithm** | Fixed window counter (comment says sliding window but it's not) | Token bucket via Lua script — atomic INCR+check+EXPIRE, allows controlled burst |
| **Password validation** | Min 6 characters only | Add complexity rules (uppercase, number, special char), or integrate zxcvbn |
| **Email verification** | None — any email can register | Send verification email with token, activate account on confirm |
| **Graceful shutdown** | Not implemented | `signal.NotifyContext` + gRPC graceful stop + drain RabbitMQ consumers before exit |
| **Circuit breaker** | No circuit breaker on gRPC calls | Add `go-kit` or `sony/gobreaker` between gateway and downstream services |
| **Request validation** | Basic struct binding only | Add deeper business validation (e.g., due_date must be future, priority enum check) |
| **Pagination** | Offset-based (`LIMIT/OFFSET`) | Cursor-based pagination for large datasets (avoid `OFFSET` performance degradation) |

### Observability & Operations

| Area | Current (Demo) | Production Fix |
|------|---------------|---------------|
| APM setup | Manual Fleet + integration install in Kibana | Automated via docker-compose init script or Terraform |
| Structured logging | `gin.Logger()` (text format) | JSON structured logs with trace_id correlation (ELK/CloudWatch) |
| Health checks | `/health` returns 200 OK only | Deep health checks: verify DB, Redis, RabbitMQ connectivity per service |
| Alerting | None | PagerDuty/OpsGenie alerts on error rate, latency P99, queue depth |
| DB migrations | GORM AutoMigrate at startup | Versioned migrations with `golang-migrate` or `atlas`, run separately from app startup |

### Testing

| Area | Current (Demo) | Production Fix |
|------|---------------|---------------|
| Unit tests | 5 suites (use-case layer only) | Add repository integration tests, gateway handler tests |
| Integration tests | Manual E2E via Swagger | Automated integration tests with `testcontainers-go` (real DB/Redis/RabbitMQ) |
| Load testing | None | k6 or Locust scripts for performance benchmarks |
| Contract tests | None | Protobuf backward compatibility checks in CI (`buf breaking`) |

### Security Hardening

| Area | Current (Demo) | Production Fix |
|------|---------------|---------------|
| CORS | Not configured | Whitelist allowed origins |
| Request size limit | No limit | `gin.MaxMultipartMemory` + request body size middleware |
| SQL injection | GORM parameterized (safe) | Already safe — no raw SQL |
| Input sanitization | None | Sanitize HTML/script in user-provided strings (task title, description) |
| Audit log tampering | Append-only table but DB admin can still modify | Write-once storage (S3 Object Lock) or blockchain-anchored hashes |
| API versioning | `/api/v1` prefix only | Version negotiation strategy for breaking changes |

---

## Tech Stack

| Category | Technology |
|----------|-----------|
| Language | Go 1.23+ |
| HTTP Framework | Gin |
| Internal Communication | gRPC + Protocol Buffers |
| ORM | GORM |
| Auth | JWT RS256 (golang-jwt/v5) |
| Authorization | Casbin v2 |
| Message Broker | RabbitMQ (direct exchange) |
| Database | PostgreSQL 16 |
| Cache / Lock | Redis 7 |
| Search | Elasticsearch 8 |
| Object Storage | MinIO (S3-compatible) |
| Observability | Elastic APM + Kibana |
| API Docs | Swagger / OpenAPI (swaggo) |
| Proto Tooling | buf |
| Testing | testify + gomock, table-driven, `-race` |
| CI/CD | GitHub Actions (golangci-lint + go test -race) |
| Containers | Docker Compose (dev), Kubernetes (prod) |
