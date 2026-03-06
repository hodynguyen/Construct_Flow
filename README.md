# ConstructFlow

Multi-tenant construction task management system — Go microservices backend.

---

## Architecture

```
Client
  │
  ▼
gw-gateway :8080  (JWT RS256 auth, Casbin RBAC, Redis rate limit)
  │
  ├── /auth/*          → user-service :50053
  ├── /projects/*      → task-service :50051
  ├── /tasks/*         → task-service :50051
  ├── /notifications/* → notification-service :50052
  ├── /files/*         → file-service :50054         [extended]
  ├── /reports/*       → report-service :50055       [extended]
  ├── /audit/*         → audit-service :50056        [extended]
  ├── /search/*        → search-service :50057       [extended]
  └── /scheduler/*     → scheduler-service :50058    [extended]

RabbitMQ — constructflow.events (fanout exchange)
  task.assigned        → notification-service, audit-service
  task.status_changed  → notification-service, audit-service
  task.created         → audit-service, search-service (index)
  project.created      → audit-service, search-service (index)
  file.uploaded        → audit-service, search-service (index metadata)
  report.requested     → report-service
  report.completed     → notification-service
  deadline.reminder    → notification-service
  deadline.overdue     → notification-service, audit-service
```

---

## Services

| Service | Port | Role | Key Pattern |
|---------|------|------|-------------|
| `gw-gateway` | :8080 | HTTP gateway | JWT RS256, Casbin RBAC, rate limit |
| `task-service` | :50051 | Projects + Tasks | State machine, distributed lock |
| `user-service` | :50053 | Auth | bcrypt, JWT generation |
| `notification-service` | :50052 | Real-time alerts | RabbitMQ consumer, idempotency |
| `file-service` | :50054 | Document/photo storage | S3 presigned URL, tiered storage |
| `report-service` | :50055 | Async report generation | Job queue, status polling |
| `audit-service` | :50056 | Compliance audit trail | Append-only, event fan-out |
| `search-service` | :50057 | Full-text search | CQRS, Elasticsearch indexing |
| `scheduler-service` | :50058 | Automated jobs | Distributed cron, deadline watch |

**Infrastructure:** PostgreSQL 16, Redis 7, RabbitMQ 3, MinIO (S3), Elasticsearch, Elastic APM

---

## Quick Start

### Prerequisites

- Docker + Docker Compose
- openssl (one-time key generation)

### 1. Generate RSA key pair

```bash
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
```

### 2. Start the stack

```bash
# Core services (Days 1-6)
docker-compose up --build

# Full stack including extended services (Days 7-8)
docker-compose -f docker-compose.yml -f docker-compose.extended.yml up --build
```

Services:
- REST API: http://localhost:8080
- Swagger UI: http://localhost:8080/swagger/index.html
- RabbitMQ Management: http://localhost:15672 (guest/guest)
- MinIO Console: http://localhost:9001 (minioadmin/minioadmin)
- Kibana APM: http://localhost:5601

### 3. Sample curl commands

```bash
BASE=http://localhost:8080/api/v1

# Register + Login
curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@acme.com","name":"Admin","password":"pass123","role":"admin","company_name":"Acme Corp"}'

TOKEN=$(curl -s -X POST $BASE/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@acme.com","password":"pass123"}' | jq -r '.access_token')

# Create project + task
PROJECT_ID=$(curl -s -X POST $BASE/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Site Alpha","description":"Main construction site"}' | jq -r '.project.id')

TASK_ID=$(curl -s -X POST $BASE/projects/$PROJECT_ID/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Install plumbing","priority":"high"}' | jq -r '.task.id')

# Assign task → triggers notification + audit event
curl -s -X POST $BASE/tasks/$TASK_ID/assign \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"assigned_to":"<worker_user_id>"}'

# Upload a file to task (presigned URL flow)
PRESIGN=$(curl -s -X POST $BASE/files/presign \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"task_id\":\"$TASK_ID\",\"filename\":\"blueprint.pdf\",\"mime_type\":\"application/pdf\"}")
FILE_ID=$(echo $PRESIGN | jq -r '.file_id')
UPLOAD_URL=$(echo $PRESIGN | jq -r '.upload_url')
# Upload directly to MinIO/S3
curl -s -X PUT "$UPLOAD_URL" --data-binary @blueprint.pdf
# Confirm upload
curl -s -X POST $BASE/files/$FILE_ID/confirm -H "Authorization: Bearer $TOKEN"

# Search across tasks + files
curl -s "$BASE/search?q=plumbing&types=task,file" -H "Authorization: Bearer $TOKEN"

# Request async report
JOB_ID=$(curl -s -X POST $BASE/reports/generate \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"type\":\"weekly_progress\",\"project_id\":\"$PROJECT_ID\"}" | jq -r '.job_id')
# Poll status
curl -s $BASE/reports/$JOB_ID/status -H "Authorization: Bearer $TOKEN"

# Audit trail
curl -s "$BASE/audit?resource=task&resource_id=$TASK_ID" -H "Authorization: Bearer $TOKEN"
```

---

## Key Design Decisions

### Task Status State Machine
```
todo → in_progress → done
         ↕
       blocked

done → in_progress  (manager/admin only — rework)
```
Enforced in use-case layer. Invalid transition → `400 Bad Request`.

### Distributed Lock for Assignment
`SET NX lock:task:assign:<taskID>` 5s TTL in Redis. Prevents concurrent double-assignment.

### Event Idempotency
`SET NX event:<event_id>` (24h TTL) before processing any RabbitMQ message. Retry 3x with exponential backoff (1s → 2s → 4s) → DLQ.

### S3 Tiered Storage (file-service)
```
Standard    → active projects (frequent access)
Standard-IA → completed > 6 months (scheduler migrates, -40% cost)
Glacier     → archived > 2 years (scheduler migrates, -95% cost)
```
Local dev: MinIO replaces S3 (100% API compatible).

### Async Report Generation (report-service)
`POST /reports/generate` → `202 Accepted { job_id }` → poll `GET /reports/:id/status`.
Report queries run on read-replica PostgreSQL — no OLTP impact.

### Append-Only Audit Log (audit-service)
Fan-out from all events. Never UPDATE or DELETE. PostgreSQL table partitioned by month.
Retention: 7 years (construction regulatory requirement).

### CQRS for Search (search-service)
Write path: event-driven Elasticsearch indexing.
Read path: multi-index search across tasks, projects, files.

### Distributed Cron (scheduler-service)
Redis `SET NX cron:<job> TTL` ensures only 1 pod runs each job in multi-replica deploy.

### Multi-Tenancy
`company_id` on every DB table + JWT claim. Every query: `WHERE company_id = $1`.

### RBAC
Casbin domain-scoped policies. Roles: `admin`, `manager`, `worker`.

| Role | Projects | Tasks | Files | Reports | Audit |
|------|----------|-------|-------|---------|-------|
| admin | CRUD | CRUD | CRUD | full | read |
| manager | CRUD | CRUD | upload/read | request/read | read |
| worker | read | read (assigned) | upload/read | — | — |

---

## AWS Production Mapping

| Local | AWS Equivalent |
|-------|----------------|
| PostgreSQL (Docker) | Amazon RDS Aurora |
| Read replica (Docker) | RDS Read Replica |
| Redis (Docker) | Amazon ElastiCache |
| RabbitMQ (Docker) | Amazon MQ or SQS+SNS |
| MinIO (Docker) | Amazon S3 (+ Lifecycle Policies) |
| Elasticsearch (Docker) | Amazon OpenSearch |
| Elastic APM | AWS X-Ray |
| Docker Compose | Amazon ECS (Fargate) or EKS |
| K8s CronJob | AWS EventBridge Scheduler |
| keys/ | AWS Secrets Manager |
| GitHub Actions | AWS CodePipeline + CodeBuild |

---

## Development

```bash
# Run all tests
cd apps/task-service && go test ./... -race

# Lint
golangci-lint run ./...

# Regenerate protobuf stubs
buf generate

# Regenerate mocks
go generate ./...

# Regenerate Swagger docs
cd apps/gw-gateway && swag init --generalInfo main.go --output ./docs
```

---

## Project Structure

```
apps/
├── gw-gateway/            # HTTP :8080
├── task-service/          # gRPC :50051
├── user-service/          # gRPC :50053
├── notification-service/  # gRPC :50052
├── file-service/          # gRPC :50054  [extended]
├── report-service/        # gRPC :50055  [extended]
├── audit-service/         # gRPC :50056  [extended]
├── search-service/        # gRPC :50057  [extended]
└── scheduler-service/     # gRPC :50058  [extended]
gen/go/                    # Generated proto stubs
proto/                     # .proto definitions
k8s/                       # Kubernetes manifests
scripts/
├── init.sql               # PostgreSQL schema
└── init-rabbitmq.sh       # Exchange + queue setup
```

Each service: `Controller → Use Case → Domain interfaces ← Repository`.
Domain layer: zero external imports.
