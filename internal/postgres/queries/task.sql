-- name: CreateTask :exec
INSERT INTO task (id, repo_id, title, description, status, depends_on, attempt, max_attempts, acceptance_criteria_list, max_cost_usd, skip_pr, model, ready, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- name: ReadTask :one
SELECT * FROM task WHERE id = $1;

-- name: ListTasks :many
SELECT * FROM task ORDER BY created_at DESC;

-- name: ListTasksByRepo :many
SELECT * FROM task WHERE repo_id = $1 ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' AND ready = true ORDER BY created_at ASC;

-- name: ListPendingTasksByRepos :many
SELECT * FROM task WHERE status = 'pending' AND ready = true AND repo_id = ANY($1::text[]) ORDER BY created_at ASC;

-- name: AppendTaskLogs :exec
INSERT INTO task_log (task_id, attempt, lines) VALUES (@id, @attempt, @lines);

-- name: ReadTaskLogs :many
SELECT attempt, lines FROM task_log WHERE task_id = @id ORDER BY id;

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
UPDATE task SET status = 'running', started_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'pending' AND ready = true;

-- name: HasTasksForRepo :one
SELECT EXISTS(SELECT 1 FROM task WHERE repo_id = $1);

-- name: RetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = $2, started_at = NULL, updated_at = NOW()
WHERE id = $1 AND status = 'review';

-- name: SetAgentStatus :exec
UPDATE task SET agent_status = $2, updated_at = NOW() WHERE id = $1;

-- name: SetRetryContext :exec
UPDATE task SET retry_context = $2, updated_at = NOW() WHERE id = $1;

-- name: AddTaskCost :exec
UPDATE task SET cost_usd = cost_usd + $2, updated_at = NOW() WHERE id = $1;

-- name: SetConsecutiveFailures :exec
UPDATE task SET consecutive_failures = $2, updated_at = NOW() WHERE id = $1;

-- name: SetCloseReason :exec
UPDATE task SET close_reason = $2, updated_at = NOW() WHERE id = $1;

-- name: SetBranchName :exec
UPDATE task SET branch_name = $2, status = 'review', updated_at = NOW() WHERE id = $1;

-- name: ListTasksInReviewNoPR :many
SELECT * FROM task WHERE status = 'review' AND branch_name IS NOT NULL AND pr_number IS NULL;

-- name: ManualRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  retry_reason = $2, retry_context = NULL,
  close_reason = NULL, consecutive_failures = 0,
  pull_request_url = NULL, pr_number = NULL, branch_name = NULL,
  started_at = NULL, updated_at = NOW()
WHERE id = $1 AND status = 'failed';

-- name: FeedbackRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  max_attempts = max_attempts + 1,
  retry_reason = $2, retry_context = NULL,
  consecutive_failures = 0,
  started_at = NULL, updated_at = NOW()
WHERE id = $1 AND status = 'review';

-- name: DeleteTaskLogs :exec
DELETE FROM task_log WHERE task_id = $1;

-- name: RemoveDependency :exec
UPDATE task SET depends_on = array_remove(depends_on, $2), updated_at = NOW()
WHERE id = $1;

-- name: SetReady :exec
UPDATE task SET ready = $2, updated_at = NOW() WHERE id = $1;
