CREATE TABLE github_token
(
    id              TEXT PRIMARY KEY DEFAULT 'default' CHECK (id = 'default'),
    encrypted_token TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
