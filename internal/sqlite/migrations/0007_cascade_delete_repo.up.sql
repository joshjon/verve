-- Add ON DELETE CASCADE to foreign keys referencing repo and task.
-- SQLite does not support ALTER CONSTRAINT, so we recreate affected tables.

-- Step 1: Recreate task_log with ON DELETE CASCADE on task_id.
CREATE TABLE task_log_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    TEXT    NOT NULL REFERENCES task(id) ON DELETE CASCADE,
    lines      TEXT    NOT NULL DEFAULT '[]',
    attempt    INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
INSERT INTO task_log_new (id, task_id, lines, attempt, created_at)
    SELECT id, task_id, lines, attempt, created_at FROM task_log;
DROP TABLE task_log;
ALTER TABLE task_log_new RENAME TO task_log;
CREATE INDEX idx_task_log_task_id ON task_log(task_id);

-- Step 2: Recreate task with ON DELETE CASCADE on repo_id and ON DELETE SET NULL on epic_id.
CREATE TABLE task_new (
    id                       TEXT PRIMARY KEY,
    repo_id                  TEXT    NOT NULL REFERENCES repo(id) ON DELETE CASCADE,
    title                    TEXT    NOT NULL DEFAULT '',
    description              TEXT    NOT NULL,
    status                   TEXT    NOT NULL DEFAULT 'pending'
                             CHECK(status IN ('pending', 'running', 'review', 'merged', 'closed', 'failed')),
    pull_request_url         TEXT,
    pr_number                INTEGER,
    depends_on               TEXT    NOT NULL DEFAULT '[]',
    close_reason             TEXT,
    attempt                  INTEGER NOT NULL DEFAULT 1,
    max_attempts             INTEGER NOT NULL DEFAULT 5,
    retry_reason             TEXT,
    acceptance_criteria_list TEXT    NOT NULL DEFAULT '[]',
    agent_status             TEXT,
    retry_context            TEXT,
    consecutive_failures     INTEGER NOT NULL DEFAULT 0,
    cost_usd                 REAL    NOT NULL DEFAULT 0,
    max_cost_usd             REAL,
    skip_pr                  INTEGER NOT NULL DEFAULT 0,
    draft_pr                 INTEGER NOT NULL DEFAULT 0,
    branch_name              TEXT,
    model                    TEXT,
    started_at               INTEGER,
    ready                    INTEGER NOT NULL DEFAULT 1,
    last_heartbeat_at        INTEGER,
    epic_id                  TEXT    REFERENCES epic(id) ON DELETE SET NULL,
    created_at               INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at               INTEGER NOT NULL DEFAULT (unixepoch())
);
INSERT INTO task_new (id, repo_id, title, description, status, pull_request_url, pr_number,
    depends_on, close_reason, attempt, max_attempts, retry_reason, acceptance_criteria_list,
    agent_status, retry_context, consecutive_failures, cost_usd, max_cost_usd, skip_pr, draft_pr,
    branch_name, model, started_at, ready, last_heartbeat_at, epic_id, created_at, updated_at)
    SELECT id, repo_id, title, description, status, pull_request_url, pr_number,
    depends_on, close_reason, attempt, max_attempts, retry_reason, acceptance_criteria_list,
    agent_status, retry_context, consecutive_failures, cost_usd, max_cost_usd, skip_pr, draft_pr,
    branch_name, model, started_at, ready, last_heartbeat_at, epic_id, created_at, updated_at
    FROM task;
DROP TABLE task;
ALTER TABLE task_new RENAME TO task;
CREATE INDEX idx_task_repo_id ON task(repo_id);
CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_status_pr ON task(status, pr_number) WHERE pr_number IS NOT NULL;
CREATE INDEX idx_task_epic_id ON task(epic_id) WHERE epic_id IS NOT NULL;

-- Step 3: Recreate epic with ON DELETE CASCADE on repo_id.
CREATE TABLE epic_new (
    id                TEXT PRIMARY KEY,
    repo_id           TEXT    NOT NULL REFERENCES repo(id) ON DELETE CASCADE,
    title             TEXT    NOT NULL,
    description       TEXT    NOT NULL DEFAULT '',
    status            TEXT    NOT NULL DEFAULT 'draft'
                      CHECK(status IN ('draft', 'planning', 'ready', 'active', 'completed', 'closed')),
    proposed_tasks    TEXT    NOT NULL DEFAULT '[]',
    task_ids          TEXT    NOT NULL DEFAULT '[]',
    planning_prompt   TEXT,
    session_log       TEXT    NOT NULL DEFAULT '[]',
    not_ready         INTEGER NOT NULL DEFAULT 0,
    claimed_at        INTEGER,
    last_heartbeat_at INTEGER,
    feedback          TEXT,
    feedback_type     TEXT    CHECK(feedback_type IN ('message', 'confirmed', 'closed')),
    model             TEXT,
    created_at        INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at        INTEGER NOT NULL DEFAULT (unixepoch())
);
INSERT INTO epic_new (id, repo_id, title, description, status, proposed_tasks, task_ids,
    planning_prompt, session_log, not_ready, claimed_at, last_heartbeat_at, feedback,
    feedback_type, model, created_at, updated_at)
    SELECT id, repo_id, title, description, status, proposed_tasks, task_ids,
    planning_prompt, session_log, not_ready, claimed_at, last_heartbeat_at, feedback,
    feedback_type, model, created_at, updated_at
    FROM epic;
DROP TABLE epic;
ALTER TABLE epic_new RENAME TO epic;
CREATE INDEX idx_epic_repo_id ON epic(repo_id);
CREATE INDEX idx_epic_status ON epic(status);
