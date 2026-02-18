-- name: CreateEpic :exec
INSERT INTO epic (id, repo_id, title, description, status, proposed_tasks, task_ids, planning_prompt, session_log, not_ready, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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
