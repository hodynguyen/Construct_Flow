-- ConstructFlow — Database Schema
-- PostgreSQL 16

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- enables gen_random_uuid()

-- ── companies ─────────────────────────────────────────────────────────────────
-- Root tenant entity. Every other table is scoped by company_id.
CREATE TABLE IF NOT EXISTS companies (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── users ─────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID        NOT NULL REFERENCES companies(id),
    email      VARCHAR(255) NOT NULL,
    name       VARCHAR(255) NOT NULL,
    password   VARCHAR(255) NOT NULL, -- bcrypt hash (cost=12)
    role       VARCHAR(50)  NOT NULL DEFAULT 'worker', -- admin | manager | worker
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ  -- soft delete

    -- Email must be unique within the same company.
    -- One email may register for one company only (deliberate simplification).
);

CREATE UNIQUE INDEX idx_users_email_company  ON users(email, company_id);
CREATE        INDEX idx_users_company        ON users(company_id);
CREATE        INDEX idx_users_deleted        ON users(deleted_at) WHERE deleted_at IS NULL;

-- ── projects ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS projects (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id  UUID        NOT NULL REFERENCES companies(id),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(50)  NOT NULL DEFAULT 'active', -- active | completed | archived
    start_date  DATE,
    end_date    DATE,
    created_by  UUID        REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_projects_company        ON projects(company_id);
CREATE INDEX idx_projects_company_status ON projects(company_id, status);
CREATE INDEX idx_projects_deleted        ON projects(deleted_at) WHERE deleted_at IS NULL;

-- ── tasks ─────────────────────────────────────────────────────────────────────
-- company_id is denormalized here (also on projects) to avoid JOIN on hot queries.
CREATE TABLE IF NOT EXISTS tasks (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id),
    company_id  UUID        NOT NULL,  -- denormalized for tenant-scoped query performance
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(50)  NOT NULL DEFAULT 'todo',   -- todo | in_progress | done | blocked
    priority    VARCHAR(50)  NOT NULL DEFAULT 'medium', -- low | medium | high | critical
    assigned_to UUID        REFERENCES users(id),
    created_by  UUID        NOT NULL REFERENCES users(id),
    due_date    DATE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

-- Primary query pattern: list tasks by company + project
CREATE INDEX idx_tasks_company_project   ON tasks(company_id, project_id);
-- Filter by status within a company
CREATE INDEX idx_tasks_company_status    ON tasks(company_id, status);
-- Worker's task list
CREATE INDEX idx_tasks_assigned          ON tasks(assigned_to) WHERE deleted_at IS NULL;
-- Overdue task alerts (only active tasks)
CREATE INDEX idx_tasks_due_date          ON tasks(due_date) WHERE status != 'done' AND deleted_at IS NULL;
-- Priority queue (open tasks only)
CREATE INDEX idx_tasks_priority          ON tasks(company_id, priority) WHERE status = 'todo';
CREATE INDEX idx_tasks_deleted           ON tasks(deleted_at) WHERE deleted_at IS NULL;

-- ── notifications ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS notifications (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id),
    company_id UUID        NOT NULL,
    type       VARCHAR(50)  NOT NULL, -- task_assigned | status_changed
    title      VARCHAR(255) NOT NULL,
    message    TEXT,
    is_read    BOOLEAN      NOT NULL DEFAULT FALSE,
    metadata   JSONB,        -- {task_id, project_id, old_status, new_status, assigned_by, ...}
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- No updated_at: notifications are immutable once created
    -- No deleted_at: notifications are kept for audit trail
);

-- "Get my unread notifications" — most frequent query
CREATE INDEX idx_notifications_user_unread ON notifications(user_id, is_read) WHERE is_read = FALSE;
-- "Get my notification history" — paginated, descending
CREATE INDEX idx_notifications_user_time   ON notifications(user_id, created_at DESC);
