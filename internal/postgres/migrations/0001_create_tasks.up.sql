CREATE TABLE repo
(
    id         TEXT PRIMARY KEY,
    owner      TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    full_name  TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TYPE task_status AS ENUM (
    'pending',
    'running',
    'review',
    'merged',
    'closed',
    'failed'
    );

CREATE TABLE task
(
    id                   TEXT PRIMARY KEY,
    repo_id              TEXT             NOT NULL REFERENCES repo (id),
    description          TEXT             NOT NULL,
    status               task_status      NOT NULL DEFAULT 'pending',
    pull_request_url     TEXT,
    pr_number            INTEGER,
    depends_on           TEXT[]           NOT NULL DEFAULT '{}',
    close_reason         TEXT,
    attempt              INTEGER          NOT NULL DEFAULT 1,
    max_attempts         INTEGER          NOT NULL DEFAULT 5,
    retry_reason         TEXT,
    acceptance_criteria  TEXT,
    agent_status         TEXT,
    retry_context        TEXT,
    consecutive_failures INTEGER          NOT NULL DEFAULT 0,
    cost_usd             DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_cost_usd         DOUBLE PRECISION,
    created_at           TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_repo_id ON task (repo_id);
CREATE INDEX idx_task_status ON task (status);
CREATE INDEX idx_task_status_pr ON task (status, pr_number) WHERE pr_number IS NOT NULL;

CREATE TABLE task_log
(
    id         BIGSERIAL PRIMARY KEY,
    task_id    TEXT        NOT NULL REFERENCES task (id),
    lines      TEXT[]      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_log_task_id ON task_log (task_id);
