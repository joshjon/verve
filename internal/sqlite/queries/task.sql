-- name: CreateTask :exec
INSERT INTO task (id, description, status, logs, depends_on, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ReadTask :one
SELECT * FROM task WHERE id = ?;

-- name: ListTasks :many
SELECT * FROM task ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' ORDER BY created_at ASC;

-- name: ReadTaskLogs :one
SELECT logs FROM task WHERE id = ?;

-- name: SetTaskLogs :exec
UPDATE task SET logs = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: UpdateTaskStatus :exec
UPDATE task SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetTaskPullRequest :exec
UPDATE task SET pull_request_url = ?, pr_number = ?, status = 'review', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: ListTasksInReview :many
SELECT * FROM task WHERE status = 'review';

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
