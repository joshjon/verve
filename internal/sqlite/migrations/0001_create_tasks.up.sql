CREATE TABLE repo (
    id         TEXT PRIMARY KEY,
    owner      TEXT NOT NULL,
    name       TEXT NOT NULL,
    full_name  TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE task (
    id               TEXT PRIMARY KEY,
    repo_id          TEXT NOT NULL REFERENCES repo(id),
    description      TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK(status IN ('pending', 'running', 'review', 'merged', 'closed', 'failed')),
    pull_request_url TEXT,
    pr_number        INTEGER,
    depends_on       TEXT NOT NULL DEFAULT '[]',
    close_reason     TEXT,
    attempt          INTEGER NOT NULL DEFAULT 1,
    max_attempts     INTEGER NOT NULL DEFAULT 5,
    retry_reason     TEXT,
    created_at       DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at       DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_task_repo_id ON task(repo_id);
CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_status_pr ON task(status, pr_number) WHERE pr_number IS NOT NULL;

CREATE TABLE task_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    TEXT    NOT NULL REFERENCES task(id),
    lines      TEXT    NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_task_log_task_id ON task_log(task_id);
