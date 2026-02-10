# Verve

A distributed AI agent orchestrator platform. Dispatches AI coding agents powered by Claude Code to work on tasks within customer infrastructure using Docker-in-Docker isolation.

See [DESIGN.md](DESIGN.md) for detailed architecture and design documentation.

## Prerequisites

- Go 1.22+
- Docker
- GitHub Personal Access Token (with repo permissions)
- Anthropic API Key (for Claude Code)

## Quick Start

### 1. Configure credentials

Set the required environment variables:

```bash
# GitHub Personal Access Token with repo read/write permissions
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"

# Repository to work on (format: owner/repo)
export GITHUB_REPO="your-username/your-repo"

# Anthropic API key for Claude Code
export ANTHROPIC_API_KEY="sk-ant-xxxxxxxxxxxxxxxxxxxx"
```

### 2. Build everything

```bash
# Build the agent Docker image (includes Claude Code CLI)
make build-agent

# Build the server and worker binaries
make build
```

### 3. Start the API server

```bash
make run-server
```

The server runs on `http://localhost:8080`.

### 4. Start the worker (in a separate terminal)

```bash
# Make sure credentials are exported in this terminal too
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"
export GITHUB_REPO="your-username/your-repo"
export ANTHROPIC_API_KEY="sk-ant-xxxxxxxxxxxxxxxxxxxx"

make run-worker
```

The worker connects to the API server and polls for tasks.

### 5. Create a task

```bash
# Create a new task - Claude Code will implement it
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"description":"Add a hello world function to main.py"}'
```

### 6. Monitor progress

```bash
# List all tasks
make list-tasks

# Get a specific task (includes real-time logs from Claude Code)
make get-task ID=tsk_xxxxxxxx
```

When the task completes:
- A new branch `verve/task-{task_id}` is pushed to your repository
- A pull request is automatically created with an AI-generated description
- Task status changes to `review` with the PR URL

## Task Statuses

| Status | Description |
|--------|-------------|
| `pending` | Task created, waiting to be claimed |
| `running` | Task claimed by worker, agent executing |
| `review` | PR created, awaiting human review/merge |
| `merged` | PR has been merged |
| `completed` | Task completed without changes |
| `failed` | Task failed with error |

## Configuration

### Server Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITHUB_TOKEN` | No | - | GitHub token for PR status sync |
| `GITHUB_REPO` | No | - | Repository for PR status sync (format: `owner/repo`) |

The server uses these credentials to check if PRs have been merged (background sync every 30 seconds).

### Worker Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITHUB_TOKEN` | Yes | - | GitHub Personal Access Token with repo permissions |
| `GITHUB_REPO` | Yes | - | Target repository (format: `owner/repo`) |
| `ANTHROPIC_API_KEY` | Yes | - | Anthropic API key for Claude Code |
| `API_URL` | No | `http://localhost:8080` | Verve API server URL |
| `CLAUDE_MODEL` | No | `haiku` | Claude model to use (`haiku`, `sonnet`, `opus`) |

### Getting a GitHub Token

1. Go to GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate a new token with the following permissions:
   - `repo` (Full control of private repositories)
3. Copy the token and set it as `GITHUB_TOKEN`

### Getting an Anthropic API Key

1. Go to [console.anthropic.com](https://console.anthropic.com)
2. Create an API key
3. Copy the key and set it as `ANTHROPIC_API_KEY`

## How It Works

1. **Create Task**: You submit a task description via the API (optionally with dependencies)
2. **Worker Claims Task**: The worker polls for pending tasks with met dependencies and claims one
3. **Agent Spawns**: Worker creates an isolated Docker container with:
   - Git (configured with your GitHub token)
   - Claude Code CLI (configured with your Anthropic API key)
   - GitHub CLI for PR creation
4. **Clone & Branch**: Agent clones your repo and creates a task branch
5. **Claude Code Runs**: Claude Code implements the task, making code changes
6. **Commit & Push**: Agent commits changes and pushes the branch
7. **Create PR**: Agent creates a pull request with AI-generated title and description
8. **Await Review**: Task enters `review` status with PR URL, awaiting human merge
9. **Merged**: Once PR is merged, background sync updates status to `merged`

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tasks` | Create a new task |
| GET | `/api/v1/tasks` | List all tasks |
| GET | `/api/v1/tasks/:id` | Get task details with logs |
| GET | `/api/v1/tasks/poll` | Long-poll for pending tasks (worker) |
| POST | `/api/v1/tasks/:id/logs` | Append logs (worker) |
| POST | `/api/v1/tasks/:id/complete` | Mark task complete (worker) |
| POST | `/api/v1/tasks/:id/sync` | Refresh PR merge status from GitHub |

### Create Task with Dependencies

```bash
# Create a task that depends on another task
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Add unit tests for the hello world function",
    "depends_on": ["tsk_abc123"]
  }'
```

Dependent tasks remain in `pending` until all parent tasks reach `review`, `completed`, or `merged` status.

### Task Response

```json
{
  "id": "tsk_xyz789",
  "description": "Add a hello world function",
  "status": "review",
  "pull_request_url": "https://github.com/owner/repo/pull/42",
  "pr_number": 42,
  "depends_on": ["tsk_abc123"],
  "logs": ["[agent] Starting...", "..."],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T11:45:00Z"
}
```

## Project Structure

```
verve/
├── cmd/
│   ├── server/         # API server entrypoint
│   └── worker/         # Worker entrypoint
├── internal/
│   ├── server/         # API server implementation
│   └── worker/         # Worker and Docker management
├── agent/              # Agent Docker image (with Claude Code)
├── bin/                # Compiled binaries
└── Makefile
```

## Make Targets

```bash
make build          # Build server and worker
make build-agent    # Build agent Docker image
make run-server     # Run API server
make run-worker     # Run worker (requires env vars)
make test-task      # Create a test task
make list-tasks     # List all tasks
make get-task ID=x  # Get specific task
make clean          # Remove binaries and images
make tidy           # Run go mod tidy
```

## Security Notes

- **Never commit credentials**: Keep tokens in environment variables only
- **Minimal permissions**: Use a GitHub token with only the permissions you need
- **Container isolation**: Each task runs in an isolated Docker container
- **No credential storage**: Credentials are passed at runtime, never baked into images
