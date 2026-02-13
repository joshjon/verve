-- name: CreateRepo :exec
INSERT INTO repo (id, owner, name, full_name, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ReadRepo :one
SELECT * FROM repo WHERE id = $1;

-- name: ReadRepoByFullName :one
SELECT * FROM repo WHERE full_name = $1;

-- name: ListRepos :many
SELECT * FROM repo ORDER BY created_at DESC;

-- name: DeleteRepo :exec
DELETE FROM repo WHERE id = $1;
