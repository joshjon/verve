# Tome — Agent Session Memory

Tome is a standalone CLI for recording and searching agent session history. It gives agents institutional memory — the ability to learn from previous sessions, avoid repeating mistakes, and build on past work.

## Why

Verve's agent containers are ephemeral. Each task spawns a fresh Docker container with no knowledge of what previous agents discovered. Without session memory, agents rediscover the same gotchas, repeat failed approaches, and miss patterns that earlier agents already found.

Tome solves this by giving agents an on-demand search tool — like `grep` but for institutional knowledge rather than code. Sessions are captured automatically from Claude Code transcripts, requiring zero agent effort and zero additional API tokens.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│ Agent Container                                     │
│                                                     │
│  Agent works on task...                             │
│    ├─ tome search "auth middleware"    ← on-demand  │
│    └─ tome search --file "src/api/"   ← filtered   │
│                                                     │
│  post-commit hook → tome checkpoint   ← automatic  │
│  pre-push hook → tome sync --push     ← automatic  │
│                                                     │
│  /cache/tome/data.db  ← mounted from host volume   │
└─────────────────────────────────────────────────────┘
        │ (Docker cache volume)
┌───────┴──────────────────┐
│ Host: ~/.cache/verve/tome│ ← persists across containers
└──────────────────────────┘

Standalone user:
  $ cd my-repo
  $ tome init                         ← creates DB + installs hooks
  $ tome search "authentication"      ← reads .tome/data.db
  $ tome checkpoint                   ← imports transcripts
```

Tome has **no dependency on Verve**. It's a general-purpose tool that works for both Verve agents and standalone Claude Code users.

### Data directory resolution

1. `TOME_DIR` env var — used in Docker containers (`/cache/tome`)
2. `.tome/` in git repo root — for standalone users

### Database

SQLite via `modernc.org/sqlite` (pure Go, no CGO). The database file is `data.db` inside the data directory. Schema auto-migrates on first use.

## Transcript Auto-Capture

Sessions are captured automatically from Claude Code conversation transcripts — no agent action required.

### How it works

Claude Code writes complete conversation transcripts as `.jsonl` files to `~/.claude/projects/<sanitized-repo-path>/`. Each file is one conversation.

`tome checkpoint` discovers these transcripts, deduplicates by SHA256 file hash, and parses new or changed files into sessions:

1. **Discovery**: Scans the transcript directory for `.jsonl` files
2. **Dedup**: SHA256 hash compared against `processed_transcript` table — same file + same hash → skip
3. **Parse**: Extracts session data from the JSONL conversation:
   - **Summary**: First substantive user message, truncated to 200 chars
   - **Content**: All assistant text blocks concatenated (searchable by LSA)
   - **Files**: Deduplicated file paths from Read/Write/Edit tool_use blocks, stripped to relative paths
   - **Branch**: From transcript metadata (`gitBranch` field)
4. **Re-processing**: Same file + different hash → old session deleted, new session created

### What's excluded from transcripts

- `tool_result` blocks (raw file contents/command output — too large, mostly noise)
- `thinking` blocks
- `file-history-snapshot` entries
- `isSidechain: true` entries (parallel exploration branches)

### Git hooks

Installed automatically by `tome init`:

- **`post-commit`** → `tome checkpoint` (runs in background, stderr suppressed)
- **`pre-push`** → `tome sync --push` (runs in foreground, errors non-fatal)

Hook installation is idempotent (marker comment `# managed by tome` prevents duplication) and preserves existing hook content by appending.

## Search

Tome uses a hybrid search system that combines keyword matching with semantic similarity.

### BM25 (keyword search)

FTS5 full-text search across session summaries, learnings, and tags. Fast, precise, and works from the first session. This is the baseline — it always works.

### LSA (semantic search)

Latent Semantic Analysis captures term co-occurrence patterns to find semantically related sessions. For example, a search for "authentication" will also surface sessions about "login flow" or "OAuth tokens" even if they don't contain the exact word "authentication".

For transcript sessions, LSA indexes the full `content` field (all assistant text), giving rich semantic matching against the complete reasoning and explanations from each conversation.

**How it works:**
1. Tokenize all sessions (lowercase, stop word removal)
2. Build a TF-IDF weighted document-term matrix
3. SVD decomposition reduces it to ~128 dimensions (concept space)
4. Query is projected into the same space
5. Sessions are ranked by cosine similarity to the query

**Requirements:** Needs at least 2 sessions to build an index. Terms must appear in at least 2 sessions to be included in the vocabulary.

### Hybrid scoring

When LSA is available, both scores are combined:

```
final_score = 0.4 × normalize(bm25) + 0.6 × normalize(lsa)
```

LSA gets higher weight because it captures relationships that keyword search misses. If LSA is unavailable (< 2 sessions, build failure), search gracefully degrades to BM25-only.

## Git Sync

Sessions can be synchronized across machines via git orphan branches. Each user or worker pushes sessions to their own branch, and pulls from all branches on the remote.

### Branch layout

```
Git Remote
├── main                          # Normal code
├── tome/context/alice             # Alice's sessions
├── tome/context/bob               # Bob's sessions
└── tome/context/verve-agent       # Verve agent sessions
```

Branch names are derived from `git config user.name`, sanitized to lowercase with special characters replaced by hyphens.

### Wire format

Sessions are stored as JSONL (one JSON object per line) in a file called `sessions.jsonl` on each branch:

```jsonl
{"id":"abc-123","summary":"Added JWT refresh tokens","learnings":"Redis required for blacklist","content":"...","tags":["auth","jwt"],"files":["src/auth.go"],"status":"succeeded","user":"alice","created_at":"2026-03-10T14:30:00Z"}
```

### How sync works

**Push:** Queries the database for unexported sessions, appends them to `sessions.jsonl`, commits using git plumbing commands (`hash-object`, `mktree`, `commit-tree`) without polluting the working tree, and pushes to the remote. Sessions are then marked as exported.

**Pull:** Fetches all `tome/context*` branches from the remote, reads each branch's `sessions.jsonl`, and imports sessions into the local database. Session IDs are used for deduplication — re-pulling is safe and idempotent.

**Conflict avoidance:** Each user pushes only to their own branch, so there are no write conflicts. All branches are merged into the local database on pull.

### Concurrent sync safety

When multiple agents run concurrently (e.g., Verve agent containers sharing the same cache volume):

1. **File lock**: `sync.lock` in the tome data directory serializes all sync operations across containers on the same host
2. **Fetch-from-remote**: Push reads existing content from the remote ref (after fetching), not the local ref, preventing diverged local state on rejected pushes
3. **SQLite busy timeout**: The kit `sqlitedb` library sets `_busy_timeout=5000` automatically, so concurrent DB access doesn't fail with SQLITE_BUSY

## Verve integration

### Docker image

The `tome` binary is cross-compiled for Linux and included in the agent Docker image at `/usr/local/bin/tome`. The `TOME_DIR` env var is set to `/cache/tome`, which maps to the host's cache volume so sessions persist across containers.

### Worker

When cache is enabled, the worker sets `TOME_DIR=/cache/tome` in the container environment. The cache volume (`~/.cache/verve` → `/cache`) is already mounted, so `/cache/tome/` is automatically available.

### Agent prompt

When `tome` is available in the container, the agent receives search-only instructions:

```
SESSION MEMORY: You have access to `tome` for searching past session history.
- Before starting, search for relevant past sessions: `tome search "relevant topic"`
- Filter by files touched: `tome search --file "src/auth/" "query"`
- View recent sessions: `tome log`
Sessions are captured automatically from transcripts — no need to record manually.
```

## Package structure

```
cmd/tome/
    main.go                     # CLI entry point (urfave/cli/v2)

internal/tome/
    tome.go                     # Tome struct: Open, Close, Log, LSA management
    session.go                  # Session, SearchOpts, SearchResult types
    record.go                   # Record() — insert session
    search.go                   # Search() — hybrid BM25+LSA
    lsa.go                      # TF-IDF matrix, SVD, cosine similarity
    tokenizer.go                # Text tokenization and stop words
    transcript.go               # Parse Claude Code .jsonl transcripts
    checkpoint.go               # Discover + process transcripts
    hooks.go                    # Git hook installation
    sync.go                     # Git orphan branch sync (pull/push)
    jsonl.go                    # JSONL encode/decode for wire format
    format.go                   # Text and JSON output formatting
    tome_test.go                # Core integration tests
    lsa_test.go                 # LSA and hybrid search tests
    sync_test.go                # Git sync tests
    transcript_test.go          # Transcript parser tests
    checkpoint_test.go          # Checkpoint discovery/dedup tests
    hooks_test.go               # Git hook installation tests
    migrations/
        fs.go                   # //go:embed *.sql
        0001_init.up.sql        # sessions table + FTS5 + processed_transcript
        0002_sync_metadata.up.sql  # exported flag + user column
```

## CLI reference

```
tome search <query>              # Hybrid BM25+LSA search
tome search --bm25-only <query>  # Keyword-only search
tome search --file "path" <q>    # Filter by files touched
tome search --status failed <q>  # Filter by outcome
tome search --limit 3 <query>    # Top N results (default: 5)
tome search --json <query>       # JSON output

tome record \
  --summary "What you did" \
  --learnings "Key findings" \
  --tags "auth,jwt" \
  --files "src/auth.go" \
  --status succeeded \
  --user "alice"                 # Auto-detected from git config

tome checkpoint                  # Import new transcripts
tome log                         # Recent sessions (default: 10)
tome log --limit 5 --json        # Last 5, JSON format

tome index                       # Rebuild LSA index (diagnostics)
tome init                        # Initialize database + install hooks
tome init --no-hooks             # Initialize without hooks

tome sync                        # Pull + push (default)
tome sync --pull                 # Import from remote only
tome sync --push                 # Export to remote only
tome sync --branch "custom"      # Override branch name
```

---

## Testing guide

This section walks through testing the full tome feature set end-to-end. All commands run from the repo root.

### Prerequisites

```bash
# Build the tome binary
make build-tome

# Verify it runs
./bin/tome --help
```

### 1. Initialize and install hooks

```bash
# Initialize (creates DB + installs git hooks)
./bin/tome init

# Verify hooks were installed
cat .git/hooks/post-commit
cat .git/hooks/pre-push

# Initialize without hooks
./bin/tome init --no-hooks
```

### 2. Record sessions manually

```bash
# Record a few sessions with different topics
./bin/tome record \
  --summary "Added JWT authentication middleware" \
  --learnings "Token validation uses Bearer scheme. Refresh tokens stored in httponly cookies. Redis required for token blacklist." \
  --tags "auth,jwt,middleware" \
  --files "src/auth/middleware.go,src/auth/tokens.go" \
  --status succeeded

./bin/tome record \
  --summary "Implemented user login flow" \
  --learnings "Password hashing with bcrypt. Session cookies for login state. Auth redirect on expired session." \
  --tags "auth,login,user" \
  --files "src/auth/login.go,src/user/handler.go" \
  --status succeeded

# Verify they're stored
./bin/tome log
```

### 3. Checkpoint (transcript auto-capture)

```bash
# Import Claude Code transcripts
./bin/tome checkpoint

# Expected: Imports new transcripts or reports none found

# Run again — should skip already-processed files
./bin/tome checkpoint

# Expected: "Skipped N (already processed)."
```

### 4. Search

```bash
# Basic keyword search
./bin/tome search "authentication"

# Hybrid search (with LSA after ≥2 sessions)
./bin/tome search "login"

# BM25-only
./bin/tome search --bm25-only "middleware"

# Filter by status/file
./bin/tome search --status failed "email"
./bin/tome search --file "src/api/" "middleware"
```

### 5. Git sync

```bash
# Set up a test remote
TMPDIR=$(mktemp -d)
git init --bare --initial-branch=main "$TMPDIR/remote.git"
git clone "$TMPDIR/remote.git" "$TMPDIR/clone1"
git -C "$TMPDIR/clone1" config user.name "Alice"
echo "# test" > "$TMPDIR/clone1/README.md"
git -C "$TMPDIR/clone1" add README.md
git -C "$TMPDIR/clone1" commit -m "init"
git -C "$TMPDIR/clone1" push -u origin main

# Record and push
cd "$TMPDIR/clone1"
TOME_DIR="$TMPDIR/tome1" tome record \
  --summary "Alice added auth middleware" \
  --learnings "Bearer token validation in middleware" \
  --tags "auth" \
  --user "Alice"

TOME_DIR="$TMPDIR/tome1" tome sync --push --user "Alice"

# Verify branch name uses sanitized username
git branch -a | grep tome
# Expected: tome/context/alice

# Clean up
rm -rf "$TMPDIR"
```

### 6. Run the automated test suite

```bash
# All tome tests (core + LSA + sync + transcript + checkpoint + hooks)
go test ./internal/tome/... -v -count=1
```

### 7. Docker integration (requires Docker)

```bash
# Build agent image with tome included
make build-agent

# Verify tome is in the image
docker run --rm verve:base tome --help

# Verify TOME_DIR is set
docker run --rm verve:base env | grep TOME_DIR
# Expected: TOME_DIR=/cache/tome
```
