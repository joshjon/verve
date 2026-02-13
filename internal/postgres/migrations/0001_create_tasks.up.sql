CREATE TYPE task_status AS ENUM (
    'pending',
    'running',
    'review',
    'merged',
    'closed',
    'failed'
);

CREATE TABLE task (
    id               TEXT        PRIMARY KEY,
    description      TEXT        NOT NULL,
    status           task_status NOT NULL DEFAULT 'pending',
    logs             TEXT[]      NOT NULL DEFAULT '{}',
    pull_request_url TEXT,
    pr_number        INTEGER,
    depends_on       TEXT[]      NOT NULL DEFAULT '{}',
    close_reason     TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_status_pr ON task(status, pr_number) WHERE pr_number IS NOT NULL;
