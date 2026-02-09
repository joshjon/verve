# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Verve is a distributed AI agent orchestrator platform. It dispatches AI coding agents to work on tasks within customer infrastructure. The system has two halves:

1. **Internal Cloud** (we control): API server, Postgres database, and web UI for task management
2. **Orchestrator Worker** (customer deploys): Docker container that long-polls for work, runs isolated agents, streams logs, and creates PRs

Key constraint: Customer source code and secrets never leave their network. We send task descriptions in; we get logs and PR notifications out.

## Development Commands

```bash
# Build
make build                        # Build server and worker binaries
make build-server                 # Build API server only
make build-worker                 # Build worker only
make build-agent                  # Build agent Docker image

# Run
make run-server                   # Start API server (port 8080)
make run-worker                   # Start worker (connects to localhost:8080)

# Test
make test-task                    # Create a test task via curl
make list-tasks                   # List all tasks
make get-task ID=tsk_xxx          # Get specific task details

# Clean
make clean                        # Remove binaries and Docker image
make tidy                         # Run go mod tidy
```

## Technology Stack

- **Language**: Go 1.22+
- **HTTP Framework**: Echo v4
- **Container Runtime**: Docker (via Docker SDK for Go)
- **Database**: In-memory (PoC), PostgreSQL planned for production
- **Testing**: testify (require/assert) - planned
- **Release**: GoReleaser - planned

## Architecture

```
Internal Cloud                          Customer Environment
┌─────────────────────────┐            ┌─────────────────────────┐
│ Postgres ◄─► API Server │◄── HTTPS ──│ Orchestrator Worker     │
│              ◄─► UI     │            │   └─► Agent containers  │
└─────────────────────────┘            └─────────────────────────┘
```

## Mono Repo Structure

```
verve/
├── cmd/
│   ├── server/main.go      # API server entrypoint
│   └── worker/main.go      # Worker entrypoint
├── internal/
│   ├── server/
│   │   ├── server.go       # Echo HTTP server setup
│   │   ├── handlers.go     # REST API handlers
│   │   └── store.go        # In-memory task store (PoC)
│   └── worker/
│       ├── worker.go       # Polling loop and task execution
│       └── docker.go       # Docker SDK integration
├── agent/
│   ├── Dockerfile          # Agent container image
│   └── entrypoint.sh       # Agent execution script
├── bin/                    # Compiled binaries (gitignored)
├── go.mod
└── Makefile
```

## Database Layer

Currently using in-memory storage for the PoC. Future implementation will use:

### Repository Pattern
- Common `Repository` interface with sub-interfaces per entity
- PostgreSQL and SQLite both implement the same interface
- Allows swapping databases without changing business logic

### SQLC Conventions (planned)
- Query files in `postgres/queries/*.sql` and `sqlite/queries/*.sql`
- Use `-- name: QueryName :one/:many/:exec` comment syntax
- Generated code in `*/sqlc/` directories

## API Structure

Base path: `/api/v1`

```
/tasks
├── POST                     # Create task
├── GET                      # List all tasks
└── /{task_id}
    ├── GET                  # Get task details with logs
    ├── /logs                # POST logs from worker
    └── /complete            # POST completion status from worker

/tasks/poll                  # Long-poll for pending tasks (worker calls this)
```

## Worker-Cloud Communication

Worker communicates with API server via REST/JSON:
- `GET /tasks/poll`: Long-poll to claim pending tasks
- `POST /tasks/{id}/logs`: Send collected agent logs
- `POST /tasks/{id}/complete`: Report success/failure

## Entity Model

### Task Status Lifecycle
```
pending → running → completed | failed
```

### Entity Identity Pattern
Use TypeID prefixes for entity IDs:
- `prj_*` = Project
- `tsk_*` = Task
- `wrk_*` = Worker

## Key Patterns

### Error Handling
Use semantic error types for consistent HTTP status mapping:
- `errtag.NotFound` → 404
- `errtag.AlreadyExists` → 409
- `errtag.InvalidArgument` → 400

### Configuration
- YAML files for structured configuration
- Environment variable overrides supported
- Implement `Validation()` method on config structs

### Log Streaming
Worker streams logs from Docker container in real-time:
1. Attaches to container stdout/stderr with `Follow=true`
2. Demultiplexes the Docker stream using `stdcopy`
3. Buffers lines and sends batches to API server every 2 seconds (or when buffer reaches 50 lines)
4. UI can poll `/tasks/{id}` to see logs incrementally as the agent runs

### Agent Isolation
Uses Docker-in-Docker approach:
- Each task spawns an isolated Docker container
- Container receives task via environment variables (TASK_ID, TASK_DESCRIPTION)
- Container is automatically removed after execution
- Agent image: `verve-agent:latest`

## Important Notes

- Customer code never leaves their network - only task descriptions flow in, logs and PR notifications flow out
- Workers authenticate with per-customer API keys
- Task queue uses long-polling (worker initiates connection, server holds until task available)
- Agents are ephemeral - one process per task, destroyed after completion
- PR creation happens on customer side using their Git credentials
