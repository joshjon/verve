-- name: CreateTask :exec
INSERT INTO task (id, repo_id, description, status, depends_on, attempt, max_attempts, acceptance_criteria, max_cost_usd, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReadTask :one
SELECT * FROM task WHERE id = ?;

-- name: ListTasks :many
SELECT * FROM task ORDER BY created_at DESC;

-- name: ListTasksByRepo :many
SELECT * FROM task WHERE repo_id = ? ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' ORDER BY created_at ASC;

-- name: AppendTaskLogs :exec
INSERT INTO task_log (task_id, lines) VALUES (?, ?);

-- name: ReadTaskLogs :many
SELECT lines FROM task_log WHERE task_id = ? ORDER BY id;

-- name: UpdateTaskStatus :exec
UPDATE task SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetTaskPullRequest :exec
UPDATE task SET pull_request_url = ?, pr_number = ?, status = 'review', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: ListTasksInReview :many
SELECT * FROM task WHERE status = 'review';

-- name: ListTasksInReviewByRepo :many
SELECT * FROM task WHERE repo_id = ? AND status = 'review';

-- name: CloseTask :exec
UPDATE task SET status = 'closed', close_reason = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: TaskExists :one
SELECT EXISTS(SELECT 1 FROM task WHERE id = ?);

-- name: ReadTaskStatus :one
SELECT status FROM task WHERE id = ?;

-- name: ClaimTask :execrows
UPDATE task SET status = 'running', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'pending';

-- name: HasTasksForRepo :one
SELECT EXISTS(SELECT 1 FROM task WHERE repo_id = ?);

-- name: RetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = ?, agent_status = NULL, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'review';

-- name: SetAgentStatus :exec
UPDATE task SET agent_status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: SetRetryContext :exec
UPDATE task SET retry_context = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: AddTaskCost :exec
UPDATE task SET cost_usd = cost_usd + ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: SetConsecutiveFailures :exec
UPDATE task SET consecutive_failures = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;
