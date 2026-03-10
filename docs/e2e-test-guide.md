# ConstructFlow — E2E Test Guide (Swagger UI)

Swagger UI: **http://localhost:8080/swagger/index.html**

> Follow the steps in order. Copy the required output from each step for the next one.

---

## Setup: Authorize on Swagger

After getting `access_token` from the Login step:
1. Click the **Authorize** button (top right of Swagger UI)
2. Paste: `Bearer <access_token>`
3. Click **Authorize** → **Close**

All subsequent requests will automatically include the `Authorization` header.

---

## Scenario 1 — Happy Path: Full Task Lifecycle

### Step 1 — Register user

**POST** `/api/v1/auth/register`

```json
{
  "email": "manager@acme.com",
  "password": "secret123",
  "name": "Alice Manager",
  "company_id": "11111111-1111-1111-1111-111111111111",
  "role": "manager"
}
```

Expected: `201` — user object returned. Save the `id` (used as `assignee_id` in step 8).

---

### Step 2 — Login

**POST** `/api/v1/auth/login`

```json
{
  "email": "manager@acme.com",
  "password": "secret123"
}
```

Expected: `200`

```json
{
  "access_token": "eyJ...",
  "user": { "id": "...", "role": "manager" }
}
```

Copy `access_token` → paste into Swagger **Authorize**.

---

### Step 3 — Create Project

**POST** `/api/v1/projects`

```json
{
  "name": "Tower B Construction",
  "description": "Phase 2 office tower"
}
```

Expected: `201` — save `project.id`.

---

### Step 4 — List Projects

**GET** `/api/v1/projects`

Expected: `200` — array containing the project just created.

---

### Step 5 — Get Project by ID

**GET** `/api/v1/projects/{id}` — use `id` from step 3.

Expected: `200` — project detail.

---

### Step 6 — Create Task

**POST** `/api/v1/projects/{id}/tasks` — `{id}` is the project id.

```json
{
  "title": "Pour concrete floor 3",
  "description": "Mix ratio 1:2:3, cure 28 days",
  "priority": "high"
}
```

Expected: `201` — `task.status` = `"todo"`. Save `task.id`.

---

### Step 7 — List Tasks in Project

**GET** `/api/v1/projects/{id}/tasks`

Expected: `200` — array with 1 task, status `todo`.

---

### Step 8 — Assign Task

**POST** `/api/v1/tasks/{id}/assign` — `{id}` is the task id.

```json
{
  "assigned_to": "<user_id from step 1>"
}
```

Expected: `200` — `task.assigned_to` = user id.

> Trigger: RabbitMQ publishes `task.assigned` event → notification-service creates a notification.

---

### Step 9 — Check Notification

**GET** `/api/v1/notifications`

Expected: `200` — 1 notification with type `task_assigned`.

---

### Step 10 — Get Unread Count

**GET** `/api/v1/notifications/unread/count`

Expected: `200`

```json
{ "count": 1 }
```

---

### Step 11 — Update Task Status: todo → in_progress

**PATCH** `/api/v1/tasks/{id}/status`

```json
{ "status": "in_progress" }
```

Expected: `200` — `task.status` = `"in_progress"`.

---

### Step 12 — Update Task Status: in_progress → done

**PATCH** `/api/v1/tasks/{id}/status`

```json
{ "status": "done" }
```

Expected: `200` — `task.status` = `"done"`.

---

### Step 13 — Mark Notification as Read

**PATCH** `/api/v1/notifications/{id}/read` — `{id}` is the notification id from step 9.

Expected: `200`.

---

### Step 14 — Verify Unread Count = 0

**GET** `/api/v1/notifications/unread/count`

Expected: `200`

```json
{ "count": 0 }
```

---

## Scenario 2 — State Machine: Invalid Transitions

### Test 2a — `todo` → `done` (invalid, must go through `in_progress`)

**PATCH** `/api/v1/tasks/{id}/status` (use a new task with status = `todo`)

```json
{ "status": "done" }
```

Expected: `400` — error message about invalid transition.

---

### Test 2b — `done` → `todo` (invalid)

**PATCH** `/api/v1/tasks/{id}/status` (task currently `done`)

```json
{ "status": "todo" }
```

Expected: `400` — not allowed.

---

### Test 2c — `in_progress` → `blocked`

**PATCH** `/api/v1/tasks/{id}/status`

```json
{ "status": "blocked" }
```

Expected: `200` — valid transition.

---

### Test 2d — `blocked` → `in_progress` (resume)

**PATCH** `/api/v1/tasks/{id}/status`

```json
{ "status": "in_progress" }
```

Expected: `200`.

---

## Scenario 3 — RBAC: Permission Denied

Register a user with `worker` role:

**POST** `/api/v1/auth/register`

```json
{
  "email": "worker@acme.com",
  "password": "secret123",
  "name": "Bob Worker",
  "company_id": "11111111-1111-1111-1111-111111111111",
  "role": "worker"
}
```

Login as worker, re-authorize on Swagger.

### Test 3a — Worker cannot assign task

**POST** `/api/v1/tasks/{id}/assign`

Expected: `403 Forbidden` — `"insufficient permissions"`.

### Test 3b — Worker cannot create project

**POST** `/api/v1/projects`

Expected: `403 Forbidden`.

### Test 3c — Worker can read tasks

**GET** `/api/v1/projects/{id}/tasks`

Expected: `200` — read is allowed.

---

## Scenario 4 — Auth: Invalid Token

Remove Authorization header (click Authorize → Logout).

**GET** `/api/v1/projects`

Expected: `401 Unauthorized`.

---

## Scenario 5 — Rate Limiting

Send more than 60 requests per minute to any endpoint:

```bash
for i in $(seq 1 70); do
  curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health
done
```

Expected: After ~60 requests → `429 Too Many Requests`.

---

## Quick Reference

| Data | Value |
|------|-------|
| Base URL | `http://localhost:8080` |
| Swagger UI | `http://localhost:8080/swagger/index.html` |
| Test company_id | `11111111-1111-1111-1111-111111111111` |
| Admin email | `admin@acme.com` / `secret123` |

### Task Status Valid Transitions

```
todo → in_progress
in_progress → done
in_progress → blocked
blocked → in_progress
done → in_progress  (manager/admin only, for rework)
```
