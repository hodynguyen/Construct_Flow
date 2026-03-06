# ConstructFlow — Implementation Plan

> 8-Day Build Plan | ANDPAD Round 2 Interview
> Generated: 2026-03-05 | Extended: 2026-03-06 (5 new services)

---

## Quick Reference

```
MUST SHIP: distributed lock + state machine + unit tests + docker-compose up + event flow
CUT FIRST: Elastic APM → then K8s → then list/filter endpoints
NEVER CUT: AssignTask, UpdateStatus, JWT auth, RabbitMQ consumer
```

---

## Technical Setup

### Prerequisites (install before Day 1)

```bash
# Go 1.23+
brew install go

# buf (proto toolchain)
brew install bufbuild/buf/buf

# golangci-lint
brew install golangci-lint

# mockgen
go install github.com/golang/mock/mockgen@latest

# swag (Swagger)
go install github.com/swaggo/swag/cmd/swag@latest

# Verify
go version   # 1.23+
buf --version
golangci-lint --version
mockgen --version
```

### Project Init Commands

```bash
# Create directory structure
mkdir -p construct-flow && cd construct-flow
mkdir -p apps/{gw-gateway,task-service,user-service,notification-service}
mkdir -p proto/{task_service,user_service,notification_service}/v1
mkdir -p scripts k8s/{gateway,task-service,user-service,notification-service,infrastructure}

# Initialize Go workspace
go work init

# Init each service module
cd apps/task-service && go mod init github.com/hody/construct-flow/apps/task-service && cd ../..
cd apps/user-service && go mod init github.com/hody/construct-flow/apps/user-service && cd ../..
cd apps/notification-service && go mod init github.com/hody/construct-flow/apps/notification-service && cd ../..
cd apps/gw-gateway && go mod init github.com/hody/construct-flow/apps/gw-gateway && cd ../..

# Register in workspace
go work use ./apps/task-service ./apps/user-service ./apps/notification-service ./apps/gw-gateway

# buf workspace
cat > buf.yaml << 'EOF'
version: v1
modules:
  - directory: proto/task_service/v1
  - directory: proto/user_service/v1
  - directory: proto/notification_service/v1
EOF

cat > buf.gen.yaml << 'EOF'
version: v1
plugins:
  - name: go
    out: gen/go
    opt: paths=source_relative
  - name: go-grpc
    out: gen/go
    opt: paths=source_relative,require_unimplemented_servers=false
EOF
```

### Key Dependencies per Service

```bash
# task-service
cd apps/task-service
go get \
  google.golang.org/grpc \
  google.golang.org/protobuf \
  gorm.io/gorm \
  gorm.io/driver/postgres \
  github.com/redis/go-redis/v9 \
  github.com/rabbitmq/amqp091-go \
  github.com/spf13/viper \
  github.com/google/uuid \
  github.com/golang/mock/gomock \
  github.com/stretchr/testify \
  go.elastic.co/apm/module/apmgrpc/v2

# user-service
cd apps/user-service
go get \
  google.golang.org/grpc \
  gorm.io/gorm gorm.io/driver/postgres \
  github.com/golang-jwt/jwt/v5 \
  golang.org/x/crypto \
  github.com/spf13/viper

# gw-gateway
cd apps/gw-gateway
go get \
  github.com/gin-gonic/gin \
  github.com/casbin/casbin/v2 \
  github.com/casbin/gorm-adapter/v3 \
  github.com/redis/go-redis/v9 \
  github.com/golang-jwt/jwt/v5 \
  github.com/swaggo/swag \
  github.com/swaggo/gin-swagger \
  go.elastic.co/apm/module/apmgin/v2 \
  google.golang.org/grpc

# notification-service
cd apps/notification-service
go get \
  google.golang.org/grpc \
  gorm.io/gorm gorm.io/driver/postgres \
  github.com/redis/go-redis/v9 \
  github.com/rabbitmq/amqp091-go \
  github.com/spf13/viper
```

---

## Day 1 — Foundation

**Goal:** Everything compiles, docker-compose up runs all infra, proto stubs generated, DB schema ready.

### Morning (4h) — Project scaffold + Docker + Proto

```
[x] Create directory structure (commands above)
[x] Write docker-compose.yml (copy from proposal section 11, verify)
[x] Write scripts/init.sql (create tables + indexes from proposal section 5)
[x] Write proto/task_service/v1/task.proto
[x] Write proto/user_service/v1/user.proto
[x] Write proto/notification_service/v1/notification.proto
[x] buf generate → verify stubs generated in gen/go/
[ ] docker-compose up -d postgres redis rabbitmq
[ ] Verify: docker-compose exec postgres psql -U admin -d constructflow
```

**Expected output:** Infrastructure running, proto stubs compiled.

### Afternoon (4h) — Service skeleton + config

```
[x] task-service: bootstrap/config.go (Viper env loading)
[x] task-service: bootstrap/database.go (GORM + AutoMigrate)
[x] task-service: bootstrap/redis.go
[x] task-service: bootstrap/rabbitmq.go (connection + channel setup)
[x] task-service: entity/model/task.go + project.go (GORM models)
[x] task-service: domain/task_repository.go (interface)
[x] task-service: domain/project_repository.go (interface)
[x] task-service: domain/event_publisher.go (interface)
[x] task-service: common/errors.go
[x] task-service: main.go (stub — compiles but does nothing)
[x] Repeat skeleton for user-service, notification-service, gw-gateway
[x] Verify: go build ./... for all services
```

**Expected output:** All 4 services compile. Infra running.

---

## Day 2 — Task Service (Core)

**Goal:** AssignTask, UpdateTaskStatus, CreateTask working end-to-end with unit tests.

### Morning (4h) — Repository + Use Cases

```
[x] task-service/repository/sql/task_repository.go (GORM impl)
    [x] FindByID(ctx, companyID, taskID) — with company_id scope
    [x] Create(ctx, task)
    [x] Update(ctx, task)
    [x] ListByProject(ctx, companyID, projectID, filter, pagination)
[x] task-service/repository/sql/project_repository.go
    [x] Create, FindByID, ListByCompany
[x] task-service/use-case/create_task/create_task.go
    [x] Validate input, persist, return DTO
[x] task-service/use-case/create_project/create_project.go
[x] task-service/use-case/assign_task/assign_task.go  ← PRIORITY
    [x] SetNX distributed lock
    [x] FindByID task (with companyID)
    [x] Validate assignee in same company
    [x] Update DB
    [x] Publish task.assigned event
    [x] Release lock (defer Del)
[x] task-service/service/rabbitmq_publisher.go (implements EventPublisher)
    [x] Publish(ctx, eventType, payload) → amqp091 publish
```

### Afternoon (4h) — State Machine + Unit Tests

```
[x] task-service/use-case/update_task_status/update_task_status.go  ← PRIORITY
    [x] Load allowed transitions map
    [x] Validate current status → new status
    [x] Validate role (done→in_progress requires manager)
    [x] Update DB
    [x] Publish task.status_changed event

[x] go generate ./... (generate mocks for domain interfaces)

UNIT TESTS (all in same afternoon):
[x] use-case/assign_task/assign_task_test.go
    [x] TestAssignTask_Success
    [x] TestAssignTask_LockFailed (concurrent protection)
    [x] TestAssignTask_TaskNotFound
    [x] TestAssignTask_CrossTenantBlocked (assignee from different company)
    [x] TestAssignTask_PublishFails (publish error, task still assigned)

[x] use-case/update_task_status/update_task_status_test.go
    [x] TestUpdateStatus_TodoToInProgress_Allowed
    [x] TestUpdateStatus_InProgressToDone_Allowed
    [x] TestUpdateStatus_InProgressToBlocked_Allowed
    [x] TestUpdateStatus_BlockedToInProgress_Allowed
    [x] TestUpdateStatus_DoneToTodo_Forbidden
    [x] TestUpdateStatus_DoneToInProgress_ManagerAllowed
    [x] TestUpdateStatus_DoneToInProgress_WorkerForbidden

[x] use-case/create_task/create_task_test.go
    [x] TestCreateTask_Success
    [x] TestCreateTask_InvalidInput

[x] Verify: go test ./... (all tests pass)
```

**Expected output:** Core business logic working with full test coverage. `go test ./...` passes.

---

## Day 3 — User Service + Gateway (Auth Layer)

**Goal:** Register/login working, JWT issued, gateway validates JWT.

### Morning (3.5h) — User Service

```
[x] user-service/entity/model/user.go
[x] user-service/domain/user_repository.go (interface)
[x] user-service/domain/token_service.go (interface — GenerateToken, ValidateToken)
[x] user-service/repository/sql/user_repository.go
    [x] Create(ctx, user)
    [x] FindByEmail(ctx, email)
    [x] FindByID(ctx, companyID, id)
[x] user-service/service/jwt_service.go
    [x] GenerateToken(userID, companyID, role) → RS256 signed JWT
    [x] ValidateToken(token) → claims
    [x] Load private/public key from file (RSA key pair)
[x] user-service/use-case/register/register.go
    [x] Validate email uniqueness
    [x] Hash password (bcrypt, cost 12)
    [x] Create user record
[x] user-service/use-case/login/login.go
    [x] Find user by email
    [x] Compare bcrypt hash
    [x] Generate JWT token
[x] user-service/use-case/register/register_test.go
    [x] TestRegister_Success
    [x] TestRegister_EmailAlreadyExists
[x] user-service/use-case/login/login_test.go
    [x] TestLogin_Success
    [x] TestLogin_WrongPassword
    [x] TestLogin_UserNotFound
[x] user-service/api/grpc/controller/user_controller.go
    [x] Register RPC handler
    [x] Login RPC handler
[x] user-service/main.go (wire everything, start gRPC server :50053)
```

### Afternoon (3.5h) — Gateway Core

```
[x] gw-gateway/bootstrap/config.go
[x] gw-gateway/bootstrap/redis.go (rate limiting)
[x] gw-gateway/bootstrap/grpc_clients.go (dial task-service, user-service, notification-service)
[x] gw-gateway/api/http/middleware/auth.go
    [x] Parse Bearer token from Authorization header
    [x] Validate JWT signature (RS256 public key)
    [x] Inject company_id, user_id, role into gin.Context
[x] gw-gateway/api/http/middleware/rate_limit.go
    [x] Redis sliding window counter per IP
    [x] Return 429 if exceeded
[x] gw-gateway/api/http/handler/auth_handler.go
    [x] POST /api/v1/auth/register → call user-service gRPC
    [x] POST /api/v1/auth/login → call user-service gRPC, return JWT
[x] gw-gateway/api/http/router.go
    [x] Register auth routes (public)
    [x] Register project/task/notification routes (JWT required) — placeholder, wired Day 4
[x] gw-gateway/main.go (start Gin server :8080)
[ ] Manual test: register + login via curl, receive JWT
```

**Expected output:** Can register user, login, receive JWT. Gateway validates JWT and rejects invalid tokens.

---

## Day 4 — Gateway (RBAC + Full Endpoints) + Notification Service

**Goal:** All REST endpoints working. Events flow from task-service → RabbitMQ → notification-service.

### Morning (4h) — Gateway Casbin + All Handlers

```
[x] gw-gateway/api/http/middleware/rbac.go
    [x] Initialize Casbin enforcer with domain model
    [x] Write casbin policy (from proposal section 9.3)
    [x] Enforce(role, companyID, resource, action) before each handler
[x] gw-gateway/api/http/handler/project_handler.go
    [x] GET /projects — list projects (forward to task-service gRPC)
    [x] POST /projects — create project
    [x] GET /projects/:id — get project
    [x] PUT /projects/:id — update project
    [x] DELETE /projects/:id — soft delete
[x] gw-gateway/api/http/handler/task_handler.go
    [x] GET /projects/:id/tasks
    [x] POST /projects/:id/tasks
    [x] GET /tasks/:id
    [x] PATCH /tasks/:id/status
    [x] POST /tasks/:id/assign
[x] gw-gateway/api/http/handler/notification_handler.go
    [x] GET /notifications
    [x] GET /notifications/unread/count
    [x] PATCH /notifications/:id/read
[x] task-service: wire gRPC controller (TaskController, ProjectController) into server
[ ] Manual test: create project → create task → assign task → check 403 for worker
```

### Afternoon (4h) — Notification Service

```
[x] notification-service/entity/model/notification.go
[x] notification-service/domain/notification_repository.go
[x] notification-service/repository/sql/notification_repository.go
    [x] Create(ctx, notification)
    [x] ListByUser(ctx, userID, companyID, pagination)
    [x] MarkRead(ctx, notificationID, userID)
    [x] GetUnreadCount(ctx, userID)
[x] notification-service/use-case/create_notification/create_notification.go
    [x] Check Redis idempotency key (SET NX event:<event_id>)
    [x] Skip if already processed
    [x] Create notification record
[x] notification-service/consumer/task_assigned_consumer.go
    [x] Consume from task_assigned_queue
    [x] Unmarshal event payload
    [x] Call create_notification use case
    [x] ACK on success, NACK on failure (with retry counter)
    [x] Route to DLQ after 3 failures
[x] notification-service/consumer/task_status_changed_consumer.go
    [x] Same pattern as above
[x] notification-service/api/grpc/controller/notification_controller.go
    [x] GetNotifications RPC
    [x] GetUnreadCount RPC
    [x] MarkAsRead RPC
[x] notification-service/main.go (start consumer goroutines + gRPC server)
[ ] End-to-end test:
    [ ] Assign task via REST API
    [ ] Verify notification appears in GET /notifications
```

**Expected output:** Full event flow working. Assign task → notification created → visible via API.

---

## Day 5 — Observability + Kubernetes + Polish

**Goal:** APM traces visible (or cleanly skipped), K8s manifests ready, docker-compose proven.

### Morning (3.5h) — Elastic APM (or skip if behind)

> **Skip trigger:** If Day 3 or Day 4 overran by >2h, skip APM entirely. Note it as future work.

```
[x] Add apm-server, elasticsearch, kibana to docker-compose.yml (already in proposal)
[x] task-service: add apmgrpc interceptor to gRPC server
[x] gw-gateway: add apmgin middleware to Gin router
[x] notification-service: add custom span for RabbitMQ message processing
[ ] docker-compose up (full stack)
[ ] Verify: Kibana at :5601 shows APM service map
[ ] Verify: Assign task → trace visible across gateway → task-service
```

**If skipping APM:**
```
[x] Add GitHub Actions CI pipeline instead (lint + test)
    [x] .github/workflows/ci.yml
        on: push, pull_request
        jobs:
          lint: golangci-lint run
          test: go test ./...
```

### Afternoon (3.5h) — Kubernetes + Docker Compose Validation

```
[x] k8s/namespace.yaml
[x] k8s/gateway/deployment.yaml (2 replicas, readiness/liveness probes)
[x] k8s/gateway/service.yaml (ClusterIP :8080)
[x] k8s/gateway/hpa.yaml (min:2 max:10, cpu:70%)
[x] k8s/task-service/deployment.yaml
[x] k8s/task-service/service.yaml (ClusterIP :50051)
[x] k8s/user-service/deployment.yaml + service.yaml
[x] k8s/notification-service/deployment.yaml + service.yaml

[ ] docker-compose down && docker-compose up --build
[ ] Full end-to-end smoke test:
    [ ] POST /auth/register (new user)
    [ ] POST /auth/login (get JWT)
    [ ] POST /projects (create project)
    [ ] POST /projects/:id/tasks (create task)
    [ ] POST /tasks/:id/assign (assign to worker)
    [ ] GET /notifications (verify notification created)
    [ ] PATCH /tasks/:id/status (status → in_progress)
    [ ] Try invalid transition (done → todo) → verify 400
    [ ] Try worker assigning task → verify 403
```

**Expected output:** `docker-compose up` works, all smoke tests pass.

---

## Day 6 — Polish & Presentation Prep

**Goal:** Swagger docs, README, demo script rehearsed, answers to hard questions ready.

### Morning (4h) — Documentation + Swagger

```
[x] Add swaggo annotations to all gateway handlers:
    [x] @Summary, @Param, @Success, @Failure, @Router
[x] swag init in gw-gateway (generates docs/swagger.json)
[x] Add swagger UI route to gateway
[x] Write README.md:
    [x] One-paragraph overview
    [x] Architecture diagram (copy from proposal)
    [x] Quick start: docker-compose up + sample curl commands
    [x] AWS production mapping table
    [x] Design decisions summary
[x] Add GitHub Actions CI workflow (.github/workflows/ci.yml)
    [x] Lint step + test step
[x] golangci-lint run ./... → fix all warnings
[x] go test ./... → verify all tests still pass
```

### Afternoon (4h) — Presentation Prep

```
[ ] Prepare terminal/browser layout for live demo:
    [ ] Terminal 1: docker-compose logs -f
    [ ] Terminal 2: curl commands ready
    [ ] Browser: Swagger UI at :8080/swagger/index.html
    [ ] Browser: Kibana APM at :5601 (if implemented)
[ ] Practice demo flow (see Demo Script below)
[ ] Practice "code walkthrough" — navigate to each key file:
    [ ] use-case/assign_task/assign_task.go → explain line by line
    [ ] use-case/update_task_status/update_task_status.go → show state machine
    [ ] use-case/assign_task/assign_task_test.go → explain mock setup
    [ ] domain/task_repository.go → show interface, explain why no deps
[ ] Practice Q&A answers (see Q&A Guide below)
[ ] Time yourself: aim for 12-15 minute prepared talk
```

---

## Day 7 — Extended Services: file-service + report-service

**Goal:** Demonstrate tiered S3 storage và async job pattern — hai use case justify microservice mạnh nhất.

### Morning (4h) — file-service :50054

**Use case:** Worker upload ảnh công trình + blueprint CAD, attach vào task/project. S3 tiered storage tự động migrate cold data sang Glacier.

```
[x] proto/file_service/v1/file.proto
    [x] PresignUpload(PresignUploadRequest) → PresignUploadResponse { upload_url, file_id }
    [x] ConfirmUpload(ConfirmUploadRequest) → File
    [x] GetDownloadURL(GetDownloadURLRequest) → { download_url }
    [x] ListFiles(ListFilesRequest) → FilesResponse
    [x] DeleteFile(DeleteFileRequest) → Empty
[x] apps/file-service/bootstrap/ (config, DB, gRPC server)
[x] apps/file-service/entity/model/file.go
    [x] fields: id, company_id, project_id, task_id, uploaded_by
    [x] s3_key, s3_bucket, size_bytes, mime_type
    [x] storage_tier: standard | standard_ia | glacier
    [x] status: pending | active | deleted
[x] apps/file-service/domain/file_repository.go + storage_client.go
[x] apps/file-service/repository/sql/file_repository.go
[x] apps/file-service/service/storage/
    [x] minio_client.go — implement StorageClient (presign upload/download, delete)
    [x] Demo: MinIO thay thế S3 (compatible API)
[x] apps/file-service/use-case/presign_upload/presign_upload.go
    [x] Generate presigned PUT URL (15min TTL)
    [x] Create file record với status=pending
[x] apps/file-service/use-case/confirm_upload/confirm_upload.go
    [x] Verify file tồn tại trên MinIO
    [x] Update status=active, lưu size_bytes thực tế
    [x] Publish file.uploaded event → audit-service consume
[x] apps/file-service/use-case/get_download_url/get_download_url.go
    [x] Check storage_tier: nếu glacier → return error "restore required"
    [x] Generate presigned GET URL (15min TTL)
[x] apps/file-service/api/grpc/controller/file_controller.go
[x] apps/file-service/main.go
[x] gw-gateway: thêm /files/* routes
[x] docker-compose: thêm minio service + file-service
```

**S3 Storage Tiers — Design:**
```
Standard    → active project, accessed frequently
Standard-IA → project completed > 6 months (scheduler-service trigger)
Glacier     → project archived > 2 years (scheduler-service trigger)

Lifecycle migration: scheduler gọi file-service.MigrateStorage RPC
MinIO: không có lifecycle tự động → scheduler-service giả lập bằng cron
AWS S3: dùng lifecycle policies thật
```

### Afternoon (4h) — report-service :50055

**Use case:** Manager request báo cáo tiến độ tuần → async generate PDF → notify khi xong.

```
[x] proto/report_service/v1/report.proto
    [x] RequestReport(RequestReportRequest) → { job_id, status: "queued" } (202)
    [x] GetReportStatus(GetReportStatusRequest) → ReportJob { status, download_url? }
    [x] ListReports(ListReportsRequest) → ReportsResponse
[x] apps/report-service/bootstrap/
[x] apps/report-service/entity/model/report_job.go
    [x] fields: id, company_id, requested_by, type, params (JSONB)
    [x] status: queued | processing | ready | failed
    [x] s3_key, error_msg, created_at, completed_at
[x] apps/report-service/domain/
[x] apps/report-service/repository/sql/report_repository.go
[x] apps/report-service/use-case/request_report/request_report.go
    [x] Save job record (status=queued)
    [x] Publish report.requested event → RabbitMQ
[x] apps/report-service/use-case/generate_report/generate_report.go
    [x] Query aggregation data (task completion stats)
    [x] Demo: generate JSON summary (thay PDF)
    [x] Upload result to MinIO
    [x] Update job status=ready, set s3_key
    [x] Publish report.completed → notification-service
[x] apps/report-service/consumer/report_requested_consumer.go
    [x] Consume report.requested queue
    [x] Call generate_report use case
    [x] Retry 3x với backoff
[x] apps/report-service/api/grpc/controller/report_controller.go
[x] apps/report-service/main.go
[x] gw-gateway: thêm /reports/* routes
[x] docker-compose: thêm report-service
```

**Expected output:** POST /reports/generate → job_id → poll status → GET download URL

---

## Day 8 — Extended Services: audit-service + search-service + scheduler-service

**Goal:** Complete the extended architecture. audit = compliance, search = UX, scheduler = automation.

### Morning — audit-service :50056 + search-service :50057

**audit-service:**
```
[ ] proto/audit_service/v1/audit.proto
    [ ] QueryAuditLogs(QueryRequest) → AuditLogsResponse
[ ] apps/audit-service/entity/model/audit_log.go
    [ ] id, company_id, user_id, action, resource, resource_id
    [ ] before_state (JSONB), after_state (JSONB)
    [ ] ip_address, occurred_at
[ ] apps/audit-service/repository/sql/ — append-only (INSERT only, no UPDATE/DELETE)
[ ] apps/audit-service/consumer/event_consumer.go
    [ ] Fan-out: subscribe tất cả events (task.*, file.*, user.login)
    [ ] Append vào audit_logs
[ ] apps/audit-service/api/grpc/controller/
[ ] apps/audit-service/main.go
[ ] scripts/init.sql: thêm audit_logs table với PARTITION BY RANGE(occurred_at)
[ ] gw-gateway: GET /audit?resource=task&resource_id=X
```

**search-service:**
```
[ ] proto/search_service/v1/search.proto
    [ ] Search(SearchRequest { q, types[], company_id }) → SearchResponse
[ ] apps/search-service/bootstrap/ (thêm Elasticsearch client)
[ ] apps/search-service/service/elastic/client.go
    [ ] IndexDocument(index, id, doc)
    [ ] Search(index, query) → hits
[ ] apps/search-service/consumer/indexer_consumer.go
    [ ] Consume: task.created, task.updated, project.created, file.uploaded
    [ ] Index vào Elasticsearch: tasks, projects, files indices
[ ] apps/search-service/use-case/search/search.go
    [ ] Multi-index search: query tasks + projects + files cùng lúc
    [ ] Merge và sort kết quả theo _score
[ ] apps/search-service/api/grpc/controller/
[ ] apps/search-service/main.go
[ ] docker-compose: thêm elasticsearch (single-node dev mode) + search-service
[ ] gw-gateway: GET /search?q=điện+tầng+3&types=task,file
```

### Afternoon — scheduler-service :50058

**scheduler-service:**
```
[ ] apps/scheduler-service/bootstrap/
[ ] apps/scheduler-service/job/
    [ ] deadline_checker.go — mỗi 1 phút: tasks sắp deadline (24h) → publish deadline.reminder
    [ ] overdue_checker.go — mỗi 5 phút: tasks quá hạn → publish deadline.overdue
    [ ] weekly_report.go — mỗi thứ 2 8am: publish report.requested cho active projects
    [ ] file_lifecycle.go — mỗi ngày 2am: gọi file-service migrate cold files
[ ] apps/scheduler-service/bootstrap/scheduler.go
    [ ] Dùng github.com/robfig/cron/v3
    [ ] Distributed lock: Redis SET NX "cron:<job_name>" TTL=job_interval
    [ ] Đảm bảo chỉ 1 pod chạy mỗi cron job (production-safe)
[ ] apps/scheduler-service/main.go
[ ] docker-compose: thêm scheduler-service
[ ] k8s/scheduler-service/: dùng CronJob thay Deployment (alternative approach)
[ ] gw-gateway: GET /scheduler/jobs (admin only — list scheduled jobs status)
```

**RabbitMQ topology mở rộng:**
```
[ ] scripts/init-rabbitmq.sh — setup exchanges + queues
    Exchange: constructflow.events (fanout thay vì direct)
    Queues:
      task_assigned_queue          → notification-service
      task_status_changed_queue    → notification-service
      audit_queue                  → audit-service (bind ALL events)
      search_index_queue           → search-service (bind task.*, project.*, file.*)
      report_requested_queue       → report-service
      deadline_reminder_queue      → notification-service
      deadline_overdue_queue       → notification-service + audit-service
    DLQs: <queue>.dlq cho mỗi queue
```

**Expected output:** Full 9-service system. `docker-compose up` khởi động tất cả.

---

## File-by-File Checklist

### Shared / Root

```
[ ] docker-compose.yml                        ~1h
[ ] buf.yaml + buf.gen.yaml                   ~30m
[ ] go.work                                   ~15m
[ ] scripts/init.sql (tables + indexes)       ~45m
[ ] README.md                                 ~1h
[ ] .github/workflows/ci.yml                  ~30m
```

### Proto Files

```
[ ] proto/task_service/v1/task.proto          ~45m
[ ] proto/user_service/v1/user.proto          ~30m
[ ] proto/notification_service/v1/notification.proto  ~30m
```

### task-service (~16h total)

```
[ ] main.go                                   ~30m
[ ] bootstrap/config.go                       ~30m
[ ] bootstrap/database.go                     ~20m
[ ] bootstrap/redis.go                        ~20m
[ ] bootstrap/rabbitmq.go                     ~30m
[ ] bootstrap/grpc_server.go                  ~20m
[ ] domain/task_repository.go                 ~20m
[ ] domain/project_repository.go              ~15m
[ ] domain/event_publisher.go                 ~15m
[ ] entity/model/task.go                      ~20m
[ ] entity/model/project.go                   ~15m
[ ] entity/dto/create_task_request.go         ~15m
[ ] entity/dto/assign_task_request.go         ~15m
[ ] entity/dto/update_status_request.go       ~15m
[ ] entity/dto/task_response.go               ~20m
[ ] use-case/create_task/create_task.go       ~45m
[ ] use-case/create_task/create_task_test.go  ~30m
[ ] use-case/assign_task/assign_task.go       ~1.5h ← PRIORITY
[ ] use-case/assign_task/assign_task_test.go  ~1.5h ← PRIORITY
[ ] use-case/update_task_status/update_task_status.go  ~1.5h ← PRIORITY
[ ] use-case/update_task_status/update_task_status_test.go  ~1.5h ← PRIORITY
[ ] use-case/create_project/create_project.go ~30m
[ ] use-case/create_project/create_project_test.go ~30m
[ ] use-case/list_tasks/list_tasks.go         ~45m
[ ] use-case/list_tasks/list_tasks_test.go    ~30m
[ ] repository/sql/task_repository.go         ~1h
[ ] repository/sql/project_repository.go      ~45m
[ ] service/rabbitmq_publisher.go             ~45m
[ ] api/grpc/controller/task_controller.go    ~1h
[ ] api/grpc/controller/project_controller.go ~45m
[ ] api/grpc/controller/mapper/task_mapper.go ~30m
[ ] common/errors.go                          ~20m
[ ] common/pagination.go                      ~20m
```

### user-service (~6h total)

```
[ ] main.go                                   ~20m
[ ] bootstrap/config.go                       ~20m
[ ] bootstrap/database.go                     ~15m
[ ] bootstrap/grpc_server.go                  ~15m
[ ] domain/user_repository.go                 ~15m
[ ] domain/token_service.go                   ~15m
[ ] entity/model/user.go                      ~20m
[ ] entity/dto/register_request.go            ~10m
[ ] entity/dto/login_request.go               ~10m
[ ] entity/dto/user_response.go               ~10m
[ ] use-case/register/register.go             ~45m
[ ] use-case/register/register_test.go        ~30m
[ ] use-case/login/login.go                   ~45m
[ ] use-case/login/login_test.go              ~30m
[ ] repository/sql/user_repository.go         ~45m
[ ] service/jwt_service.go                    ~45m
[ ] api/grpc/controller/user_controller.go    ~45m
```

### gw-gateway (~8h total)

```
[ ] main.go                                   ~20m
[ ] bootstrap/config.go                       ~20m
[ ] bootstrap/redis.go                        ~15m
[ ] bootstrap/grpc_clients.go                 ~30m
[ ] api/http/middleware/auth.go               ~1h
[ ] api/http/middleware/rbac.go               ~1.5h (Casbin setup is fiddly)
[ ] api/http/middleware/rate_limit.go         ~45m
[ ] api/http/handler/auth_handler.go          ~45m
[ ] api/http/handler/project_handler.go       ~1h
[ ] api/http/handler/task_handler.go          ~1h
[ ] api/http/handler/notification_handler.go  ~45m
[ ] api/http/router.go                        ~30m
[ ] casbin/model.conf                         ~20m
[ ] casbin/policy.csv                         ~15m
```

### notification-service (~6h total)

```
[ ] main.go                                   ~20m
[ ] bootstrap/config.go                       ~20m
[ ] bootstrap/database.go                     ~15m
[ ] bootstrap/redis.go                        ~15m
[ ] bootstrap/rabbitmq.go                     ~30m
[ ] bootstrap/grpc_server.go                  ~15m
[ ] domain/notification_repository.go         ~15m
[ ] entity/model/notification.go              ~20m
[ ] entity/dto/notification_response.go       ~15m
[ ] use-case/create_notification/create_notification.go  ~45m
[ ] use-case/create_notification/create_notification_test.go  ~30m
[ ] use-case/mark_read/mark_read.go           ~30m
[ ] use-case/mark_read/mark_read_test.go      ~20m
[ ] repository/sql/notification_repository.go ~45m
[ ] consumer/task_assigned_consumer.go        ~1h
[ ] consumer/task_status_changed_consumer.go  ~45m
[ ] api/grpc/controller/notification_controller.go  ~45m
```

### Kubernetes (optional — Day 5)

```
[ ] k8s/namespace.yaml                        ~10m
[ ] k8s/gateway/deployment.yaml               ~30m
[ ] k8s/gateway/service.yaml                  ~10m
[ ] k8s/gateway/hpa.yaml                      ~15m
[ ] k8s/task-service/deployment.yaml          ~20m
[ ] k8s/task-service/service.yaml             ~10m
[ ] k8s/user-service/deployment.yaml          ~20m
[ ] k8s/user-service/service.yaml             ~10m
[ ] k8s/notification-service/deployment.yaml  ~20m
[ ] k8s/notification-service/service.yaml     ~10m
```

---

## Testing Checklist

### Unit Tests (target: all passing before Day 5)

```
task-service:
[x] TestAssignTask_Success
[x] TestAssignTask_LockFailed
[x] TestAssignTask_TaskNotFound
[x] TestAssignTask_CrossTenantBlocked
[x] TestAssignTask_PublishFails
[x] TestUpdateStatus_TodoToInProgress_Allowed
[x] TestUpdateStatus_InProgressToDone_Allowed
[x] TestUpdateStatus_InProgressToBlocked_Allowed
[x] TestUpdateStatus_BlockedToInProgress_Allowed
[x] TestUpdateStatus_DoneToInProgress_ManagerAllowed
[x] TestUpdateStatus_DoneToInProgress_WorkerForbidden
[x] TestUpdateStatus_InvalidTransition_DoneToTodo
[x] TestCreateTask_Success
[x] TestCreateTask_InvalidInput
[ ] TestCreateProject_Success

user-service:
[x] TestRegister_Success
[x] TestRegister_EmailAlreadyExists
[x] TestLogin_Success
[x] TestLogin_WrongPassword
[x] TestLogin_UserNotFound

notification-service:
[ ] TestCreateNotification_Success
[ ] TestCreateNotification_DuplicateEvent_Skipped
[ ] TestMarkRead_Success
[ ] TestMarkRead_WrongUser
```

### Integration / Manual Tests (Day 5 smoke test)

```
[ ] Full flow: register → login → create project → create task → assign → check notification
[ ] Error case: invalid JWT → 401
[ ] Error case: worker tries to assign → 403
[ ] Error case: invalid status transition → 400 + correct error code
[ ] Error case: task not found → 404
[ ] Concurrent assignment (run 2 curl commands simultaneously) → only 1 succeeds
[ ] docker-compose down && docker-compose up → data persists (volumes)
```

### Run Commands

```bash
# All tests
cd apps/task-service && go test ./... -v

# Specific test
cd apps/task-service && go test ./use-case/assign_task/ -run TestAssignTask_Success -v

# With coverage
cd apps/task-service && go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out

# Lint
golangci-lint run ./...
```

---

## Demo Script (Live Demo — 12-15 minutes prepared)

### Setup (before interviewer joins)

```bash
# Terminal 1 — start everything
docker-compose up

# Terminal 2 — ready for curl commands
# Have these commands pre-typed, ready to run
```

### Flow to Demo

```bash
# 1. Register manager
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"manager@acme.com","name":"Suzuki Manager","password":"secret123","role":"manager","company_name":"ACME Construction"}'

# 2. Register worker
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"worker@acme.com","name":"Tanaka Worker","password":"secret123","role":"worker","company_id":"<company_id>"}'

# 3. Login as manager → copy JWT
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"manager@acme.com","password":"secret123"}' | jq .

# 4. Create project
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $MANAGER_JWT" \
  -d '{"name":"Building A","description":"Main construction site"}'

# 5. Create task
curl -X POST http://localhost:8080/api/v1/projects/$PROJECT_ID/tasks \
  -H "Authorization: Bearer $MANAGER_JWT" \
  -d '{"title":"Install electrical wiring - Floor 3","priority":"high","due_date":"2026-03-15"}'

# 6. Assign task — show distributed lock
curl -X POST http://localhost:8080/api/v1/tasks/$TASK_ID/assign \
  -H "Authorization: Bearer $MANAGER_JWT" \
  -d '{"assigned_to":"$WORKER_ID"}'

# 7. Check notification (worker's perspective)
curl -X GET http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $WORKER_JWT"
# → notification appears! (async, via RabbitMQ)

# 8. Worker updates status
curl -X PATCH http://localhost:8080/api/v1/tasks/$TASK_ID/status \
  -H "Authorization: Bearer $WORKER_JWT" \
  -d '{"status":"in_progress"}'

# 9. Try invalid transition
curl -X PATCH http://localhost:8080/api/v1/tasks/$TASK_ID/status \
  -H "Authorization: Bearer $WORKER_JWT" \
  -d '{"status":"todo"}'
# → 400 INVALID_STATUS_TRANSITION

# 10. Try worker assigning (should fail)
curl -X POST http://localhost:8080/api/v1/tasks/$TASK_ID/assign \
  -H "Authorization: Bearer $WORKER_JWT" \
  -d '{"assigned_to":"$WORKER_ID"}'
# → 403 Forbidden
```

---

## Q&A Guide — Anticipated Hard Questions

### Architecture

**Q: "Why microservices over a monolith?"**
> "For a 4-entity domain at this scale, a monolith would actually be simpler. I chose microservices deliberately to demonstrate the patterns ANDPAD would care about: service decomposition, gRPC contracts, event-driven decoupling. In production, I'd actually start with a modular monolith and extract services only where scale demands it."

**Q: "Your gateway is a single point of failure. How do you handle that?"**
> "In the K8s setup the gateway runs as 2+ replicas behind a load balancer, with HPA scaling up to 10 on CPU. The gateway is stateless — it holds no session state (JWT is validated from the public key on every request), so horizontal scaling is trivial. Redis holds all shared state (rate limiting counters), which would use ElastiCache in production."

**Q: "Why direct exchange in RabbitMQ instead of topic exchange?"**
> "With two event types and two consumers, direct exchange is simpler and more explicit — each queue gets exactly the events it needs with no routing ambiguity. A topic exchange would make sense if I wanted one consumer to receive multiple event types with wildcard routing — for example, `notification.*` to get both assignment and status events. That pattern becomes valuable when adding more event types."

### Database

**Q: "Why PostgreSQL and not MySQL?"**
> "PostgreSQL gives me JSONB for the notification metadata column — efficient binary storage with indexing capability, avoiding the need for a separate notifications_metadata table. The partial indexes (`WHERE is_read = FALSE`) are also native in PostgreSQL and reduce index size significantly. In ANDPAD's Aurora MySQL environment, the JSONB column would become a JSON column (slightly different but functionally similar), and partial indexes would be replaced with covering indexes. The design is portable."

**Q: "Your email constraint is globally unique. What if a user wants to work for multiple companies?"**
> "Good catch. This is a deliberate simplification — users belong to one company in this design. A production SaaS platform would use a `user_company_memberships` join table with roles per company. I'd be happy to walk through how that schema change would propagate through the JWT claims and multi-tenant scoping."

### Code

**Q: "Walk me through what happens if the Redis lock TTL expires during AssignTask."**
> "Good edge case. If the 5s TTL expires while our DB write is in progress, another goroutine can acquire the lock. When our defer fires, we delete the new goroutine's lock key — not ours. This could lead to a concurrent assignment window. The correct fix is a Lua script for atomic CAS-delete: `if GET key == our_value then DEL key end`. For this demo I accepted the race window given the 5s TTL makes it very unlikely, but I flagged it as a known limitation."

**Q: "What happens if RabbitMQ publish fails in AssignTask?"**
> "Currently, the task gets assigned (DB updated) but the notification may never be sent. I chose to not fail the assignment — the core business operation succeeds, the notification is a secondary concern. The production fix is the transactional outbox pattern: write an `events` record in the same DB transaction as the task update, then have a background poller publish pending events to RabbitMQ and mark them sent. This gives exactly-once delivery guarantees."

**Q: "How does your Casbin model handle a manager from Company A trying to access Company B's data?"**
> "Casbin uses domain-scoped RBAC — the policy is `(subject, domain, resource, action)` where domain is `company_id`. A manager from Company A has policies for `company_A` only. Even if they obtained a valid JWT, the enforcer would reject `Enforce("manager", "company_B", "tasks", "write")` because no such policy exists. The JWT's `company_id` claim is injected into every enforcement call by the middleware."

### Testing

**Q: "What's your testing philosophy?"**
> "Test the use case layer thoroughly with mocked infrastructure — this gives fast, deterministic tests that don't require DB or Redis. The repository layer I leave to integration tests (or trust GORM's behavior and write a few critical query tests against a test DB). I never test the controller layer with unit tests — it's just mapping, and integration tests cover it. The highest ROI tests are on state machine transitions and business invariants like multi-tenant isolation."

### Culture Fit

**Q: "Tell me about a time you disagreed with a technical decision."**
> Prepare a specific example. Structure: situation → your position → how you argued it (data, not opinion) → outcome. Show you can escalate a disagreement professionally and accept the final call.

**Q: "How do you handle tech debt in a fast-moving team?"**
> "I track it explicitly — in this project I seeded a DEBT section in the knowledge base from day one: no DB migration files, no DLQ replay, Redis single point of failure. Making debt visible is the first step. I advocate for a 20% buffer in sprint planning for debt reduction — similar to the Google 20% rule. The key is agreeing on debt explicitly rather than pretending it doesn't exist."

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Day 3 overrun (too much in one day) | High | High | Split Day 3/4 as per revised plan above |
| Elastic APM setup complexity | High | Medium | Drop entirely if behind; cite as future work |
| Casbin setup bugs | Medium | Medium | Use file adapter first (simpler), DB adapter later |
| RabbitMQ consumer retry logic | Medium | Medium | Start with basic ACK/NACK, add backoff after basic flow works |
| gRPC stub version conflicts | Low | High | Pin exact buf + grpc-go versions, generate stubs on Day 1 |
| Docker networking issues | Low | Medium | Test `docker-compose up` on Day 1, fix networking early |
| Proto/Go type mismatches | Low | Medium | Write proto → generate → compile → fix on Day 1 only |

### Definitive Cut-List (ordered by priority to cut)

1. **Elastic APM** — drop if Day 3 or Day 4 overruns by >2h
2. **Kubernetes manifests** — drop if Day 5 morning is needed for catching up
3. **`/auth/refresh` endpoint** — low value, cut freely
4. **`/notifications/read-all`** — cut freely
5. **List tasks with sort/filter** — basic pagination is enough
6. **Swagger annotations** — mention it as "annotations exist, run swag init to generate"
7. **`/tasks/my` endpoint** — fix the routing bug (rename or use query param), then cut if no time

### What to Never Cut

- `docker-compose up` bringing up all 4 services ← demo requires this
- `AssignTask` use case with distributed lock ← most differentiating feature
- `UpdateTaskStatus` state machine ← core business logic showcase
- Unit tests for the above ← explicitly in JD requirements
- JWT auth + basic RBAC in gateway ← security requirement in JD
- At least one RabbitMQ event flowing end-to-end ← event-driven is in the pitch
