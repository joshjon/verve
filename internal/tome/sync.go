package tome

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// SyncOpts configures a sync operation.
type SyncOpts struct {
	PullOnly bool   // only import from remote
	PushOnly bool   // only export to remote
	Branch   string // override branch name (default: tome/context/<author>)
}

// SyncResult reports what happened during sync.
type SyncResult struct {
	Imported int
	Exported int
}

// Sync synchronizes sessions with a git remote via orphan branches.
// Sessions are stored as JSONL on branches like tome/context/<author>.
func (t *Tome) Sync(ctx context.Context, repoDir string, author string, opts SyncOpts) (SyncResult, error) {
	var result SyncResult

	if !opts.PushOnly {
		imported, err := t.pull(ctx, repoDir)
		if err != nil {
			return result, fmt.Errorf("pull: %w", err)
		}
		result.Imported = imported
	}

	if !opts.PullOnly {
		branch := opts.Branch
		if branch == "" {
			branch = "tome/context/" + author
		}

		exported, err := t.push(ctx, repoDir, branch)
		if err != nil {
			return result, fmt.Errorf("push: %w", err)
		}
		result.Exported = exported
	}

	return result, nil
}

// pull fetches all tome/context* branches from the remote and imports sessions.
func (t *Tome) pull(ctx context.Context, repoDir string) (int, error) {
	// Fetch all tome branches from origin.
	_ = gitExec(ctx, repoDir, "fetch", "origin", "refs/heads/tome/context*:refs/heads/tome/context*")

	// List all local tome branches.
	out, err := gitOutput(ctx, repoDir, "for-each-ref", "--format=%(refname:short)", "refs/heads/tome/context")
	if err != nil || strings.TrimSpace(out) == "" {
		return 0, nil // no tome branches
	}

	// Collect existing session IDs to dedup.
	existingIDs, err := t.allSessionIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("load existing IDs: %w", err)
	}

	var imported int
	branches := strings.Split(strings.TrimSpace(out), "\n")
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}

		content, err := gitOutput(ctx, repoDir, "show", branch+":sessions.jsonl")
		if err != nil {
			continue // branch exists but no sessions.jsonl
		}

		sessions, err := decodeJSONL(strings.NewReader(content))
		if err != nil {
			continue // skip malformed branch data
		}

		for _, s := range sessions {
			if existingIDs[s.ID] {
				continue
			}
			if err := t.importSession(ctx, s); err != nil {
				continue
			}
			existingIDs[s.ID] = true
			imported++
		}
	}

	return imported, nil
}

// push exports unexported sessions to the given branch on the remote.
func (t *Tome) push(ctx context.Context, repoDir, branch string) (int, error) {
	sessions, err := t.unexportedSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("load unexported: %w", err)
	}
	if len(sessions) == 0 {
		return 0, nil
	}

	// Read existing content from branch (if any).
	var existingContent string
	if existing, err := gitOutput(ctx, repoDir, "show", branch+":sessions.jsonl"); err == nil {
		existingContent = existing
	}

	// Append new sessions.
	var buf bytes.Buffer
	if existingContent != "" {
		buf.WriteString(existingContent)
		if !strings.HasSuffix(existingContent, "\n") {
			buf.WriteByte('\n')
		}
	}
	if err := encodeJSONL(&buf, sessions); err != nil {
		return 0, fmt.Errorf("encode sessions: %w", err)
	}

	// Create blob.
	blobHash, err := gitInputOutput(ctx, repoDir, buf.Bytes(), "hash-object", "-w", "--stdin")
	if err != nil {
		return 0, fmt.Errorf("hash-object: %w", err)
	}
	blobHash = strings.TrimSpace(blobHash)

	// Create tree with single file: sessions.jsonl.
	treeInput := fmt.Sprintf("100644 blob %s\tsessions.jsonl\n", blobHash)
	treeHash, err := gitInputOutput(ctx, repoDir, []byte(treeInput), "mktree")
	if err != nil {
		return 0, fmt.Errorf("mktree: %w", err)
	}
	treeHash = strings.TrimSpace(treeHash)

	// Create commit (with parent if branch exists).
	commitMsg := fmt.Sprintf("tome: sync %d sessions", len(sessions))
	commitArgs := []string{"commit-tree", treeHash, "-m", commitMsg}

	parentHash, err := gitOutput(ctx, repoDir, "rev-parse", "--verify", "refs/heads/"+branch)
	if err == nil {
		commitArgs = append(commitArgs, "-p", strings.TrimSpace(parentHash))
	}

	commitHash, err := gitInputOutput(ctx, repoDir, nil, commitArgs...)
	if err != nil {
		return 0, fmt.Errorf("commit-tree: %w", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	// Update local ref.
	if err := gitExec(ctx, repoDir, "update-ref", "refs/heads/"+branch, commitHash); err != nil {
		return 0, fmt.Errorf("update-ref: %w", err)
	}

	// Push to remote.
	if err := gitExec(ctx, repoDir, "push", "origin", branch); err != nil {
		return 0, fmt.Errorf("push: %w", err)
	}

	// Mark sessions as exported.
	if err := t.markExported(ctx, sessions); err != nil {
		return 0, fmt.Errorf("mark exported: %w", err)
	}

	return len(sessions), nil
}

// allSessionIDs returns a set of all session IDs in the database.
func (t *Tome) allSessionIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := t.db.QueryContext(ctx, "SELECT id FROM session")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// importSession inserts a session from a remote branch, marking it as already exported.
func (t *Tome) importSession(ctx context.Context, s Session) error {
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Files == nil {
		s.Files = []string{}
	}

	tagsJSON, _ := json.Marshal(s.Tags)
	filesJSON, _ := json.Marshal(s.Files)

	_, err := t.db.ExecContext(ctx, `
		INSERT INTO session (id, summary, learnings, tags, files, branch, status, author, created_at, exported)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, s.ID, s.Summary, s.Learnings, string(tagsJSON), string(filesJSON), s.Branch, s.Status, s.Author, s.CreatedAt.Unix())
	return err
}

// unexportedSessions returns all sessions not yet pushed to a remote.
func (t *Tome) unexportedSessions(ctx context.Context) ([]Session, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT id, summary, learnings, tags, files, branch, status, author, created_at
		FROM session
		WHERE exported = 0
		ORDER BY created_at ASC
	`)
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

// markExported marks the given sessions as exported.
func (t *Tome) markExported(ctx context.Context, sessions []Session) error {
	for _, s := range sessions {
		if _, err := t.db.ExecContext(ctx, "UPDATE session SET exported = 1 WHERE id = ?", s.ID); err != nil {
			return err
		}
	}
	return nil
}

// gitExec runs a git command and returns an error if it fails.
func gitExec(ctx context.Context, repoDir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

// gitOutput runs a git command and returns its stdout.
func gitOutput(ctx context.Context, repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}

// gitInputOutput runs a git command with stdin data and returns stdout.
func gitInputOutput(ctx context.Context, repoDir string, input []byte, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}
