CREATE TABLE github_token (
    id              TEXT PRIMARY KEY DEFAULT 'default' CHECK (id = 'default'),
    encrypted_token TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
