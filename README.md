# Verve

A distributed AI agent orchestrator. Dispatches Claude Code agents to work on tasks within user infrastructure using isolated Docker containers. User source code never leaves their network — task descriptions go in, logs and PRs come out.

## Quick Start

### Prerequisites

- Docker
- [GitHub Personal Access Token](https://github.com/settings/tokens) (with repo permissions)
- [Anthropic API Key](https://console.anthropic.com)

### 1. Set credentials

Create a `.env` file (see `.env.example`):

### 2. Start the stack

```bash
# Build the agent image (needed for the worker to spawn containers)
make build-agent

# Start PostgreSQL, API server, and worker
make up
```

This starts four containers:
- **postgres** — PostgreSQL 16 database
- **server** — UI/API server on `http://localhost:7400`
- **worker** — polls for tasks and spawns agent containers

Useful commands:

```bash
make logs     # Tail all container logs
make down     # Stop everything
```

### 3. Open the dashboard

Open [http://localhost:7400](http://localhost:7400) to create tasks, monitor progress, and view agent logs.

When complete, the agent pushes a branch and opens a PR on your repository.

## Try It Out

Use the [verve-example](https://github.com/joshjon/verve-example) repo to see the full task lifecycle in action, including CI failure detection and automatic retries.

### Setup

1. Fork [joshjon/verve-example](https://github.com/joshjon/verve-example) to your GitHub account
2. Add the forked repo in the Verve dashboard
3. Create a task with the details below

### Sample task

**Title:** Add readability function

**Description:**
Implement and export a `readability(text)` function in `src/textstats.js` that calculates a Flesch-Kincaid grade level score. It should return an object with `grade` (number, rounded to 1 decimal place) and `level` (string: "easy", "moderate", or "difficult").

**Acceptance Criteria:**
1. All tests pass
2. Lint checks pass

### What to expect

1. The agent implements the `readability` function, makes tests and lint pass
2. The agent pushes a branch and opens a PR
3. CI runs three checks: **test**, **lint**, and **changelog** validation
4. The **changelog** check fails because the agent didn't add an entry to `CHANGELOG.md` (it wasn't mentioned in the task)
5. Verve detects the CI failure and automatically retries the task with the failure context
6. On retry, the agent reads the CI error, adds a changelog entry, and pushes the fix
7. All CI checks pass and the task moves to review
