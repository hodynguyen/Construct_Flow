# E2E API Test Results

**Date**: 2026-03-10
**Environment**: Docker Compose (local), all 16 containers running
**Result**: **34/34 tests passed**

---

## Test Summary

| Category            | Tests  | Passed | Status |
|---------------------|--------|--------|--------|
| Health & Setup      | 6      | 6      | PASS   |
| Authentication      | 4      | 4      | PASS   |
| Project CRUD        | 6      | 6      | PASS   |
| Task CRUD           | 3      | 3      | PASS   |
| Task Assignment     | 2      | 2      | PASS   |
| Task State Machine  | 7      | 7      | PASS   |
| Notifications       | 4      | 4      | PASS   |
| Delete & Soft Delete| 2      | 2      | PASS   |
| **Total**           | **34** | **34** | **PASS** |

---

## Test Details

### Setup (Tests 1-6)

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 1 | Health check | `GET /health` | 200 | 200 `{"status":"ok"}` | PASS |
| 2 | Register admin | `POST /api/v1/auth/register` | 201 | 201 (user + company created) | PASS |
| 3 | Login admin | `POST /api/v1/auth/login` | 200 | 200 (access_token returned) | PASS |
| 4 | Register manager (same company) | `POST /api/v1/auth/register` | 201 | 201 | PASS |
| 5 | Login manager | `POST /api/v1/auth/login` | 200 | 200 | PASS |
| 6 | Register + login worker | `POST /api/v1/auth/register` | 201 | 201 | PASS |

### Authentication (Tests 7-10)

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 7 | Duplicate email registration | `POST /api/v1/auth/register` | 409 | 409 `"email already registered"` | PASS |
| 8 | Login with wrong password | `POST /api/v1/auth/login` | 401 | 401 `"invalid email or password"` | PASS |
| 9 | Request without auth header | `GET /api/v1/projects` | 401 | 401 `"missing or malformed Authorization header"` | PASS |
| 10 | Request with valid token | `GET /api/v1/projects` | 200 | 200 (empty list returned) | PASS |

### Project CRUD (Tests 11-16)

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 11 | Create project (manager) | `POST /api/v1/projects` | 201 | 201 (project object returned) | PASS |
| 12 | Get project by ID | `GET /api/v1/projects/:id` | 200 | 200 (project object) | PASS |
| 13 | List projects | `GET /api/v1/projects` | 200 | 200 (with `page`, `page_size`, `total`) | PASS |
| 14 | Update project | `PUT /api/v1/projects/:id` | 200 | 200 (name/description updated) | PASS |
| 15 | Create project as worker (RBAC) | `POST /api/v1/projects` | 403 | 403 `"insufficient permissions"` | PASS |
| 16 | Create second project (for delete) | `POST /api/v1/projects` | 201 | 201 | PASS |

### Task CRUD (Tests 17-19)

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 17 | Create task (manager) | `POST /api/v1/projects/:id/tasks` | 201 | 201 (task with status=todo, priority=high) | PASS |
| 18 | Get task by ID | `GET /api/v1/tasks/:id` | 200 | 200 (task object) | PASS |
| 19 | List tasks in project | `GET /api/v1/projects/:id/tasks` | 200 | 200 (with pagination) | PASS |

### Task Assignment (Tests 20-21)

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 20 | Manager assigns task to worker | `POST /api/v1/tasks/:id/assign` | 200 | 200 (assigned_to set to worker ID) | PASS |
| 21 | Worker tries to assign (RBAC) | `POST /api/v1/tasks/:id/assign` | 403 | 403 `"insufficient permissions"` | PASS |

Assignment uses Redis distributed lock (`SET NX lock:task:assign:<taskID>` TTL=5s) to prevent concurrent double-assignment.

### Task State Machine (Tests 22-28)

Tests the enforced state transitions:

```
todo ──► in_progress ──► done
              │  ▲          │
              ▼  │          │ (manager/admin only)
           blocked          ▼
                        in_progress
```

| # | Test | Transition | Actor | Expected | Actual | Result |
|---|------|-----------|-------|----------|--------|--------|
| 22 | Valid transition | todo → in_progress | worker | 200 | 200 | PASS |
| 23 | Valid transition | in_progress → blocked | worker | 200 | 200 | PASS |
| 24 | Resume from blocked | blocked → in_progress | worker | 200 | 200 | PASS |
| 25 | Complete task | in_progress → done | worker | 200 | 200 | PASS |
| 26 | Worker reopen (denied) | done → in_progress | worker | 403 | 403 | PASS |
| 27 | Manager reopen (allowed) | done → in_progress | manager | 200 | 200 | PASS |
| 28 | Invalid transition | in_progress → todo | manager | 400 | 400 `"invalid task status transition"` | PASS |

### Notifications (Tests 29-32)

Event flow: task-service → RabbitMQ (`task.assigned`) → notification-service → PostgreSQL

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 29 | Get worker notifications | `GET /api/v1/notifications` | has notifications | 1 notification (type=task_assigned) | PASS |
| 30 | Get unread count | `GET /api/v1/notifications/unread/count` | count > 0 | `{"count":1}` | PASS |
| 31 | Mark as read | `PATCH /api/v1/notifications/:id/read` | 204 | 204 | PASS |
| 32 | Unread count after mark-read | `GET /api/v1/notifications/unread/count` | count = 0 | `{"count":0}` | PASS |

### Delete & Soft Delete (Tests 33-34)

| # | Test | Method | Expected | Actual | Result |
|---|------|--------|----------|--------|--------|
| 33 | Delete project (manager) | `DELETE /api/v1/projects/:id` | 204 | 204 | PASS |
| 34 | Get deleted project | `GET /api/v1/projects/:id` | 404 | 404 `"resource not found"` | PASS |

Uses GORM soft delete (`deleted_at IS NOT NULL`).

---

## Bugs Found and Fixed During Testing

### Bug 1: Duplicate Email Registration Allowed (Fixed)

**Symptom**: Registering the same email in a different company returned 201 instead of 409.
**Root cause**: `ExistsByEmail()` in `apps/user-service/repository/sql/user_repository.go` checked `WHERE email = ? AND company_id = ?` — scoped to company, not global.
**Fix**: Removed `company_id` from the uniqueness check. Email is now globally unique since login uses email without company context.
**File**: `apps/user-service/repository/sql/user_repository.go:57`

### Bug 2: Workers Cannot Mark Notifications as Read (Fixed)

**Symptom**: `PATCH /api/v1/notifications/:id/read` returned 403 for workers.
**Root cause**: Casbin policy only granted `write` on `/notifications` to `admin`. Workers and managers only had `read`.
**Fix**: Added `write` permission for `worker` and `manager` roles on `/notifications`.
**Files**: `casbin/policy.csv`, `apps/gw-gateway/casbin/policy.csv`

### Issue 3: Stale AMQP Channel (Operational)

**Symptom**: After 2 days of container uptime, task-service stopped publishing events to RabbitMQ.
**Root cause**: AMQP channel created once at startup, never reconnected. Channel silently died, and `_ = publisher.Publish(...)` swallowed the error.
**Workaround**: Restart task-service container to get a fresh AMQP channel.
**Production fix**: Implement channel reconnection logic or use a connection pool with health checks.

---

## Coverage Matrix

| Feature | Positive Test | Negative Test | RBAC Test |
|---------|:---:|:---:|:---:|
| User Registration | #2, #4, #6 | #7 (dup email) | - |
| User Login | #3, #5 | #8 (wrong pass) | - |
| JWT Authentication | #10 | #9 (no header) | - |
| Project Create | #11 | - | #15 (worker blocked) |
| Project Read | #12, #13 | - | - |
| Project Update | #14 | - | - |
| Project Delete | #33 | #34 (404 after) | - |
| Task Create | #17 | - | - |
| Task Read | #18, #19 | - | - |
| Task Assign | #20 | - | #21 (worker blocked) |
| State: todo→in_progress | #22 | - | - |
| State: in_progress→blocked | #23 | - | - |
| State: blocked→in_progress | #24 | - | - |
| State: in_progress→done | #25 | - | - |
| State: done→in_progress | #27 | - | #26 (worker blocked) |
| State: invalid transition | - | #28 | - |
| Notification List | #29 | - | - |
| Notification Count | #30, #32 | - | - |
| Notification Mark Read | #31 | - | - |

---

## How to Reproduce

```bash
# 1. Start all services
docker-compose up -d

# 2. Wait for healthy status
docker-compose ps

# 3. Run health check
curl http://localhost:8080/health

# 4. Follow the test sequence above using curl or Swagger UI at:
#    http://localhost:8080/swagger/index.html
```
