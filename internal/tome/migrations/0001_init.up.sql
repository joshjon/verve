CREATE TABLE IF NOT EXISTS session (
    id               TEXT PRIMARY KEY,
    summary          TEXT NOT NULL,
    learnings        TEXT NOT NULL DEFAULT '',
    content          TEXT NOT NULL DEFAULT '',
    tags             TEXT NOT NULL DEFAULT '[]',
    files            TEXT NOT NULL DEFAULT '[]',
    branch           TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'succeeded',
    transcript_hash  TEXT,
    created_at       INTEGER NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS session_fts USING fts5(
    summary,
    learnings,
    tags,
    content='session',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS session_fts_insert AFTER INSERT ON session BEGIN
    INSERT INTO session_fts(rowid, summary, learnings, tags)
    VALUES (new.rowid, new.summary, new.learnings, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS session_fts_delete AFTER DELETE ON session BEGIN
    INSERT INTO session_fts(session_fts, rowid, summary, learnings, tags)
    VALUES ('delete', old.rowid, old.summary, old.learnings, old.tags);
END;

CREATE TABLE IF NOT EXISTS processed_transcript (
    file_path    TEXT PRIMARY KEY,
    sha256       TEXT NOT NULL,
    session_id   TEXT NOT NULL,
    processed_at INTEGER NOT NULL
);
