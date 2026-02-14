-- name: CreateTask :exec
INSERT INTO task (id, repo_id, description, status, depends_on, attempt, max_attempts, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ReadTask :one
SELECT * FROM task WHERE id = $1;

-- name: ListTasks :many
SELECT * FROM task ORDER BY created_at DESC;

-- name: ListTasksByRepo :many
SELECT * FROM task WHERE repo_id = $1 ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' ORDER BY created_at ASC;

-- name: ListPendingTasksByRepos :many
SELECT * FROM task WHERE status = 'pending' AND repo_id = ANY($1::text[]) ORDER BY created_at ASC;

-- name: AppendTaskLogs :exec
INSERT INTO task_log (task_id, lines) VALUES (@id, @lines);

-- name: ReadTaskLogs :many
SELECT lines FROM task_log WHERE task_id = @id ORDER BY id;

-- name: UpdateTaskStatus :exec
UPDATE task SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetTaskPullRequest :exec
UPDATE task SET pull_request_url = $2, pr_number = $3, status = 'review', updated_at = NOW()
WHERE id = $1;

-- name: ListTasksInReview :many
SELECT * FROM task WHERE status = 'review';

-- name: ListTasksInReviewByRepo :many
SELECT * FROM task WHERE repo_id = $1 AND status = 'review';

-- name: CloseTask :exec
UPDATE task SET status = 'closed', close_reason = $2, updated_at = NOW()
WHERE id = $1;

-- name: TaskExists :one
SELECT EXISTS(SELECT 1 FROM task WHERE id = $1);

-- name: ReadTaskStatus :one
SELECT status FROM task WHERE id = $1;

-- name: ClaimTask :execrows
UPDATE task SET status = 'running', updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: HasTasksForRepo :one
SELECT EXISTS(SELECT 1 FROM task WHERE repo_id = $1);

-- name: RetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = $2, updated_at = NOW()
WHERE id = $1 AND status = 'review';
