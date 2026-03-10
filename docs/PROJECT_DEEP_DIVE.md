# ConstructFlow — Complete Project Deep Dive

> This document contains everything about the ConstructFlow project: architecture, code structure, every service, every API, database schema, infrastructure, patterns, config, and deployment. Intended as a comprehensive reference for anyone who needs to fully understand the system.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Tech Stack](#2-tech-stack)
3. [System Architecture](#3-system-architecture)
4. [Service Details](#4-service-details)
5. [REST API Endpoints](#5-rest-api-endpoints)
6. [gRPC Contracts (Proto)](#6-grpc-contracts-proto)
7. [Database Schema](#7-database-schema)
8. [Authentication & Authorization](#8-authentication--authorization)
9. [Task State Machine](#9-task-state-machine)
10. [Event-Driven Architecture (RabbitMQ)](#10-event-driven-architecture-rabbitmq)
11. [Key Design Patterns](#11-key-design-patterns)
12. [Clean Architecture](#12-clean-architecture)
13. [Multi-Tenancy](#13-multi-tenancy)
14. [Infrastructure & Config](#14-infrastructure--config)
15. [Docker Compose](#15-docker-compose)
16. [Kubernetes](#16-kubernetes)
17. [CI/CD](#17-cicd)
18. [Testing](#18-testing)
19. [Observability (Elastic APM)](#19-observability-elastic-apm)
20. [Known Limitations — Demo vs Production](#20-known-limitations--demo-vs-production)
21. [How to Run](#21-how-to-run)
22. [E2E Test Flow](#22-e2e-test-flow)

---

## 1. Problem Statement

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

## 2. Tech Stack

| Category | Technology |
|----------|-----------|
| Language | Go 1.23+ |
| HTTP Framework | Gin |
| Internal Communication | gRPC + Protocol Buffers |
| ORM | GORM |
| Auth | JWT RS256 (golang-jwt/v5) |
| Authorization | Casbin v2 |
| Message Broker | RabbitMQ 3 (direct exchange, durable queues) |
| Database | PostgreSQL 16 |
| Cache / Distributed Lock | Redis 7 |
| Search | Elasticsearch 8 |
| Object Storage | MinIO (S3-compatible) |
| Observability | Elastic APM + Kibana |
| API Docs | Swagger / OpenAPI (swaggo, auto-generated) |
| Proto Tooling | buf |
| Testing | testify + gomock, table-driven, `-race` |
| CI/CD | GitHub Actions (golangci-lint + go test -race) |
| Containers | Docker Compose (dev), Kubernetes (prod) |
| Workspace | Go workspace (`go.work` with 10 modules) |

---

## 3. System Architecture

### Overview Diagram

```
                              ┌──────────────────────────────────────┐
                              │         gw-gateway :8080             │
                              │                                      │
  Client ──── REST/JSON ────▶ │  Rate Limit ──▶ JWT Auth ──▶ RBAC   │
  (Browser/Mobile)            │       │            │           │     │
                              │       ▼            ▼           ▼     │
                              │              Gin Router              │
                              └──────┬───┬───┬───┬───┬───────────────┘
                                     │   │   │   │   │
                          gRPC calls │   │   │   │   │
                    ┌────────────────┘   │   │   │   └──────────────┐
                    ▼                    ▼   │   ▼                  ▼
            ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
            │ user-service │  │ task-service  │  │ notification │  │ file-service │
            │    :50053    │  │    :50051     │  │   -service   │  │    :50054    │
            └──────┬───────┘  └──┬───┬───┬───┘  │    :50052    │  └──────┬───────┘
                   │             │   │   │      └──────┬───────┘         │
                   │             │   │   │             │                 │
                   ▼             ▼   │   ▼             ▼                 ▼
              PostgreSQL    PostgreSQL│  Redis      PostgreSQL        MinIO (S3)
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

### Services Summary

| Service | Port | Protocol | Responsibility | Infrastructure |
|---------|------|----------|---------------|----------------|
| gw-gateway | :8080 | HTTP (Gin) | Rate limit, JWT auth, RBAC, REST routing, Swagger UI | Redis |
| user-service | :50053 | gRPC | Register, login, JWT signing (RS256 private key) | PostgreSQL |
| task-service | :50051 | gRPC | Project/task CRUD, assignment, status state machine, event publishing | PostgreSQL, Redis, RabbitMQ |
| notification-service | :50052 | gRPC | Consume events, idempotency check, notification store, mark-read | PostgreSQL, Redis, RabbitMQ |
| file-service | :50054 | gRPC | File upload via S3 presigned URLs, tiered storage lifecycle | PostgreSQL, MinIO |
| report-service | :50055 | gRPC | Async report generation, job queue, status polling, output to S3 | PostgreSQL, RabbitMQ, MinIO |
| audit-service | :50056 | gRPC | Append-only event log, partitioned by month, 7-year retention | PostgreSQL, RabbitMQ |
| search-service | :50057 | gRPC | CQRS full-text search, event-driven Elasticsearch indexing | Elasticsearch, RabbitMQ |
| scheduler-service | :50058 | gRPC | Distributed cron (Redis lock), deadline watch, file lifecycle | Redis, RabbitMQ |

---

## 4. Service Details

### 4.1 gw-gateway (:8080) — API Gateway

**Role**: Single entry point for all client requests. No business logic.

**Middleware pipeline** (executed in order for every request):
1. `apmgin.Middleware` — Elastic APM tracing (creates trace span)
2. `gin.Recovery()` — Panic recovery (returns 500 instead of crashing)
3. `gin.Logger()` — Request logging
4. `RateLimitMiddleware` — Redis INCR per client IP, configurable RPM (default 60)
5. For protected routes:
   - `AuthMiddleware` — Verify JWT RS256 with public key, extract claims (user_id, company_id, role) into Gin context
   - `RBACMiddleware` — Casbin enforce `(role, company_id, resource, action)`

**Dependency injection** (main.go):
```go
router := gwhttp.NewRouter(gwhttp.RouterConfig{
    UserClient:       userv1.NewUserServiceClient(grpcClients.UserServiceConn),
    TaskClient:       taskv1.NewTaskServiceClient(grpcClients.TaskServiceConn),
    NotifClient:      notifv1.NewNotificationServiceClient(grpcClients.NotificationServiceConn),
    RedisClient:      redisClient,
    Enforcer:         enforcer,          // Casbin
    RateLimitRPM:     cfg.RateLimitRequestsPerMinute,
    JWTPublicKeyPath: cfg.JWTPublicKeyPath,
})
```

**Config** (env vars):
- `HTTP_PORT` (default 8080)
- `REDIS_ADDR` — for rate limiting
- `TASK_SERVICE_ADDR`, `USER_SERVICE_ADDR`, `NOTIFICATION_SERVICE_ADDR` — gRPC backends
- `JWT_PUBLIC_KEY_PATH` — RS256 public key (verify only, cannot sign)
- `CASBIN_MODEL_PATH`, `CASBIN_POLICY_PATH`
- `RATE_LIMIT_RPM` (default 60)
- `ELASTIC_APM_SERVER_URL`, `ELASTIC_APM_SERVICE_NAME`

---

### 4.2 user-service (:50053) — Authentication

**gRPC RPCs**: `Register`, `Login`, `GetUser`

**Register flow**:
1. Validate: email (required, format), name (required), password (required, min 6)
2. `role` optional — default `"worker"`. Valid values: `admin`, `manager`, `worker`
3. Must provide EITHER `company_id` (join existing) OR `company_name` (create new)
4. Check email unique within company: `UNIQUE(email, company_id)` — same email in different companies = OK
5. Hash password with bcrypt cost=12 (~280ms)
6. INSERT user, return user object + company_id

**Login flow**:
1. `FindByEmail` — not found → "invalid credentials" (does NOT leak whether email exists)
2. `bcrypt.CompareHashAndPassword` — wrong password → same "invalid credentials" message
3. Generate JWT RS256 token with claims: `{user_id, company_id, role}`
4. Sign with **private key** (only user-service has it)
5. Return `{access_token, user}`

**JWT Claims structure**:
```go
type jwtClaims struct {
    jwt.RegisteredClaims
    UserID    string `json:"user_id"`
    CompanyID string `json:"company_id"`
    Role      string `json:"role"`
}
```

**Config**: `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `GRPC_PORT`, `JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`

---

### 4.3 task-service (:50051) — Core Business Logic

**gRPC RPCs**:
- Project: `CreateProject`, `GetProject`, `ListProjects`, `UpdateProject`, `DeleteProject`
- Task: `CreateTask`, `GetTask`, `ListTasks`, `UpdateTask`, `DeleteTask`, `AssignTask`, `UpdateTaskStatus`

**Domain interfaces** (zero external imports):
```go
type TaskRepository interface {
    Create(ctx, task) error
    FindByID(ctx, companyID, taskID) (*Task, error)
    Update(ctx, task) error
    Delete(ctx, companyID, taskID) error
    ListByProject(ctx, companyID, projectID, filter, page, pageSize) ([]Task, int64, error)
}

type ProjectRepository interface {
    Create, FindByID, Update, Delete, ListByCompany
}

type UserRepository interface {
    FindByID(ctx, companyID, userID) (*User, error)  // read-only in task-service
}

type EventPublisher interface {
    Publish(ctx, eventType string, payload interface{}) error
}

type LockClient interface {
    SetNX(ctx, key string, value interface{}, expiration) (bool, error)
    Del(ctx, keys...) error
}
```

**Use Cases**:

**CreateTask**:
- Validate title required
- Verify project exists AND belongs to same company (cross-tenant check)
- Default status=`"todo"`, priority=`"medium"`
- Parse due_date (YYYY-MM-DD) if provided
- INSERT and return

**AssignTask** (most complex — see Section 11 for full flow):
- Acquire Redis distributed lock `lock:task:assign:<taskID>` (SET NX, TTL=5s)
- Load task by ID + company_id
- Validate assignee exists in same company (cross-tenant protection)
- UPDATE task SET assigned_to
- Publish `task.assigned` event to RabbitMQ (non-fatal — task saved even if publish fails)
- defer release lock

**UpdateTaskStatus** (state machine — see Section 9):
- Load task by ID + company_id
- Validate transition is allowed (hardcoded map)
- Check role restriction for `done → in_progress` (manager/admin only)
- UPDATE task SET status
- Publish `task.status_changed` event

**CreateProject**:
- Validate name required
- Parse start_date, end_date (YYYY-MM-DD, optional)
- Default status=`"active"`
- INSERT and return

**Dependency injection** (main.go):
```go
taskRepo    := sqlrepo.NewTaskRepository(db)
projectRepo := sqlrepo.NewProjectRepository(db)
userRepo    := sqlrepo.NewUserRepository(db)
lockClient  := service.NewRedisLockClient(redisClient)
publisher   := service.NewRabbitMQPublisher(mq.Channel)

createProjectUC := create_project.New(projectRepo)
createTaskUC    := create_task.New(taskRepo, projectRepo)
assignTaskUC    := assign_task.New(taskRepo, userRepo, publisher, lockClient)
updateStatusUC  := update_task_status.New(taskRepo, publisher)
```

**RabbitMQ setup** (bootstrap/rabbitmq.go):
- Exchange: `constructflow.events` (direct, durable)
- Queues: `task_assigned_queue`, `task_status_changed_queue`
- Routing keys: `task.assigned`, `task.status_changed`

---

### 4.4 notification-service (:50052) — Real-time Notifications

**gRPC RPCs**: `GetNotifications`, `GetUnreadCount`, `MarkAsRead`, `MarkAllAsRead`

**RabbitMQ consumers** (started as goroutines in main.go):
- `TaskAssignedConsumer` — listens on `task_assigned_queue`
- `TaskStatusChangedConsumer` — listens on `task_status_changed_queue`

**Event processing flow**:
1. Receive message from RabbitMQ
2. **Idempotency check**: Redis `SET NX event:<event_id>` with 24h TTL
   - Already processed → ACK and skip
3. Parse event payload
4. Create notification record in PostgreSQL
5. ACK message

**Retry & DLQ**:
- 3 retries with exponential backoff (1s → 2s → 4s)
- After 3 failures → message goes to `<queue>.dlq`

**Graceful shutdown** (notification-service has it, unlike other services):
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    cancel()                    // cancel consumer context
    grpcServer.GracefulStop()   // drain in-flight RPCs
}()
```

---

### 4.5 file-service (:50054) — File Management

**Pattern**: Presigned URL upload — client uploads directly to S3/MinIO, file never passes through the service.

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
  │──────────────────────────────────────────────────────────────▶│
  │                               │                              │
  │  POST /files/confirm          │                              │
  │  { file_id }                  │                              │
  │──────────────────────────────▶│  INSERT files (S3 key ref)   │
  │    200 OK                     │                              │
  │◀──────────────────────────────│                              │
```

**Tiered storage**: Standard → Standard-IA (30 days) → Glacier (90 days) via S3 lifecycle rules.

**MinIO locally** → **S3 in production** (same API, endpoint swap via env var `S3_ENDPOINT`).

**Config**: `S3_ENDPOINT`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_BUCKET_NAME`, `S3_USE_SSL`

---

### 4.6 report-service (:50055) — Async Report Generation

```
Client                       report-service                 MinIO (S3)
  │                               │                            │
  │  POST /reports                │                            │
  │  { type: "project_summary" }  │                            │
  │──────────────────────────────▶│                            │
  │    { job_id, status: "pending" }                           │
  │◀──────────────────────────────│                            │
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

**Polling pattern**: Client submits job → gets job_id → polls until done → download via presigned URL.
**Production**: Read from DB read replica for OLAP queries — no impact on OLTP.

---

### 4.7 audit-service (:50056) — Compliance Audit Trail

- Consumes **all events** from RabbitMQ (task.assigned, task.status_changed, etc.)
- **Append-only**: No UPDATE or DELETE — immutable audit trail
- **Range partitioning by month**: Query planner auto-prunes partitions for date-range queries
- **7-year retention**: `DROP TABLE audit_logs_2019_01` — instant, no expensive DELETE scan

---

### 4.8 search-service (:50057) — Full-Text Search

- **CQRS pattern**: Write path (PostgreSQL via events) is separate from read/search path (Elasticsearch)
- **Event-driven indexing**: Every task/project change triggers re-index via RabbitMQ event
- Multi-index queries: Search across tasks, projects, and files in a single request
- **Elasticsearch locally** → **Amazon OpenSearch in production**

Indexes:
- `tasks` (title, description, status, assignee)
- `projects` (name, description)
- `files` (filename, content_type)

---

### 4.9 scheduler-service (:50058) — Distributed Cron

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

---

## 5. REST API Endpoints

All routes go through gw-gateway (:8080).

### Public (no auth)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/v1/auth/register` | AuthHandler.Register | Register a new user |
| POST | `/api/v1/auth/login` | AuthHandler.Login | Login, get JWT |
| GET | `/health` | - | Health check (returns `{"status":"ok"}`) |
| GET | `/swagger/*` | - | Swagger UI |

### Protected (JWT + RBAC required)

| Method | Path | RBAC Resource/Action | Description | Roles |
|--------|------|---------------------|-------------|-------|
| GET | `/api/v1/projects` | `/projects` read | List projects (paginated) | All |
| POST | `/api/v1/projects` | `/projects` write | Create project | Admin, Manager |
| GET | `/api/v1/projects/:id` | `/projects` read | Get project by ID | All |
| PUT | `/api/v1/projects/:id` | `/projects` write | Update project | Admin, Manager |
| DELETE | `/api/v1/projects/:id` | `/projects` write | Soft-delete project | Admin, Manager |
| GET | `/api/v1/projects/:id/tasks` | `/tasks` read | List tasks in project (paginated, filterable) | All |
| POST | `/api/v1/projects/:id/tasks` | `/tasks` write | Create task in project | Admin, Manager |
| GET | `/api/v1/tasks/:id` | `/tasks` read | Get task by ID | All |
| POST | `/api/v1/tasks/:id/assign` | `/tasks/assign` write | Assign task to user | Admin, Manager |
| PATCH | `/api/v1/tasks/:id/status` | `/tasks/status` write | Update task status | All (state machine restricts further) |
| GET | `/api/v1/notifications` | `/notifications` read | List notifications (paginated) | All |
| GET | `/api/v1/notifications/unread/count` | `/notifications` read | Get unread count | All |
| PATCH | `/api/v1/notifications/:id/read` | `/notifications` write | Mark notification as read | All |

### Query Parameters (List endpoints)

| Endpoint | Param | Type | Description |
|----------|-------|------|-------------|
| List Projects | `page` | int | Page number (default 1) |
| | `page_size` | int | Items per page (default 20, max 100) |
| | `status` | string | Filter by status |
| List Tasks | `page`, `page_size` | int | Pagination |
| | `status` | string | Filter: todo, in_progress, done, blocked |
| | `assigned_to` | string | Filter by user ID |
| | `priority` | string | Filter: low, medium, high, critical |
| List Notifications | `page`, `page_size` | int | Pagination |
| | `unread_only` | bool | Only unread notifications |

### Request/Response Examples

**Register**:
```json
// POST /api/v1/auth/register
// Request:
{
  "email": "admin@acme.com",
  "name": "Alice Admin",
  "password": "secret123",
  "role": "admin",
  "company_name": "Acme Construction"
}
// OR join existing:
{
  "email": "worker@acme.com",
  "name": "Bob Worker",
  "password": "secret123",
  "role": "worker",
  "company_id": "uuid-of-existing-company"
}

// Response 201:
{
  "user": { "id": "...", "company_id": "...", "email": "...", "name": "...", "role": "admin" },
  "company_id": "..."
}
```

**Login**:
```json
// POST /api/v1/auth/login
// Request:
{ "email": "admin@acme.com", "password": "secret123" }

// Response 200:
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "user": { "id": "...", "company_id": "...", "email": "...", "name": "...", "role": "admin" }
}
```

**Create Project**:
```json
// POST /api/v1/projects  (Authorization: Bearer <token>)
// Request:
{ "name": "Tower B Construction", "description": "Phase 2 office tower" }

// Response 201:
{ "project": { "id": "...", "company_id": "...", "name": "...", "status": "active", ... } }
```

**Create Task**:
```json
// POST /api/v1/projects/{project_id}/tasks
// Request:
{ "title": "Pour concrete floor 3", "description": "Mix 1:2:3", "priority": "high", "due_date": "2026-04-01" }

// Response 201:
{ "task": { "id": "...", "project_id": "...", "status": "todo", "priority": "high", ... } }
```

**Assign Task**:
```json
// POST /api/v1/tasks/{task_id}/assign
// Request:
{ "assigned_to": "<user_id>" }

// Response 200:
{ "task": { "id": "...", "assigned_to": "<user_id>", ... } }
```

**Update Status**:
```json
// PATCH /api/v1/tasks/{task_id}/status
// Request:
{ "status": "in_progress" }

// Response 200:
{ "task": { "id": "...", "status": "in_progress", ... } }
```

---

## 6. gRPC Contracts (Proto)

### task_service.proto

```protobuf
service TaskService {
  rpc CreateProject(CreateProjectRequest) returns (ProjectResponse);
  rpc GetProject(GetProjectRequest)       returns (ProjectResponse);
  rpc ListProjects(ListProjectsRequest)   returns (ListProjectsResponse);
  rpc UpdateProject(UpdateProjectRequest) returns (ProjectResponse);
  rpc DeleteProject(DeleteProjectRequest) returns (google.protobuf.Empty);

  rpc CreateTask(CreateTaskRequest)             returns (TaskResponse);
  rpc GetTask(GetTaskRequest)                   returns (TaskResponse);
  rpc ListTasks(ListTasksRequest)               returns (ListTasksResponse);
  rpc UpdateTask(UpdateTaskRequest)             returns (TaskResponse);
  rpc DeleteTask(DeleteTaskRequest)             returns (google.protobuf.Empty);
  rpc AssignTask(AssignTaskRequest)             returns (TaskResponse);
  rpc UpdateTaskStatus(UpdateTaskStatusRequest) returns (TaskResponse);
}

message Task {
  string id, project_id, company_id, title, description;
  string status;       // todo | in_progress | done | blocked
  string priority;     // low | medium | high | critical
  string assigned_to;  // user_id, empty if unassigned
  string created_by, due_date;
  Timestamp created_at, updated_at;
}

message Project {
  string id, company_id, name, description;
  string status;       // active | completed | archived
  string start_date, end_date, created_by;
  Timestamp created_at, updated_at;
}
```

### user_service.proto

```protobuf
service UserService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest)       returns (LoginResponse);
  rpc GetUser(GetUserRequest)   returns (UserResponse);
}

message RegisterRequest {
  string email, name, password;
  string role;         // admin | manager | worker
  string company_id;   // join existing (if non-empty)
  string company_name; // create new (if company_id empty)
}

message LoginResponse {
  string access_token;  // RS256 signed JWT
  User   user;
}
```

### notification_service.proto

```protobuf
service NotificationService {
  rpc GetNotifications(GetNotificationsRequest) returns (NotificationsResponse);
  rpc GetUnreadCount(GetUnreadCountRequest)     returns (UnreadCountResponse);
  rpc MarkAsRead(MarkAsReadRequest)             returns (google.protobuf.Empty);
  rpc MarkAllAsRead(MarkAllAsReadRequest)       returns (google.protobuf.Empty);
}

message Notification {
  string id, user_id, company_id;
  string type;      // task_assigned | status_changed
  string title, message;
  bool   is_read;
  string metadata;  // JSON: {task_id, project_id, old_status, new_status, ...}
  Timestamp created_at;
}
```

---

## 7. Database Schema

Database: PostgreSQL 16. Single database `constructflow`, all services share it (with table isolation).

### Tables

#### companies
```sql
CREATE TABLE companies (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### users
```sql
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id),
    email      VARCHAR(255) NOT NULL,
    name       VARCHAR(255) NOT NULL,
    password   VARCHAR(255) NOT NULL,  -- bcrypt hash (cost=12)
    role       VARCHAR(50) NOT NULL DEFAULT 'worker',  -- admin | manager | worker
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ  -- soft delete
);

CREATE UNIQUE INDEX idx_users_email_company ON users(email, company_id);
CREATE INDEX idx_users_company ON users(company_id);
CREATE INDEX idx_users_deleted ON users(deleted_at) WHERE deleted_at IS NULL;
```

#### projects
```sql
CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id  UUID NOT NULL REFERENCES companies(id),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(50) NOT NULL DEFAULT 'active',  -- active | completed | archived
    start_date  DATE,
    end_date    DATE,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_projects_company ON projects(company_id);
CREATE INDEX idx_projects_company_status ON projects(company_id, status);
CREATE INDEX idx_projects_deleted ON projects(deleted_at) WHERE deleted_at IS NULL;
```

#### tasks
```sql
CREATE TABLE tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id),
    company_id  UUID NOT NULL,  -- denormalized for tenant-scoped query performance
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(50) NOT NULL DEFAULT 'todo',    -- todo | in_progress | done | blocked
    priority    VARCHAR(50) NOT NULL DEFAULT 'medium',  -- low | medium | high | critical
    assigned_to UUID REFERENCES users(id),
    created_by  UUID NOT NULL REFERENCES users(id),
    due_date    DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

-- Primary query: list tasks by company + project
CREATE INDEX idx_tasks_company_project ON tasks(company_id, project_id);
-- Filter by status
CREATE INDEX idx_tasks_company_status ON tasks(company_id, status);
-- Worker's task list
CREATE INDEX idx_tasks_assigned ON tasks(assigned_to) WHERE deleted_at IS NULL;
-- Overdue alerts (only active tasks)
CREATE INDEX idx_tasks_due_date ON tasks(due_date) WHERE status != 'done' AND deleted_at IS NULL;
-- Priority queue (open tasks only)
CREATE INDEX idx_tasks_priority ON tasks(company_id, priority) WHERE status = 'todo';
-- Skip soft-deleted
CREATE INDEX idx_tasks_deleted ON tasks(deleted_at) WHERE deleted_at IS NULL;
```

#### notifications
```sql
CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    company_id UUID NOT NULL,
    type       VARCHAR(50) NOT NULL,   -- task_assigned | status_changed
    title      VARCHAR(255) NOT NULL,
    message    TEXT,
    is_read    BOOLEAN NOT NULL DEFAULT FALSE,
    metadata   JSONB,   -- {task_id, project_id, old_status, new_status, assigned_by, ...}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- No updated_at: immutable once created. No deleted_at: kept for audit.
);

CREATE INDEX idx_notifications_user_unread ON notifications(user_id, is_read) WHERE is_read = FALSE;
CREATE INDEX idx_notifications_user_time ON notifications(user_id, created_at DESC);
```

#### audit_logs (partitioned)
```sql
CREATE TABLE audit_logs (
    id           UUID NOT NULL DEFAULT gen_random_uuid(),
    company_id   UUID NOT NULL,
    user_id      UUID NOT NULL,
    action       VARCHAR(100) NOT NULL,  -- task.assigned | file.uploaded | user.login
    resource     VARCHAR(50) NOT NULL,   -- task | file | user | report
    resource_id  UUID NOT NULL,
    before_state JSONB,
    after_state  JSONB,
    ip_address   VARCHAR(45),
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (occurred_at);

-- Monthly partitions
CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
-- ... (one per month)

CREATE INDEX idx_audit_company_resource ON audit_logs(company_id, resource, resource_id);
CREATE INDEX idx_audit_company_time ON audit_logs(company_id, occurred_at DESC);
CREATE INDEX idx_audit_company_user ON audit_logs(company_id, user_id);
```

#### files
```sql
CREATE TABLE files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   UUID NOT NULL,
    project_id   UUID,
    task_id      UUID,
    uploaded_by  UUID NOT NULL,
    name         VARCHAR(500) NOT NULL,
    s3_key       VARCHAR(1000) NOT NULL,
    s3_bucket    VARCHAR(100) NOT NULL,
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    mime_type    VARCHAR(100),
    storage_tier VARCHAR(20) NOT NULL DEFAULT 'standard',
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending | confirmed | deleted
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX idx_files_company ON files(company_id);
CREATE INDEX idx_files_task ON files(task_id) WHERE task_id IS NOT NULL;
CREATE INDEX idx_files_lifecycle ON files(storage_tier, created_at) WHERE deleted_at IS NULL;
```

#### report_jobs
```sql
CREATE TABLE report_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   UUID NOT NULL,
    requested_by UUID NOT NULL,
    type         VARCHAR(50) NOT NULL,
    params       JSONB,
    status       VARCHAR(20) NOT NULL DEFAULT 'queued',  -- queued | processing | done | failed
    s3_key       VARCHAR(1000),
    download_url VARCHAR(2000),
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_report_jobs_company ON report_jobs(company_id, created_at DESC);
```

### Index Design Rationale

| Index | Why |
|-------|-----|
| `idx_tasks_company_project` | Composite — primary query pattern: list tasks in a project for a company |
| `idx_tasks_due_date` (partial) | Only active tasks — skip ~80% completed rows, used by scheduler deadline check |
| `idx_tasks_priority` (partial) | Only `todo` tasks — backlog priority sorting |
| `idx_tasks_assigned` (partial) | Skip deleted — "my tasks" query for workers |
| `idx_notifications_user_unread` (partial) | Only unread — most frequent notification query |
| `audit_logs` (partitioned) | Monthly partition pruning — queries with date range only scan relevant partitions |

---

## 8. Authentication & Authorization

### JWT RS256 (Asymmetric)

```
┌──────────────┐                    ┌──────────────┐
│ user-service │                    │  gw-gateway  │
│              │                    │              │
│ Private Key ─┼── sign JWT ──────▶│ Public Key   │
│ (RS256)      │   {user_id,       │ (verify only)│
│              │    company_id,    │              │
│ bcrypt       │    role}          │ Casbin RBAC  │
│ cost=12      │                    │ Rate Limit   │
└──────────────┘                    └──────────────┘
```

- **Private key** only exists in user-service → gateway compromise cannot forge tokens
- **Public key** in gateway is only used for verification
- This is why RS256 was chosen over HS256 (shared secret)

### Auth Middleware (gateway)

```go
func authHandler(pubKey *rsa.PublicKey) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Extract "Bearer <token>" from Authorization header
        // 2. jwt.ParseWithClaims — verify RSA signature + expiry
        // 3. Check signing method is RSA (reject HS256, none, etc.)
        // 4. Extract claims: user_id, company_id, role
        // 5. Validate: user_id + company_id not empty
        // 6. Inject into Gin context for downstream use
    }
}
```

### RBAC (Casbin)

**Model** (`casbin/model.conf`):
```
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && (r.dom == p.dom || p.dom == "*") && keyMatch2(r.obj, p.obj) && r.act == p.act
```

**Policy** (`casbin/policy.csv`):
```csv
# Admin: full access
p, admin, *, /projects,       read
p, admin, *, /projects,       write
p, admin, *, /tasks,          read
p, admin, *, /tasks,          write
p, admin, *, /tasks/assign,   write
p, admin, *, /tasks/status,   write
p, admin, *, /notifications,  read
p, admin, *, /notifications,  write

# Manager: projects+tasks CRUD, assign tasks, notifications read
p, manager, *, /projects,      read
p, manager, *, /projects,      write
p, manager, *, /tasks,         read
p, manager, *, /tasks,         write
p, manager, *, /tasks/assign,  write
p, manager, *, /tasks/status,  write
p, manager, *, /notifications, read

# Worker: read projects, read tasks, update status, read notifications
p, worker, *, /projects,       read
p, worker, *, /tasks,          read
p, worker, *, /tasks/status,   write
p, worker, *, /notifications,  read
```

**How it works**: `enforcer.Enforce(role, companyID, resource, action)` — the `*` in domain means the policy applies to ALL companies. The matcher includes `(r.dom == p.dom || p.dom == "*")` to support wildcard domains.

### Rate Limiting

```go
func RateLimitMiddleware(rdb *redis.Client, requestsPerMinute int) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        key := fmt.Sprintf("ratelimit:%s", ip)
        count, err := rdb.Incr(ctx, key).Result()
        if err != nil { c.Next(); return }  // fail open
        if count == 1 { rdb.Expire(ctx, key, time.Minute) }
        if int(count) > requestsPerMinute {
            c.AbortWithStatusJSON(429, ...)
            return
        }
        c.Next()
    }
}
```

Algorithm: Fixed Window Counter per client IP. Configurable via `RATE_LIMIT_RPM` env var (default 60).

---

## 9. Task State Machine

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

**Transition map** (hardcoded in use-case layer):
```go
var validTransitions = map[string][]string{
    "todo":        {"in_progress"},
    "in_progress": {"done", "blocked"},
    "blocked":     {"in_progress"},
    "done":        {"in_progress"},  // manager/admin only
}
```

**Role-based restriction**: `done → in_progress` requires `role == "manager" || role == "admin"`. Workers cannot reopen completed tasks.

**Invalid transitions** (return 400):
- `todo → done` (must go through in_progress)
- `todo → blocked` (must start work first)
- `done → todo` (no direct reverse)
- `blocked → done` (must resume first)

---

## 10. Event-Driven Architecture (RabbitMQ)

### Topology

```
task-service ──publish──▶ constructflow.events (direct exchange, durable)
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

**Publisher implementation** (task-service):
```go
type envelope struct {
    EventID   string      `json:"event_id"`    // uuid.New().String()
    EventType string      `json:"event_type"`
    Timestamp time.Time   `json:"timestamp"`   // time.Now().UTC()
    Payload   interface{} `json:"payload"`
}
// Published to exchange "constructflow.events" with routing key = eventType
// DeliveryMode: Persistent (survives broker restart)
// ContentType: application/json
```

### Reliability Patterns

| Pattern | Implementation |
|---------|---------------|
| **Idempotency** | `SET NX event:<event_id>` in Redis (24h TTL) before processing. Duplicate → skip + ACK |
| **Dead letter queue** | 3 retries with exponential backoff (1s → 2s → 4s) → `<queue>.dlq` |
| **Non-fatal publish** | Task DB write succeeds even if RabbitMQ publish fails (eventual consistency) |
| **Persistent delivery** | `DeliveryMode: Persistent` — messages survive broker restart |
| **Durable exchange/queues** | Exchange and queues survive broker restart |

---

## 11. Key Design Patterns

### Distributed Lock (Task Assignment)

```go
// Prevent concurrent double-assignment
locked, err := uc.lockClient.SetNX(ctx, "lock:task:assign:"+req.TaskID, "1", 5*time.Second)
if !locked {
    return nil, common.ErrTaskLocked  // → gRPC Aborted → HTTP 409
}
defer uc.lockClient.Del(ctx, "lock:task:assign:"+req.TaskID)
```

- Key: `lock:task:assign:<taskID>`
- TTL: 5 seconds (auto-release if process crashes)
- Implementation: Redis `SET NX` (atomic check-and-set)

### Full Assign Task Flow

```
Client POST /tasks/:id/assign
  │
  ├── gw-gateway: Rate Limit → JWT Auth → RBAC (manager/admin)
  │
  ├── task-service:
  │   ├── Redis SET NX lock:task:assign:<id> TTL=5s
  │   │   └── Locked? → 409 Conflict
  │   ├── PostgreSQL SELECT task WHERE id=? AND company_id=?
  │   │   └── Not found? → 404
  │   ├── PostgreSQL SELECT user WHERE id=? AND company_id=?
  │   │   └── Not in same company? → 400 (cross-tenant protection)
  │   ├── PostgreSQL UPDATE task SET assigned_to=?
  │   ├── RabbitMQ publish "task.assigned" (non-fatal)
  │   └── defer Redis DEL lock
  │
  └── RabbitMQ fan-out:
      ├── notification-service → SET NX event:<id> → INSERT notification
      ├── audit-service → INSERT audit_log
      ├── search-service → Update Elasticsearch index
      └── scheduler-service → Track deadline
```

### Error Mapping Chain

```
Domain Error                → gRPC Code           → HTTP Status
────────────────────────────────────────────────────────────────
ErrNotFound                 → codes.NotFound       → 404
ErrUserNotFound             → codes.NotFound       → 404
ErrInvalidInput             → codes.InvalidArgument → 400
ErrInvalidStatusTransition  → codes.InvalidArgument → 400
ErrForbidden                → codes.PermissionDenied → 403
ErrTaskLocked               → codes.Aborted         → 409
ErrAlreadyExists            → codes.AlreadyExists    → 409
ErrInvalidCredentials       → codes.Unauthenticated  → 401
unknown                     → codes.Internal         → 500
```

---

## 12. Clean Architecture

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
│                          Service ─────-┘            │
│                          (service/)                 │
└─────────────────────────────────────────────────────┘
```

**Inner layer** (domain/ + use-case/): ZERO external imports. Pure business logic.
**Outer layer**: gRPC handlers, GORM repositories, Redis/RabbitMQ service implementations.

### Directory Layout (task-service example)

```
apps/task-service/
├── main.go                          # Dependency injection (wire everything)
├── bootstrap/
│   ├── config.go                    # Viper env parsing
│   ├── database.go                  # GORM PostgreSQL + AutoMigrate
│   ├── redis.go                     # Redis client
│   ├── rabbitmq.go                  # Exchange + queue declaration
│   └── grpc_server.go              # gRPC server + APM interceptor
├── api/grpc/controller/
│   └── task_controller.go          # gRPC handlers + proto↔domain mappers
├── domain/                          # Interfaces ONLY (zero deps)
│   ├── task_repository.go
│   ├── project_repository.go
│   ├── user_repository.go
│   ├── event_publisher.go
│   └── lock_client.go
├── entity/
│   ├── model/                       # GORM structs (Task, Project, User, Company)
│   └── dto/                         # Request/Response structs + mappers
├── use-case/
│   ├── create_task/                 # create_task.go + create_task_test.go
│   ├── assign_task/                 # assign_task.go + assign_task_test.go
│   ├── update_task_status/          # update_task_status.go + update_task_status_test.go
│   └── create_project/             # create_project.go
├── repository/sql/                  # GORM implementations
│   ├── task_repository.go
│   ├── project_repository.go
│   └── user_repository.go
├── service/                         # Infrastructure implementations
│   ├── redis_lock.go               # domain.LockClient → Redis SET NX
│   └── rabbitmq_publisher.go       # domain.EventPublisher → RabbitMQ
└── common/
    ├── errors.go                    # Domain error types
    └── pagination.go               # Page, PageSize, Offset helpers
```

**Swap-ability**: Because use cases depend only on interfaces, you can swap Redis lock → DynamoDB lock, RabbitMQ → Kafka — without changing a single line of business logic.

---

## 13. Multi-Tenancy

```
JWT Claims { user_id, company_id, role }
       │
       ▼
Gateway extracts company_id from JWT
       │
       ▼
gRPC Metadata carries company_id to backend services
       │
       ▼
Every Repository query: WHERE company_id = ?
```

- Every table has `company_id` column
- Every query scopes to `WHERE company_id = ?`
- `company_id` is denormalized on `tasks` table (also on `projects`) to avoid JOINs on hot queries
- No code path exists to query data without tenant scope
- Cross-tenant access is **structurally impossible**
- Assign task validates assignee belongs to same company

---

## 14. Infrastructure & Config

### Docker Compose Containers (16 total)

**Infrastructure (7)**:
| Container | Image | Ports | Purpose |
|-----------|-------|-------|---------|
| postgres | postgres:16-alpine | 5432 | Primary database |
| redis | redis:7-alpine | 6379 | Rate limit, distributed lock, idempotency |
| rabbitmq | rabbitmq:3-management-alpine | 5672, 15672 | Message broker (management UI: guest/guest) |
| minio | minio/minio:latest | 9000, 9001 | S3-compatible storage (console: minioadmin/minioadmin) |
| elasticsearch | elasticsearch:8.13.0 | 9200 | Full-text search, APM data (elastic/changeme) |
| kibana | kibana:8.13.0 | 5601 | Observability UI (elastic/changeme) |
| apm-server | apm-server:8.13.0 | 8200 | APM trace collection |

**Application (9)**:
| Container | Port | Depends On |
|-----------|------|-----------|
| user-service | 50053 | postgres |
| task-service | 50051 | postgres, redis, rabbitmq |
| notification-service | 50052 | postgres, redis, rabbitmq |
| gateway | 8080 | redis, user-service, task-service, notification-service |
| file-service | 50054 | postgres, minio |
| report-service | 50055 | postgres, rabbitmq, minio |
| audit-service | 50056 | postgres, rabbitmq |
| search-service | 50057 | rabbitmq, elasticsearch |
| scheduler-service | 50058 | redis, rabbitmq |

### Credentials

| Service | Username | Password |
|---------|----------|----------|
| PostgreSQL | admin | secret |
| RabbitMQ | guest | guest |
| MinIO | minioadmin | minioadmin |
| Elasticsearch/Kibana | elastic | changeme |

### Volumes

```yaml
volumes:
  postgres_data:    # PostgreSQL data
  redis_data:       # Redis AOF persistence
  rabbitmq_data:    # RabbitMQ messages
  es_data:          # Elasticsearch indices
  minio_data:       # S3 object storage
```

### Dockerfiles

All services use the same multi-stage pattern:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY gen/go/ gen/go/      # shared proto stubs
COPY apps/ apps/          # all service source
WORKDIR /app/apps/<service-name>
RUN go build -o /<binary-name> .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /<binary-name> /<binary-name>
EXPOSE <port>
ENTRYPOINT ["/<binary-name>"]
```

### Go Workspace

```
go.work:
  go 1.25.0
  use (
    ./apps/gw-gateway
    ./apps/notification-service
    ./apps/task-service
    ./apps/user-service
    ./apps/file-service
    ./apps/report-service
    ./apps/audit-service
    ./apps/search-service
    ./apps/scheduler-service
    ./gen/go
  )
```

10 modules in a single workspace — shared proto stubs via `gen/go/`.

---

## 15. Docker Compose

Full docker-compose.yml includes:
- Health checks on all infrastructure containers
- `depends_on` with `condition: service_healthy` for correct startup order
- Volume mounts: `./keys:/keys:ro` (JWT keys), `./casbin:/casbin:ro` (RBAC policy)
- Elasticsearch security: `ELASTIC_PASSWORD=changeme`, `xpack.security.http.ssl.enabled=false`
- Kibana: `ELASTICSEARCH_USERNAME=kibana_system`
- APM Server: configured with ES + Kibana credentials
- All services get `ELASTIC_APM_SERVER_URL` + `ELASTIC_APM_SERVICE_NAME` for tracing

---

## 16. Kubernetes

Manifests in `k8s/` directory:

```
k8s/
├── namespace.yaml                    # constructflow namespace
├── gateway/
│   ├── deployment.yaml              # 2 replicas, readiness+liveness probes, resource limits
│   ├── service.yaml                 # ClusterIP :8080
│   └── hpa.yaml                     # HPA: 2-10 replicas, CPU target 70%
├── task-service/
│   ├── deployment.yaml              # 2 replicas, secrets for DB+RabbitMQ creds
│   └── service.yaml                 # ClusterIP :50051
├── user-service/
│   ├── deployment.yaml              # 2 replicas, JWT key volume mount
│   └── service.yaml                 # ClusterIP :50053
└── notification-service/
    ├── deployment.yaml              # 2 replicas
    └── service.yaml                 # ClusterIP :50052
```

**Key K8s features**:
- Readiness/liveness probes on gateway (`/health`)
- Resource requests/limits (100m/128Mi → 500m/256Mi)
- Secrets: `db-credentials`, `rabbitmq-credentials`, `jwt-keys`
- HPA for gateway: auto-scale 2→10 pods at 70% CPU

---

## 17. CI/CD

`.github/workflows/ci.yml`:

```yaml
name: CI
on:
  push: { branches: ["main"] }
  pull_request: { branches: ["main"] }

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }

      # Lint all 4 core services
      - golangci-lint (task-service)
      - golangci-lint (user-service)
      - golangci-lint (notification-service)
      - golangci-lint (gw-gateway)

      # Test all 4 core services with race detector
      - go test ./... -race -timeout 60s (each service)
```

---

## 18. Testing

### Test Suites

| Suite | Service | File | Key Test Cases |
|-------|---------|------|---------------|
| assign_task | task-service | `use-case/assign_task/assign_task_test.go` | Lock acquired success, lock contention (409), task not found, cross-tenant assignee rejection, publish failure (non-fatal), invalid input |
| update_task_status | task-service | `use-case/update_task_status/update_task_status_test.go` | All valid transitions, worker denied reopen (done→in_progress), invalid transitions (todo→done, done→todo, todo→blocked) |
| create_task | task-service | `use-case/create_task/create_task_test.go` | Success with priority+due_date, priority defaults to medium, missing title, cross-tenant project rejection |
| login | user-service | `use-case/login/login_test.go` | Successful login, email not found (masked error), wrong password (same masked error), JWT generation |
| register | user-service | `use-case/register/register_test.go` | Email uniqueness per company, company create/join flow, role default to worker |

### Testing Approach

- **gomock**: All domain interfaces mocked — tests never touch real DB/Redis/RabbitMQ
- **Table-driven**: Each scenario is a row in a test table — easy to add edge cases
- **`-race` flag**: Go race detector runs on all tests — catches concurrent data races
- **Fast**: All tests complete in < 1s (pure in-memory, no I/O)

### Running Tests

```bash
# All tests for a service
cd apps/task-service && go test ./... -race

# Single test
cd apps/task-service && go test ./use-case/assign_task/ -run TestAssignTask_Success

# All services
for svc in task-service user-service notification-service gw-gateway; do
  cd apps/$svc && go test ./... -race && cd ../..
done
```

---

## 19. Observability (Elastic APM)

```
All 9 services ──── traces ────▶ APM Server :8200 ──▶ Elasticsearch ──▶ Kibana :5601

Instrumentation layers:
  ├── Gateway:     apmgin middleware (every HTTP request automatically traced)
  ├── gRPC:        apmgrpc interceptors (every RPC call on all servers)
  ├── RabbitMQ:    custom spans (message publish + consume)
  └── PostgreSQL:  GORM plugin (every DB query)
```

**What you can see in Kibana**:
- **Service map**: Visual dependency graph auto-generated from trace data
- **Distributed tracing**: Single request traced end-to-end (client → gateway → task-service → RabbitMQ → notification-service)
- **Latency breakdown**: Time spent in each service, DB query, Redis call
- **Error tracking**: Failed requests, panics, slow queries captured automatically
- **Transaction details**: Each HTTP/gRPC call with full metadata

**Access**: Kibana at `http://localhost:5601` → login `elastic/changeme` → Observability → APM → Services

**Setup notes**: Elasticsearch 8.x requires security enabled. Fleet + APM integration must be installed in Kibana for APM Server index templates.

---

## 20. Known Limitations — Demo vs Production

### Infrastructure

| Area | Current (Demo) | Production |
|------|---------------|------------|
| Object storage | MinIO (local container) | AWS S3 (endpoint swap via env var) |
| Database | Single PostgreSQL instance | Aurora with read replicas |
| Cache | Single Redis instance | ElastiCache cluster (Sentinel/Cluster) |
| Message broker | Single RabbitMQ instance | Amazon MQ or RabbitMQ cluster (quorum queues) |
| Search | Single Elasticsearch node | Amazon OpenSearch (multi-node) |
| Deployment | Docker Compose | EKS cluster with HPA, PDB, resource limits |
| Secrets | Hardcoded in docker-compose env | AWS Secrets Manager |
| TLS | None (plain HTTP/gRPC) | TLS at ALB/Ingress, mTLS between services |

### Application

| Area | Current (Demo) | Production Fix |
|------|---------------|---------------|
| Casbin RBAC policy | CSV file, loaded at startup | `gorm-adapter` — store in PostgreSQL, management API for runtime CRUD |
| JWT refresh token | No refresh token — access token only | Add `POST /auth/refresh` with rotating refresh tokens, short-lived access tokens (15min) |
| Dual-write risk | DB write + RabbitMQ publish in same handler | Transactional outbox pattern: write `outbox` table in same DB tx, background worker publishes |
| Redis lock TTL race | `defer Del(lockKey)` may delete another goroutine's lock | Lua script atomic check-and-delete: only delete if value matches unique lock token |
| Rate limiting | Fixed window counter | Token bucket via Lua script — atomic, allows controlled burst |
| Password validation | Min 6 chars only | Complexity rules or zxcvbn |
| Email verification | None | Verification email with token |
| Graceful shutdown | Only notification-service has it | All services: `signal.NotifyContext` + gRPC graceful stop + drain consumers |
| Circuit breaker | None | `sony/gobreaker` between gateway and downstream services |
| Pagination | Offset-based (LIMIT/OFFSET) | Cursor-based for large datasets |
| DB migrations | GORM AutoMigrate at startup | `golang-migrate` or `atlas`, run separately |

### Security

| Area | Current (Demo) | Production Fix |
|------|---------------|---------------|
| CORS | Not configured | Whitelist allowed origins |
| Request size limit | No limit | Body size middleware |
| Input sanitization | None | Sanitize HTML/script in user strings |
| API versioning | `/api/v1` prefix only | Version negotiation strategy |

---

## 21. How to Run

### Prerequisites
- Docker + Docker Compose
- openssl (one-time key generation)

### Steps

```bash
# 1. Clone
git clone https://github.com/hodynguyen/construct-flow.git
cd construct-flow

# 2. Generate RSA keys (one-time)
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem

# 3. Start everything
docker-compose up --build

# 4. Wait for all containers to be healthy (~60s)
docker-compose ps

# 5. Access
#    REST API:          http://localhost:8080
#    Swagger UI:        http://localhost:8080/swagger/index.html
#    RabbitMQ UI:       http://localhost:15672 (guest/guest)
#    MinIO Console:     http://localhost:9001 (minioadmin/minioadmin)
#    Kibana:            http://localhost:5601 (elastic/changeme)

# 6. First-time Kibana APM setup
#    POST http://localhost:5601/api/fleet/setup (elastic/changeme)
#    POST http://localhost:5601/api/fleet/epm/packages/apm
```

### Quick Smoke Test

```bash
BASE=http://localhost:8080/api/v1

# Register
curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@acme.com","name":"Admin","password":"pass123","role":"admin","company_name":"Acme Corp"}'

# Login
TOKEN=$(curl -s -X POST $BASE/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@acme.com","password":"pass123"}' | jq -r '.access_token')

# Create Project
PROJECT_ID=$(curl -s -X POST $BASE/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Site Alpha"}' | jq -r '.project.id')

# Create Task
TASK_ID=$(curl -s -X POST $BASE/projects/$PROJECT_ID/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Install plumbing","priority":"high"}' | jq -r '.task.id')

# Check notifications
curl -s $BASE/notifications -H "Authorization: Bearer $TOKEN"
```

---

## 22. E2E Test Flow

See `docs/e2e-test-guide.md` for step-by-step Swagger UI testing with 5 scenarios:

1. **Happy Path**: Register → Login → Create Project → Create Task → Assign → Notification → Status Updates → Mark Read
2. **State Machine**: Invalid transitions (todo→done, done→todo), valid transitions (in_progress↔blocked)
3. **RBAC**: Worker cannot assign/create, worker can read
4. **Auth**: Missing token → 401
5. **Rate Limiting**: >60 requests/min → 429

---

## Project Structure (full tree)

```
construct-flow/
├── apps/
│   ├── gw-gateway/                # HTTP :8080
│   ├── task-service/              # gRPC :50051
│   ├── user-service/              # gRPC :50053
│   ├── notification-service/      # gRPC :50052
│   ├── file-service/              # gRPC :50054
│   ├── report-service/            # gRPC :50055
│   ├── audit-service/             # gRPC :50056
│   ├── search-service/            # gRPC :50057
│   └── scheduler-service/         # gRPC :50058
├── gen/go/proto/                  # Generated Go stubs (buf generate)
├── proto/
│   ├── task_service/v1/
│   ├── user_service/v1/
│   └── notification_service/v1/
├── k8s/                           # Kubernetes manifests
├── keys/                          # RSA key pair (gitignored)
├── casbin/
│   ├── model.conf                 # RBAC model
│   └── policy.csv                 # RBAC policy
├── scripts/
│   └── init.sql                   # Full database schema
├── docs/
│   ├── architecture.md
│   ├── e2e-test-guide.md
│   └── PROJECT_DEEP_DIVE.md       # This file
├── .github/workflows/ci.yml
├── docker-compose.yml
├── go.work
└── README.md
```

---

## Infrastructure Mapping (Local → AWS)

| Local | AWS Equivalent |
|-------|---------------|
| PostgreSQL 16 (Docker) | Amazon Aurora |
| Redis 7 (Docker) | ElastiCache |
| RabbitMQ 3 (Docker) | Amazon MQ |
| MinIO (Docker) | S3 + Lifecycle Policies |
| Elasticsearch 8 (Docker) | Amazon OpenSearch |
| Elastic APM (Docker) | AWS X-Ray |
| Docker Compose | EKS (Kubernetes) |
| keys/ directory | AWS Secrets Manager |
| GitHub Actions | CodePipeline + CodeBuild |

All infrastructure addresses come from environment variables — zero cloud-provider lock-in.
