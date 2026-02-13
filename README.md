# Verve

A distributed AI agent orchestrator. Dispatches Claude Code agents to work on tasks within user infrastructure using isolated Docker containers. User source code never leaves their network — task descriptions go in, logs and PRs come out.

## Quick Start

### Prerequisites

- Docker
- [GitHub Personal Access Token](https://github.com/settings/tokens) (with repo permissions)
- [Anthropic API Key](https://console.anthropic.com)

### 1. Set credentials

Create a `.env` file (see `.env.example`):

```bash
GITHUB_TOKEN=ghp_...
GITHUB_REPO=owner/repo
ANTHROPIC_API_KEY=sk-ant-...
```

### 2. Start the stack

```bash
# Build the agent image (needed for the worker to spawn containers)
make build-agent

# Start PostgreSQL, API server, and worker
make up
```

This starts four containers:
- **postgres** — PostgreSQL 16 database
- **server** — API server on `http://localhost:7400`
- **ui** — Web dashboard on `http://localhost:8080`
- **worker** — polls for tasks and spawns agent containers

Useful commands:

```bash
make logs     # Tail all container logs
make down     # Stop everything
```

### 3. Open the dashboard

Open [http://localhost:8080](http://localhost:8080) to create tasks, monitor progress, and view agent logs.

When complete, the agent pushes a branch and opens a PR on your repository.

## Documentation

- [Configuration](docs/configuration.md) — environment variables and credentials
- [API Reference](docs/api.md) — endpoints, request/response formats, task statuses
- [Custom Agents](docs/custom-agents.md) — extending the agent image with additional dependencies
- [Design](DESIGN.md) — architecture and design decisions
