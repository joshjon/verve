package tome

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Search finds sessions matching the query using FTS5 BM25 ranking.
func (t *Tome) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}

	q := `
		SELECT s.id, s.summary, s.learnings, s.tags, s.files, s.branch, s.status, s.created_at,
		       session_fts.rank
		FROM session_fts
		JOIN session s ON session_fts.rowid = s.rowid
		WHERE session_fts MATCH ?`
	args := []any{query}

	if opts.Status != "" {
		q += ` AND s.status = ?`
		args = append(args, opts.Status)
	}
	if opts.FilePattern != "" {
		q += ` AND s.files LIKE ?`
		args = append(args, "%"+opts.FilePattern+"%")
	}

	q += ` ORDER BY session_fts.rank LIMIT ?`
	args = append(args, limit)

	rows, err := t.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var tagsJSON, filesJSON string
		var createdAt int64

		err := rows.Scan(
			&r.Session.ID, &r.Session.Summary, &r.Session.Learnings,
			&tagsJSON, &filesJSON, &r.Session.Branch, &r.Session.Status, &createdAt,
			&r.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		_ = json.Unmarshal([]byte(tagsJSON), &r.Session.Tags)
		_ = json.Unmarshal([]byte(filesJSON), &r.Session.Files)
		if r.Session.Tags == nil {
			r.Session.Tags = []string{}
		}
		if r.Session.Files == nil {
			r.Session.Files = []string{}
		}
		r.Session.CreatedAt = time.Unix(createdAt, 0)
		results = append(results, r)
	}

	return results, rows.Err()
}
