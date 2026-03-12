# Tome — Agent Session Memory CLI

## Context

Verve's agent containers are ephemeral — each task spawns a fresh Docker container with no knowledge of prior sessions. Agents miss discovered patterns, gotchas, and decisions made by previous agents.

Instead of pre-loading guessed context at startup, agents should query for relevant sessions **on-demand** as they work — the same way they'd use grep or find, but for institutional knowledge. This requires a standalone CLI tool that works for both Verve agents and standalone Claude Code users.

**Tome** is that tool: a lightweight CLI for recording and searching agent session history, backed by a local SQLite database with FTS5 search. It lives in the Verve repo for now (`cmd/tome/`) but has no dependency on Verve — it's a general-purpose tool.

## Architecture

```
┌─────────────────────────────────────────────────┐
│ Agent Container                                 │
│                                                 │
│  Agent works on task...                         │
│    ├─ tome search "auth middleware"  ← on-demand│
│    ├─ tome search --file "src/api/" "error"     │
│    └─ tome record --summary "..." --learnings   │
│                                                 │
│  /cache/tome/data.db  ← mounted from host       │
└─────────────────────────────────────────────────┘
        │ (Docker cache volume)
┌───────┴──────────────────┐
│ Host: ~/.cache/verve/tome│
│  └─ <repo-hash>/data.db │ ← persists across containers
└──────────────────────────┘

Standalone Claude Code user:
  $ cd my-repo
  $ tome search "authentication"   ← reads .tome/data.db
  $ tome record --summary "..."    ← writes .tome/data.db
```

**Key principle**: The agent decides when to search, not the worker. `tome search` is another tool in the agent's toolkit alongside grep, find, and read — but for session history rather than code.

---

## Phase 1: Core CLI (BM25 Search)

### CLI Interface

```
tome search "query"                      # BM25 search across sessions
tome search --file "src/auth/" "query"   # Filter by files touched
tome search --status succeeded "query"   # Filter by outcome
tome search --limit 3 "query"            # Top N results (default: 5)

tome record \                            # Record a session
  --summary "Added JWT refresh tokens" \
  --learnings "Redis required for blacklist, Bearer auth expected" \
  --tags "auth,jwt,redis" \
  --files "src/auth.go,src/auth_test.go" \
  --status succeeded                     # or failed

tome log                                 # Recent sessions (default: 10)
tome log --limit 5                       # Last N sessions

tome init                                # Explicit init (optional — auto-inits on first record)
```

**Data directory resolution** (in priority order):
1. `TOME_DIR` env var (used in Docker containers)
2. `.tome/` in git repo root (for standalone users)

**Output format**: Human/agent-readable text by default, `--json` for machine consumption.

Search output example:
```
━━ Added JWT refresh token rotation (succeeded, 2 days ago) ━━
Files: src/auth/middleware.go, src/auth/tokens.go
Tags:  auth, jwt, middleware
Learnings:
  Redis required for token blacklist (test suite uses testcontainers).
  Auth middleware expects Bearer scheme in Authorization header.
  Refresh tokens stored in HttpOnly cookies, not localStorage.

━━ Fixed rate limiter for API endpoints (succeeded, 5 days ago) ━━
Files: src/api/middleware.go, src/api/ratelimit.go
Tags:  api, rate-limiting
Learnings:
  Rate limiter uses sliding window algorithm, configured per-route.
  Tests use a mock clock — never use time.Sleep in rate limit tests.
```

### Go Package Structure

```
cmd/tome/
    main.go                    # CLI entry point (urfave/cli/v2)

internal/tome/
    tome.go                    # Tome struct: Open, Close, auto-init
    session.go                 # Session type definition
    record.go                  # Record() — insert session + FTS
    search.go                  # Search() — FTS5 BM25 query
    format.go                  # FormatSearchResults(), FormatLog()
    migrations/
        fs.go                  # //go:embed *.sql
        0001_init.up.sql       # sessions table + FTS5
```

### Core API (`internal/tome/`)

```go
type Tome struct {
    db  *sql.DB
    dir string
}

func Open(dir string) (*Tome, error)       // Open existing, auto-init if missing
func (t *Tome) Close() error

func (t *Tome) Record(ctx context.Context, s Session) error
func (t *Tome) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error)
func (t *Tome) Log(ctx context.Context, limit int) ([]Session, error)
```

```go
type Session struct {
    ID        string    // ULID, auto-generated on record
    Summary   string
    Learnings string
    Tags      []string
    Files     []string
    Branch    string
    Status    string    // "succeeded" or "failed"
    CreatedAt time.Time
}

type SearchOpts struct {
    FilePattern string // regex filter on files touched
    Status      string // filter by status
    Limit       int    // max results (default 5)
}

type SearchResult struct {
    Session Session
    Score   float64 // BM25 rank score
    Snippet string  // matching excerpt from learnings
}
```

### Migration (`0001_init.up.sql`)

```sql
CREATE TABLE IF NOT EXISTS session (
    id         TEXT PRIMARY KEY,
    summary    TEXT NOT NULL,
    learnings  TEXT NOT NULL DEFAULT '',
    tags       TEXT NOT NULL DEFAULT '[]',
    files      TEXT NOT NULL DEFAULT '[]',
    branch     TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'succeeded',
    created_at INTEGER NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS session_fts USING fts5(
    summary,
    learnings,
    tags,
    content='session',
    content_rowid='rowid'
);

CREATE TRIGGER session_fts_insert AFTER INSERT ON session BEGIN
    INSERT INTO session_fts(rowid, summary, learnings, tags)
    VALUES (new.rowid, new.summary, new.learnings, new.tags);
END;

CREATE TRIGGER session_fts_delete AFTER DELETE ON session BEGIN
    INSERT INTO session_fts(session_fts, rowid, summary, learnings, tags)
    VALUES ('delete', old.rowid, old.summary, old.learnings, old.tags);
END;
```

> **Note**: FTS5 support in libSQL/modernc needs early verification. If virtual tables don't work with the driver, fall back to `LIKE`-based search with manual scoring.

### Files to Create

| File | Purpose |
|------|---------|
| `cmd/tome/main.go` | CLI entry point with search, record, log, init commands |
| `internal/tome/tome.go` | Core Tome struct, Open/Close, auto-init, DB connection |
| `internal/tome/session.go` | Session and SearchResult types |
| `internal/tome/record.go` | Record() implementation |
| `internal/tome/search.go` | Search() with FTS5 BM25 |
| `internal/tome/format.go` | Text and JSON output formatting |
| `internal/tome/migrations/fs.go` | `//go:embed *.sql` |
| `internal/tome/migrations/0001_init.up.sql` | Schema + FTS5 |
| `internal/tome/tome_test.go` | Core integration tests |

### Files to Modify

**`agent/Dockerfile`**
- Add build stage for `tome` binary
- Copy `tome` to `/usr/local/bin/tome` in final image

**`Makefile`**
- Add `build-tome` target: `go build -o bin/tome ./cmd/tome`
- Update `build-agent` to include tome binary in image

**`internal/worker/docker.go`**
- Add `TOME_DIR=/cache/tome` to container env vars
- The cache volume mount already exists (`~/.cache/verve` → `/cache`), so `/cache/tome/` is automatically available

**`agent/lib/prompt.sh`**
- Add tome usage instructions to the prompt (after repo context, before acceptance criteria)

### Implementation Order

1. Verify FTS5 works with modernc.org/sqlite (quick spike)
2. `internal/tome/` — session types, migrations, Tome struct with Open/Close
3. `internal/tome/` — Record() and Search() implementations
4. `internal/tome/` — format.go for text/JSON output
5. `cmd/tome/main.go` — CLI with search, record, log, init commands
6. Tests — integration tests for record/search/log cycle
7. `agent/Dockerfile` — include tome binary
8. `internal/worker/docker.go` — add TOME_DIR env var
9. `agent/lib/prompt.sh` — add tome instructions
10. `Makefile` — build-tome target
11. Update `docs/FEATURES.md`

### Verification

1. **Build**: `go build -o bin/tome ./cmd/tome` — compiles
2. **Init + record**: `tome init && tome record --summary "test" --learnings "test learning" --tags "test" --files "main.go"` — creates DB, stores session
3. **Search**: `tome search "test"` — returns the recorded session with BM25 ranking
4. **Log**: `tome log` — shows the session
5. **Docker**: `make build-agent` — image includes tome binary
6. **Container test**: Run agent container with cache volume, verify `tome search` returns results from previous runs
7. **Tests**: `go test ./internal/tome/...` — all pass
8. **Existing tests**: `go test ./...` — no regressions

---

## Phase 2: Hybrid Search (LSA)

### Overview

FTS5 BM25 is good for keyword matching but misses semantic similarity (e.g., "authentication" won't match a session about "login flow"). LSA (Latent Semantic Analysis) captures term co-occurrence patterns to find semantically related sessions.

### How LSA Works

1. **Build TF-IDF matrix**: Each session is a document. Extract terms, compute term frequency x inverse document frequency.
2. **SVD decomposition**: Reduce the TF-IDF matrix to ~128 dimensions using Singular Value Decomposition.
3. **Project query**: Transform the search query into the same reduced space.
4. **Cosine similarity**: Rank sessions by cosine distance to the query vector in the reduced space.

### Hybrid Scoring

Combine BM25 and LSA scores with normalization:

```
final_score = 0.4 x normalize(bm25_score) + 0.6 x normalize(lsa_score)
```

Both scores are normalized to [0, 1] before combining. LSA gets higher weight because it captures semantic similarity that BM25 misses.

### Implementation

**New dependency**: `gonum.org/v1/gonum` — pure Go numerical library with `mat.SVD`.

**New files**:
```
internal/tome/
    lsa.go           # TF-IDF matrix, SVD, cosine similarity
    lsa_test.go      # LSA-specific tests
    tokenizer.go     # Text tokenization (lowercase, stop words, stemming)
```

**Changes to existing files**:
- `search.go` — Add hybrid scoring path. If LSA index exists and has >=2 sessions, use hybrid. Otherwise fall back to BM25-only.
- `tome.go` — Add LSA index rebuild on `Record()` (or lazy rebuild on `Search()` if stale).

**LSA index storage**: Computed in-memory from session content. Cached as a serialized matrix in `.tome/lsa_cache.bin`. Invalidated when session count changes.

**Key types**:
```go
type LSAIndex struct {
    SessionIDs []string        // Ordered session IDs
    U          *mat.Dense      // Left singular vectors (documents in reduced space)
    S          []float64       // Singular values
    V          *mat.Dense      // Right singular vectors (terms in reduced space)
    Vocab      map[string]int  // Term -> column index
    Dim        int             // Reduced dimensions (default: 128)
}

func BuildLSA(sessions []Session, dim int) (*LSAIndex, error)
func (idx *LSAIndex) Query(text string) []ScoredSession
```

**Tokenization pipeline**:
1. Lowercase
2. Split on non-alphanumeric characters
3. Remove stop words (English common words)
4. Optional: Porter stemming (via a small Go library or hand-rolled)
5. Minimum document frequency: term must appear in >=2 sessions

**Graceful degradation**:
- <2 sessions -> BM25 only (LSA needs at least 2 documents)
- LSA build fails -> log warning, fall back to BM25 only
- gonum SVD fails (e.g., degenerate matrix) -> fall back to BM25 only

### Migration

**`0002_lsa_metadata.up.sql`** (optional):
```sql
-- Track LSA index freshness
CREATE TABLE IF NOT EXISTS index_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

Store `session_count` and `last_indexed_at` to know when to rebuild.

### CLI Changes

- `tome search` automatically uses hybrid scoring when LSA index is available
- `tome index` (new command) — force rebuild LSA index
- `tome search --bm25-only "query"` — force BM25-only mode (for debugging/comparison)

### Verification

1. Create 10+ sessions with varied content
2. Search with a query that uses different vocabulary than stored sessions
3. Verify LSA finds semantically related sessions that BM25 misses
4. Benchmark: search should complete in <200ms for 1000 sessions

---

## Phase 3: Orphan Branch Sync

### Overview

Store session data on a git orphan branch so sessions travel with the repo. This enables:
- Verve agents sharing context across workers
- Standalone Claude Code users sharing context with Verve agents
- Cross-machine sync without external infrastructure

### Orphan Branch Design

```
Git Remote
├── main                            # Normal code
├── tome/context                    # Shared sessions (Verve worker writes here)
├── tome/context/alice@example.com  # Alice's standalone sessions
└── tome/context/bob@example.com    # Bob's standalone sessions
```

Each branch contains a single file: `sessions.jsonl` — one JSON object per line, append-only.

### JSONL Wire Format

```jsonl
{"id":"01JNQX...","summary":"Added JWT refresh tokens","learnings":"Redis required...","tags":["auth","jwt"],"files":["src/auth.go"],"branch":"feat/auth","status":"succeeded","created_at":"2026-03-10T14:30:00Z","author":"alice@example.com"}
{"id":"01JNQY...","summary":"Fixed rate limiter","learnings":"Sliding window...","tags":["api"],"files":["src/ratelimit.go"],"branch":"fix/rate-limit","status":"succeeded","created_at":"2026-03-11T09:15:00Z","author":"verve-agent"}
```

**Why JSONL over binary**:
- Human-readable and debuggable (`git show tome/context:sessions.jsonl`)
- Git's pack files handle compression (typically 2-5x)
- Append-friendly (new sessions = new lines)
- Any tool can read/write (no custom codec needed)
- Good enough for the data volume (hundreds to low thousands of sessions per repo)

### Sync Mechanism

**`tome sync`** (pull + push):
1. `git fetch origin 'refs/heads/tome/context*'` — fetch all tome branches
2. For each remote branch, read `sessions.jsonl` and import into local `data.db` (dedup by session ID)
3. Export local sessions not yet on the remote branch -> append to `sessions.jsonl`
4. Commit and push to the user's branch

**`tome sync --pull`** (pull only, for Verve workers):
- Fetch + import only. No push. Used before spawning containers.

**`tome sync --push`** (push only):
- Export + commit + push. Used after agent completes.

### Conflict Handling

- Each user/worker pushes only to their own branch -> no write conflicts
- Import merges all branches into local DB -> no read conflicts
- Session ID deduplication prevents duplicates on re-import
- If push fails (non-fast-forward), fetch + rebase + retry

### Verve Worker Integration

**Before spawning container**:
```go
// In worker.go, before RunAgent():
exec.Command("tome", "sync", "--pull").Run()
```

**After container completes**:
```go
// In worker.go, after completeTask():
exec.Command("tome", "sync", "--push").Run()
```

Or: worker calls `tome` Go API directly (same process, no subprocess).

### Git Credential Handling

- Standalone users: uses existing git credentials (SSH key, credential helper)
- Verve containers: `GITHUB_TOKEN` is already available, `tome sync` uses it for HTTPS push/pull
- `tome sync` respects `GIT_ASKPASS`, `GIT_CREDENTIAL_HELPER`, etc.

### New Files

```
internal/tome/
    sync.go          # Sync implementation (fetch, import, export, push)
    sync_test.go     # Sync tests (using git init --bare for test remote)
    jsonl.go         # JSONL encoding/decoding
```

### CLI Changes

```
tome sync                    # Pull + push (interactive users)
tome sync --pull             # Pull only (Verve workers)
tome sync --push             # Push only
tome sync --branch "custom"  # Override branch name (default: tome/context/<email>)
```

### Migration

**`0003_sync_metadata.up.sql`**:
```sql
-- Track which sessions have been exported
ALTER TABLE session ADD COLUMN exported INTEGER NOT NULL DEFAULT 0;
-- Track import source
ALTER TABLE session ADD COLUMN author TEXT NOT NULL DEFAULT '';
```

### Verification

1. Init two separate repos pointing at same remote
2. Record sessions in repo A, sync push
3. In repo B, sync pull -> sessions appear in search
4. Record in repo B, sync push -> no conflicts
5. In repo A, sync pull -> sees sessions from both A and B
6. Verify Verve worker sync lifecycle works end-to-end

---

## Phase 4: Verve UI Integration

### Overview

Surface session history in Verve's web UI so users can see what agents learned.

### Approach

- Verve server reads from the tome SQLite DB (same cache directory the worker uses)
- New API endpoints expose session data to the UI
- No duplication — single source of truth in tome's DB

### API Endpoints

```
GET /api/v1/repos/:repo_id/sessions                    # List sessions
GET /api/v1/repos/:repo_id/sessions/search?q=...       # Search sessions
```

### UI Components

- Session history panel on repo detail page
- Search box for filtering sessions
- Session detail view showing learnings, files, tags

---

## Key Design Decisions (All Phases)

- **Standalone CLI**: No Verve dependency. Anyone can `go install` and use tome.
- **Agent-driven querying**: Agent decides when to search, not the worker.
- **Own SQLite DB**: `.tome/data.db` is separate from Verve's database.
- **Auto-init**: First `tome record` creates the DB. No ceremony required.
- **Cache volume reuse**: Verve already mounts `~/.cache/verve` -> `/cache`.
- **JSONL over binary**: Human-readable, debuggable, git-compressible. Good enough for our data volume.
- **modernc.org/sqlite**: Pure Go, no CGO, cross-compilable.
- **Progressive search quality**: BM25 (Phase 1) -> BM25+LSA (Phase 2) -> potential embeddings (future).
