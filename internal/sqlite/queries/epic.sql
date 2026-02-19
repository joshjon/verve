-- name: CreateEpic :exec
INSERT INTO epic (id, repo_id, title, description, status, proposed_tasks, task_ids, planning_prompt, session_log, not_ready, model, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReadEpic :one
SELECT * FROM epic WHERE id = ?;

-- name: ListEpics :many
SELECT * FROM epic ORDER BY created_at DESC;

-- name: ListEpicsByRepo :many
SELECT * FROM epic WHERE repo_id = ? ORDER BY created_at DESC;

-- name: UpdateEpic :exec
UPDATE epic SET
  title = ?,
  description = ?,
  status = ?,
  proposed_tasks = ?,
  task_ids = ?,
  planning_prompt = ?,
  session_log = ?,
  not_ready = ?,
  model = ?,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: UpdateEpicStatus :exec
UPDATE epic SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: UpdateProposedTasks :exec
UPDATE epic SET proposed_tasks = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetEpicTaskIDs :exec
UPDATE epic SET task_ids = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: AppendSessionLog :exec
UPDATE epic SET session_log = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: DeleteEpic :exec
DELETE FROM epic WHERE id = ?;

-- name: ListPlanningEpics :many
SELECT * FROM epic
WHERE status = 'planning' AND claimed_at IS NULL
ORDER BY created_at ASC;

-- name: ClaimEpic :exec
UPDATE epic SET
  claimed_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
  last_heartbeat_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'planning' AND claimed_at IS NULL;

-- name: EpicHeartbeat :exec
UPDATE epic SET
  last_heartbeat_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetEpicFeedback :exec
UPDATE epic SET
  feedback = ?,
  feedback_type = ?,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: ClearEpicFeedback :exec
UPDATE epic SET
  feedback = NULL,
  feedback_type = NULL,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: ReleaseEpicClaim :exec
UPDATE epic SET
  claimed_at = NULL,
  last_heartbeat_at = NULL,
  status = 'planning',
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: ListStaleEpics :many
SELECT * FROM epic
WHERE claimed_at IS NOT NULL
  AND last_heartbeat_at < ?
  AND status IN ('planning', 'draft')
ORDER BY last_heartbeat_at ASC;
