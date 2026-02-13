CREATE TABLE task (
    id               TEXT PRIMARY KEY,
    description      TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK(status IN ('pending', 'running', 'review', 'merged', 'closed', 'failed')),
    logs             TEXT NOT NULL DEFAULT '[]',
    pull_request_url TEXT,
    pr_number        INTEGER,
    depends_on       TEXT NOT NULL DEFAULT '[]',
    close_reason     TEXT,
    created_at       DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at       DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_status_pr ON task(status, pr_number) WHERE pr_number IS NOT NULL;
