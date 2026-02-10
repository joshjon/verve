-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(20) PRIMARY KEY,
    description TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    logs TEXT[] NOT NULL DEFAULT '{}',
    pull_request_url TEXT,
    pr_number INTEGER,
    depends_on TEXT[] NOT NULL DEFAULT '{}',
    close_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding pending tasks efficiently
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Index for finding tasks in review for sync
CREATE INDEX IF NOT EXISTS idx_tasks_status_pr ON tasks(status, pr_number) WHERE pr_number IS NOT NULL;
