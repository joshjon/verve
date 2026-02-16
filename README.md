# Verve

An AI agent orchestrator that dispatches [Claude Code](https://docs.anthropic.com/en/docs/claude-code) agents to work on
tasks in isolated Docker containers. User source code never leaves their network.

## How It Works

```
Your Cloud                              User Environment
┌───────────────────────────┐          ┌───────────────────────────┐
│ Postgres ◄─► API Server   │◄─ HTTPS ─│ Worker                    │
│              ◄─► Web UI   │          │   └─► Agent containers    │
└───────────────────────────┘          └───────────────────────────┘
```

1. You create a task in the web UI with a description and target repository
2. A worker (running in the user's environment) long-polls and claims the task
3. The worker spawns an isolated Docker container running Claude Code
4. The agent makes code changes, commits, and opens a pull request
5. Logs stream back in real-time and PR status is monitored automatically
6. If CI fails, the task is automatically retried with failure context

## Quick Start

### Prerequisites

- Docker
- [GitHub Personal Access Token](https://github.com/settings/tokens) (with `repo` scope)
- [Anthropic API Key](https://console.anthropic.com)

### 1. Set credentials

Copy `.env.example` to `.env` and fill in your keys.

### 2. Start the stack

```bash
make build-agent  # Build the agent Docker image
make up           # Start PostgreSQL, API server, and worker
```

### 3. Open the dashboard

Go to [http://localhost:7400](http://localhost:7400) to create tasks, monitor agents, and view logs.

### Useful commands

```bash
make logs    # Tail container logs
make down    # Stop everything
```

## Custom Agent Images

The base agent image includes Node.js and common tools. If your project needs additional dependencies (Go, Python, Rust,
etc.), create a custom Dockerfile:

```dockerfile
FROM ghcr.io/joshjon/verve-agent:latest

USER root
COPY --from=golang:1.23-alpine /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
USER agent
```

See [`agent/examples/`](agent/examples/) for more examples.

## Tech Stack

- **Go** — API server and worker
- **SvelteKit** — Web UI
- **PostgreSQL** / SQLite — Database (Postgres for production, SQLite in-memory for dev)
- **Docker** — Agent container isolation
- **Claude Code** — AI coding agent
