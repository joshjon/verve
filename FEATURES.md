# Features

## Task Management

- **Six-state lifecycle**: `pending` → `running` → `review` → `merged` / `closed` / `failed`
- **TypeID identifiers**: Tasks use prefixed UUIDs (`tsk_*`) for type-safe identity
- **Task dependencies**: Tasks can depend on other tasks, with validation and execution gating
- **Acceptance criteria**: Optional criteria passed to the agent for validation and reporting
- **Optimistic locking**: Concurrent task claiming without race conditions

## Retry System

- **Configurable retries**: Up to 5 attempts per task (default)
- **Categorized failures**: Retry reasons tracked by category (`ci_failure`, `merge_conflict`)
- **Retry context**: CI failure logs (up to 4KB) and previous agent status preserved across retries
- **Circuit breaker**: Fast-fails after 2 consecutive same-category failures to prevent infinite loops
- **Budget enforcement**: Tasks fail automatically if cumulative cost exceeds `max_cost_usd`

## Cost Tracking

- **Per-task cost accumulation**: Costs reported by the agent via `VERVE_COST` marker
- **Budget limits**: Optional `max_cost_usd` per task with automatic enforcement on retry
- **UI display**: Current cost and budget shown on task detail page and task cards

## Agent Execution

- **Docker isolation**: Each task runs in an ephemeral container, automatically cleaned up
- **Claude Code integration**: Stream-JSON output mode with model selection (haiku, sonnet, opus)
- **Branch management**: Auto-creates `verve/task-{id}` branches; reuses on retry with rebase
- **PR creation**: Automatic PR with Claude-generated title/description via `gh` CLI
- **Dry run mode**: Skip Claude API calls for testing; creates dummy changes with dry-run label
- **Structured agent status**: JSON output with `files_modified`, `tests_status`, `confidence`, `blockers`, `criteria_met`, `notes`

## Prerequisite Checks

- **Multi-language detection**: Go, Python, Rust, Java/Kotlin (Gradle/Maven), Ruby, PHP, .NET, Swift
- **File-based detection**: Scans for manifest files (`go.mod`, `requirements.txt`, `Cargo.toml`, etc.)
- **Description-based detection**: Keyword matching in task descriptions for empty repos
- **Structured failure reporting**: Missing tools reported with installation instructions
- **No wasted tokens**: Checks run before Claude, so API costs are not incurred on prerequisite failures

## Worker

- **Long-poll task claiming**: Atomic status transitions prevent duplicate claims
- **Configurable concurrency**: `MAX_CONCURRENT_TASKS` with semaphore-based control (default: 3)
- **Sequential mode**: Single-task execution for network-restricted environments
- **Graceful shutdown**: Waits for active tasks to complete before stopping
- **Repo-scoped polling**: Workers only poll for their configured `GITHUB_REPOS`
- **Marker protocol**: Parses structured markers from agent output (`VERVE_PR_CREATED`, `VERVE_STATUS`, `VERVE_COST`, `VERVE_PREREQ_FAILED`)

## Log Streaming

- **Real-time batching**: Logs sent every 2 seconds or when buffer reaches 50 lines
- **Docker demultiplexing**: stdout/stderr separated via `stdcopy`
- **SSE streaming**: Dedicated `/tasks/{id}/logs` endpoint with historical replay
- **Auto-scroll UI**: Log viewer with auto-scroll that disables on manual scroll

## GitHub Integration

- **Repository management**: Add/remove repos, list accessible repos for authenticated user
- **PR status sync**: Checks merged status, CI results, and mergeability
- **CI failure analysis**: Fetches failed check run logs (last 50 lines per job, 4KB total)
- **Background sync**: Every 30 seconds, syncs all tasks in `review` status
- **Auto-retry on CI failure**: Retries with `ci_failure` category and truncated logs as context
- **Auto-retry on merge conflict**: Retries with `merge_conflict` category for automatic rebase

## Multi-Repository Support

- **Repo-scoped tasks**: Each task belongs to a specific repository
- **Repo selector UI**: Dashboard filters by selected repository
- **Worker repo filtering**: Workers configured with specific repos via `GITHUB_REPOS`
- **Repo-filtered events**: SSE subscriptions scoped to selected repository

## API

- **RESTful endpoints**: Full CRUD for tasks and repos under `/api/v1`
- **Long-poll**: `GET /tasks/poll` holds connection for up to 30 seconds
- **SSE events**: `GET /events` streams `task_created`, `task_updated`, `logs_appended`
- **Task operations**: Create, list, get, close, complete, sync, append logs
- **Repo operations**: List, add, remove, list available from GitHub

## Database

- **Dual backend**: PostgreSQL (production) and SQLite in-memory (development)
- **Repository pattern**: Interface-based abstraction with interchangeable implementations
- **Auto-migrations**: Embedded SQL migrations run on startup
- **sqlc generation**: Type-safe queries generated from SQL definitions
- **PostgreSQL features**: Connection pooling (pgx/v5), NOTIFY/LISTEN for cross-instance events, ENUM types, array support
- **SQLite features**: Zero-config in-memory mode, JSON array encoding for complex fields

## Event System

- **In-process fan-out**: Broker distributes events to SSE subscribers with buffered channels
- **PostgreSQL NOTIFY/LISTEN**: Multi-instance event distribution with auto-reconnect
- **Event types**: `task_created`, `task_updated`, `logs_appended`
- **Init snapshot**: SSE connections receive full task list on connect

## UI

- **Kanban dashboard**: Six status columns with task count badges
- **Real-time updates**: SSE-driven live task state changes
- **Task detail page**: Description (markdown), status, retries, logs, agent status, cost, dependencies, PR link, acceptance criteria, prerequisite failures
- **Create task dialog**: Description, acceptance criteria, dependency search/selection, max cost budget
- **Task cards**: Preview with retry count, cost, dependency count, consecutive failure warnings
- **Repository management**: Selector dropdown, add from GitHub with search, remove repos
- **Close task**: Dialog with optional reason
- **Sync PRs**: Manual sync button with result summary

## Security & Isolation

- **User code stays on-premise**: Only task descriptions flow in; logs and PR notifications flow out
- **Docker container isolation**: Each agent runs in its own ephemeral container
- **Credential isolation**: GitHub tokens injected via environment variables, never stored server-side
- **Worker authentication**: Per-user API keys for worker-to-server communication
