# Verve

A distributed AI agent orchestrator platform. Dispatches AI coding agents to work on tasks within customer infrastructure using Docker-in-Docker isolation.

See [DESIGN.md](DESIGN.md) for detailed architecture and design documentation.

## Prerequisites

- Go 1.22+
- Docker

## Quick Start

### 1. Build everything

```bash
# Build the agent Docker image
make build-agent

# Build the server and worker binaries
make build
```

### 2. Start the API server

```bash
make run-server
```

The server runs on `http://localhost:8080`.

### 3. Start the worker (in a separate terminal)

```bash
make run-worker
```

The worker connects to the API server and polls for tasks.

### 4. Create a task

```bash
# Create a new task
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"description":"Add input validation to signup form"}'

# Or use the make target
make test-task
```

### 5. Monitor progress

```bash
# List all tasks
make list-tasks

# Get a specific task (includes logs)
make get-task ID=tsk_xxxxxxxx
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tasks` | Create a new task |
| GET | `/api/v1/tasks` | List all tasks |
| GET | `/api/v1/tasks/:id` | Get task details with logs |
| GET | `/api/v1/tasks/poll` | Long-poll for pending tasks (worker) |
| POST | `/api/v1/tasks/:id/logs` | Append logs (worker) |
| POST | `/api/v1/tasks/:id/complete` | Mark task complete (worker) |

## Project Structure

```
verve/
├── cmd/
│   ├── server/         # API server entrypoint
│   └── worker/         # Worker entrypoint
├── internal/
│   ├── server/         # API server implementation
│   └── worker/         # Worker and Docker management
├── agent/              # Agent Docker image
├── bin/                # Compiled binaries
└── Makefile
```

## Make Targets

```bash
make build          # Build server and worker
make build-agent    # Build agent Docker image
make run-server     # Run API server
make run-worker     # Run worker
make test-task      # Create a test task
make list-tasks     # List all tasks
make get-task ID=x  # Get specific task
make clean          # Remove binaries and images
make tidy           # Run go mod tidy
```

## Configuration

### Worker

The worker accepts the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `API_URL` | `http://localhost:8080` | API server URL |

## Development

```bash
# Run both server and worker in separate terminals
make run-server
make run-worker

# Create and monitor tasks
make test-task
make list-tasks
```
