-- name: CreateRepo :exec
INSERT INTO repo (id, owner, name, full_name, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ReadRepo :one
SELECT * FROM repo WHERE id = ?;

-- name: ReadRepoByFullName :one
SELECT * FROM repo WHERE full_name = ?;

-- name: ListRepos :many
SELECT * FROM repo ORDER BY created_at DESC;

-- name: DeleteRepo :exec
DELETE FROM repo WHERE id = ?;
