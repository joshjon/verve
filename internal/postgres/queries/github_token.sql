-- name: UpsertGitHubToken :exec
INSERT INTO github_token (id, encrypted_token, created_at, updated_at)
VALUES ('default', $1, $2, $2)
ON CONFLICT (id) DO UPDATE SET encrypted_token = $1, updated_at = $2;

-- name: ReadGitHubToken :one
SELECT encrypted_token FROM github_token WHERE id = 'default';

-- name: DeleteGitHubToken :exec
DELETE FROM github_token WHERE id = 'default';
