# ConstructFlow

Multi-tenant construction task management system — built as a Go microservices backend.

---

## Architecture

```
Client → gw-gateway (HTTP :8080, Gin) → task-service        (gRPC :50051)
                                       → user-service        (gRPC :50053)
                                       → notification-service (gRPC :50052)

task-service ──► RabbitMQ ──► notification-service
                (task.assigned / task.status_changed)
```

| Service | Role |
|---------|------|
| `gw-gateway` | JWT RS256 auth, Casbin RBAC, Redis rate limiting, REST routing |
| `task-service` | Projects + tasks CRUD, assignment (Redis distributed lock), status state machine, domain event publishing |
| `user-service` | Registration (bcrypt), login, JWT generation |
| `notification-service` | RabbitMQ consumer, idempotent notification store, mark-read API |

**Infrastructure:** PostgreSQL 16, Redis 7, RabbitMQ 3, Elastic APM (Kibana :5601)

---

## Quick Start

### Prerequisites

- Docker + Docker Compose
- openssl (for key generation, one-time setup)

### 1. Generate RSA key pair (first time only)

```bash
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
```

### 2. Start the full stack

```bash
docker-compose up --build
```

Services available:
- REST API: http://localhost:8080
- Swagger UI: http://localhost:8080/swagger/index.html
- RabbitMQ Management: http://localhost:15672 (guest/guest)
- Kibana APM: http://localhost:5601

### 3. Sample curl commands

```bash
BASE=http://localhost:8080/api/v1

# Register (creates company automatically if company_name provided)
curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@acme.com","name":"Admin","password":"pass123","role":"admin","company_name":"Acme Corp"}'

# Login — grab token
TOKEN=$(curl -s -X POST $BASE/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@acme.com","password":"pass123"}' | jq -r '.access_token')

# Create project
PROJECT_ID=$(curl -s -X POST $BASE/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Site Alpha","description":"Main construction site"}' | jq -r '.project.id')

# Create task
TASK_ID=$(curl -s -X POST $BASE/projects/$PROJECT_ID/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Install plumbing","priority":"high"}' | jq -r '.task.id')

# Assign task (triggers RabbitMQ → notification-service)
curl -s -X POST $BASE/tasks/$TASK_ID/assign \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"assigned_to":"<worker_user_id>"}'

# Check notifications (as assigned worker)
curl -s $BASE/notifications -H "Authorization: Bearer $WORKER_TOKEN"

# Update task status
curl -s -X PATCH $BASE/tasks/$TASK_ID/status \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}'
```

---

## Key Design Decisions

### Task Status State Machine

```
todo → in_progress → done
         ↕
       blocked

done → in_progress  (manager/admin only, for rework)
```

Enforced in the use-case layer — invalid transitions return `400 Bad Request`.

### Distributed Lock for Assignment

`SET NX lock:task:assign:<taskID>` with 5s TTL in Redis prevents concurrent double-assignment races.

### Event Idempotency

Notification-service checks `SET NX event:<event_id>` (24h TTL) before processing any RabbitMQ message. Duplicate deliveries are silently ACK'd. Failed messages retry 3x with exponential backoff (1s → 2s → 4s), then route to DLQ.

### Multi-Tenancy

Every DB table has `company_id`. JWT claims carry `company_id` + `role`. Every repository query scopes to `WHERE company_id = ?`. Cross-tenant access is structurally impossible.

### RBAC

Casbin with domain-scoped policies (`sub, company_id, resource, action`). Roles: `admin`, `manager`, `worker`.

| Role | Projects | Tasks | Assign | Status Change |
|------|----------|-------|--------|---------------|
| admin | CRUD | CRUD | ✓ | ✓ |
| manager | CRUD | CRUD | ✓ | ✓ |
| worker | read | read | ✗ | ✓ (own tasks) |

---

## AWS Production Mapping

| Local | AWS Equivalent |
|-------|----------------|
| PostgreSQL (Docker) | Amazon RDS (Aurora MySQL or PostgreSQL) |
| Redis (Docker) | Amazon ElastiCache for Redis |
| RabbitMQ (Docker) | Amazon MQ (RabbitMQ) or SQS + SNS |
| Docker Compose | Amazon ECS (Fargate) or EKS |
| keys/ volume | AWS Secrets Manager |
| Elastic APM | AWS X-Ray |
| GitHub Actions CI | AWS CodePipeline + CodeBuild |

---

## Development

```bash
# Run all tests
cd apps/task-service && go test ./... -race

# Lint
golangci-lint run ./...

# Regenerate protobuf stubs (run from repo root)
buf generate

# Regenerate mocks (from service directory)
go generate ./...

# Regenerate Swagger docs (from gw-gateway directory)
swag init --generalInfo main.go --output ./docs
```

---

## Project Structure

```
apps/
├── gw-gateway/           # HTTP :8080  — JWT auth, Casbin RBAC, rate limit
├── task-service/         # gRPC :50051 — projects, tasks, state machine
├── user-service/         # gRPC :50053 — register, login, JWT
└── notification-service/ # gRPC :50052 — consume events, store, mark-read
gen/go/                   # Generated protobuf stubs (buf generate)
proto/                    # .proto definitions
k8s/                      # Kubernetes manifests
scripts/init.sql          # PostgreSQL schema
```

Each service follows clean architecture — dependencies point inward:

```
Controller (gRPC/HTTP) → Use Case → Domain interfaces ← Repository (GORM)
```

The domain layer has **zero** external package imports.
