-- name: UpsertGitHubToken :exec
INSERT INTO github_token (id, encrypted_token, created_at, updated_at)
VALUES ('default', ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET encrypted_token = excluded.encrypted_token, updated_at = excluded.updated_at;

-- name: ReadGitHubToken :one
SELECT encrypted_token FROM github_token WHERE id = 'default';

-- name: DeleteGitHubToken :exec
DELETE FROM github_token WHERE id = 'default';
