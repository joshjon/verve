-- name: CreateEpic :exec
INSERT INTO epic (id, repo_id, title, description, status, proposed_tasks, task_ids, planning_prompt, session_log, not_ready, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: ReadEpic :one
SELECT * FROM epic WHERE id = $1;

-- name: ListEpics :many
SELECT * FROM epic ORDER BY created_at DESC;

-- name: ListEpicsByRepo :many
SELECT * FROM epic WHERE repo_id = $1 ORDER BY created_at DESC;

-- name: UpdateEpic :exec
UPDATE epic SET
  title = $2,
  description = $3,
  status = $4,
  proposed_tasks = $5,
  task_ids = $6,
  planning_prompt = $7,
  session_log = $8,
  not_ready = $9,
  updated_at = NOW()
WHERE id = $1;

-- name: UpdateEpicStatus :exec
UPDATE epic SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateProposedTasks :exec
UPDATE epic SET proposed_tasks = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetEpicTaskIDs :exec
UPDATE epic SET task_ids = $2, updated_at = NOW()
WHERE id = $1;

-- name: AppendSessionLog :exec
UPDATE epic SET session_log = session_log || $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteEpic :exec
DELETE FROM epic WHERE id = $1;
