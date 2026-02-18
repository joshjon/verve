-- name: CreateTask :exec
INSERT INTO task (id, repo_id, title, description, status, depends_on, attempt, max_attempts, acceptance_criteria_list, max_cost_usd, skip_pr, model, ready, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReadTask :one
SELECT * FROM task WHERE id = ?;

-- name: ListTasks :many
SELECT * FROM task ORDER BY created_at DESC;

-- name: ListTasksByRepo :many
SELECT * FROM task WHERE repo_id = ? ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' AND ready = 1 ORDER BY created_at ASC;

-- name: AppendTaskLogs :exec
INSERT INTO task_log (task_id, attempt, lines) VALUES (?, ?, ?);

-- name: ReadTaskLogs :many
SELECT attempt, lines FROM task_log WHERE task_id = ? ORDER BY id;

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
UPDATE task SET status = 'running', started_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'pending' AND ready = 1;

-- name: HasTasksForRepo :one
SELECT EXISTS(SELECT 1 FROM task WHERE repo_id = ?);

-- name: RetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = ?, agent_status = NULL, started_at = NULL, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'review';

-- name: SetAgentStatus :exec
UPDATE task SET agent_status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: SetRetryContext :exec
UPDATE task SET retry_context = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: AddTaskCost :exec
UPDATE task SET cost_usd = cost_usd + ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: SetConsecutiveFailures :exec
UPDATE task SET consecutive_failures = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: SetCloseReason :exec
UPDATE task SET close_reason = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: SetBranchName :exec
UPDATE task SET branch_name = ?, status = 'review', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: ListTasksInReviewNoPR :many
SELECT * FROM task WHERE status = 'review' AND branch_name IS NOT NULL AND pr_number IS NULL;

-- name: ManualRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  retry_reason = ?, retry_context = NULL, agent_status = NULL,
  close_reason = NULL, consecutive_failures = 0,
  pull_request_url = NULL, pr_number = NULL, branch_name = NULL,
  started_at = NULL, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'failed';

-- name: FeedbackRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  retry_reason = ?, agent_status = NULL,
  consecutive_failures = 0,
  started_at = NULL, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'review';

-- name: DeleteTaskLogs :exec
DELETE FROM task_log WHERE task_id = ?;

-- name: SetDependsOn :exec
UPDATE task SET depends_on = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetReady :exec
UPDATE task SET ready = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
