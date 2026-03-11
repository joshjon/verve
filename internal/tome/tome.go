package tome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joshjon/kit/sqlitedb"

	"github.com/joshjon/verve/internal/tome/migrations"
)

// Tome provides session memory backed by a local SQLite database with FTS5 search.
type Tome struct {
	db  *sql.DB
	dir string
}

// Open opens (or creates) a Tome database in the given directory.
func Open(dir string) (*Tome, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sqlitedb.Open(ctx, sqlitedb.WithDir(dir), sqlitedb.WithDBName("data"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := sqlitedb.Migrate(db, migrations.FS); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Tome{db: db, dir: dir}, nil
}

// Close closes the database connection.
func (t *Tome) Close() error {
	return t.db.Close()
}

// Log returns the most recent sessions ordered by creation time (newest first).
func (t *Tome) Log(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := t.db.QueryContext(ctx, `
		SELECT id, summary, learnings, tags, files, branch, status, created_at
		FROM session
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(row scanner) (Session, error) {
	var s Session
	var tagsJSON, filesJSON string
	var createdAt int64

	err := row.Scan(&s.ID, &s.Summary, &s.Learnings, &tagsJSON, &filesJSON, &s.Branch, &s.Status, &createdAt)
	if err != nil {
		return Session{}, err
	}

	_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
	_ = json.Unmarshal([]byte(filesJSON), &s.Files)
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Files == nil {
		s.Files = []string{}
	}
	s.CreatedAt = time.Unix(createdAt, 0)
	return s, nil
}
