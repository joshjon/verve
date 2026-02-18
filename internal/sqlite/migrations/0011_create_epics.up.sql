CREATE TABLE epic (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL REFERENCES repo(id),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'draft'
                    CHECK(status IN ('draft', 'planning', 'ready', 'active', 'completed', 'closed')),
    proposed_tasks  TEXT NOT NULL DEFAULT '[]',
    task_ids        TEXT NOT NULL DEFAULT '[]',
    planning_prompt TEXT,
    session_log     TEXT NOT NULL DEFAULT '[]',
    not_ready       INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_epic_repo_id ON epic(repo_id);
CREATE INDEX idx_epic_status ON epic(status);

ALTER TABLE task ADD COLUMN epic_id TEXT REFERENCES epic(id);
CREATE INDEX idx_task_epic_id ON task(epic_id) WHERE epic_id IS NOT NULL;
