# Distributed AI Agent Orchestrator — Design Overview

## What is this?

A platform that allows us to dispatch AI coding agents to work on tasks within a customer's own infrastructure. The system is split into two halves: a **central cloud platform** we control (task management, UI, logging) and a **worker component** that runs inside the customer's environment (task execution, code changes, PRs). The customer deploys a single Docker container that long-polls for work, spins up isolated agent sessions, streams logs back to us, and raises pull requests when done.

The key design goal is that the customer's source code and secrets never leave their network. We send task descriptions *in*; we get logs and PR notifications *out*. The actual code manipulation happens entirely on their side.

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│                 Internal Cloud                   │
│                                                  │
│   ┌──────────┐    ┌───────────┐    ┌──────────┐ │
│   │ Postgres │◄──►│ API Server│◄──►│    UI    │ │
│   │   (DB)   │    │           │    │          │ │
│   └──────────┘    └─────┬─────┘    └──────────┘ │
│                         │                        │
└─────────────────────────┼────────────────────────┘
                          │  HTTPS (outbound from worker)
                          │
┌─────────────────────────┼────────────────────────┐
│   Customer Environment  │                        │
│                         │                        │
│              ┌──────────▼──────────┐             │
│              │ Orchestrator Worker │             │
│              │  (Docker container) │             │
│              └──────┬───────┬──────┘             │
│                     │       │                    │
│            ┌────────▼──┐ ┌──▼────────┐           │
│            │  Agent 1  │ │  Agent 2  │  ...      │
│            │(ephemeral)│ │(ephemeral)│           │
│            └───────────┘ └───────────┘           │
│                                                  │
│          Customer's repos, secrets, CI           │
└──────────────────────────────────────────────────┘
```

---

## Internal Cloud

### Database (Postgres)

Stores the core domain model:

- **Projects** — a customer project linked to a repository.
- **Tasks** — units of work within a project (e.g. "Add input validation to the signup form"). Each task has a status lifecycle: `pending → queued → running → completed | failed`.
- **Task logs** — append-only log lines streamed from the worker, stored for real-time viewing in the UI.
- **PR metadata** — branch name, PR URL, review status, linked task.

### API Server

A REST (or gRPC) service that acts as the interface between the UI, the database, and the remote workers.

Key responsibilities:

- **Task queue** — exposes a long-poll endpoint (e.g. `GET /tasks/poll?worker_id=...`) that workers call to claim the next available task. This is conceptually similar to how a Temporal worker polls for activity tasks. The server marks the task as `running` and assigns it to the requesting worker.
- **Log ingestion** — accepts batched log uploads from workers (`POST /tasks/{id}/logs`). Logs are appended to the database and pushed to any connected UI clients via WebSockets or SSE.
- **Task lifecycle** — endpoints to create, update, and complete tasks. The worker calls back to report success/failure, branch name, and PR URL.
- **Notifications** — triggers notifications to the UI (and potentially Slack, email, etc.) when a task's PR is ready for review.
- **Authentication** — workers authenticate using a per-customer API key or short-lived token. The UI authenticates via standard session/OAuth.

### UI

A web application for the team operating the platform.

Core views:

- **Task board** — create, prioritise, and assign tasks to projects. View task status at a glance.
- **Live agent logs** — real-time streaming view of an agent's stdout as it works, useful for debugging and monitoring.
- **PR review queue** — list of completed tasks with links to the raised PRs. Status indicators for review state (open, approved, merged).
- **Notifications** — in-app and push notifications when an agent finishes a task and a PR is ready.

---

## Customer-Side: Orchestrator Worker

### Deployment

The worker is distributed as a **Docker image** that the customer runs in their environment (Kubernetes, ECS, a VM — whatever they use). The image contains a single Go (or similar) binary as its entrypoint. The customer provides configuration at deploy time:

- API server URL and authentication token.
- Git credentials (SSH key or token) for cloning repos and pushing branches.
- Resource limits (max concurrent agents, memory/CPU caps).
- Choice of isolation mode (see below).

### Task Polling Loop

The worker binary runs a continuous loop:

1. **Long-poll** the API server's task queue endpoint. The connection stays open until a task is available or a timeout is reached (then it reconnects).
2. **Claim** a task. The API server atomically assigns it to this worker.
3. **Provision** an isolated environment for the agent (see isolation modes below).
4. **Invoke** the AI agent process, passing it the task description, repo details, and any relevant context.
5. **Stream logs** — the agent writes to stdout/a log file. The worker watches the log file using `fsnotify` (file system event notifications), batches new lines, and pushes them to the API server periodically (e.g. every 1–2 seconds or every N lines).
6. **Wait for completion** — when the agent process exits, the worker inspects the result.
7. **Push & PR** — if successful, the worker (or agent) pushes the working branch to the remote and opens a pull request via the platform's API (GitHub, GitLab, etc.).
8. **Report back** — notify the API server with the task outcome (success/failure), branch name, and PR URL.
9. **Clean up** — tear down the ephemeral environment.
10. **Loop** — go back to step 1.

### Agent Isolation Modes

Two options for isolating each agent's workspace. The choice likely depends on the customer's security posture and infrastructure.

| | Docker-in-Docker (DinD) | Working Directory |
|---|---|---|
| **How it works** | The worker spawns a new Docker container for each task. The agent runs inside this inner container with its own filesystem, network, and process space. | The worker creates a new directory on the host filesystem, clones the repo into it, and runs the agent process directly. |
| **Isolation** | Strong — full container boundary. Each agent is sandboxed from others and from the worker. | Weak — process-level only. Agents share the worker's kernel and could theoretically interfere with each other. |
| **Requirements** | Docker socket access or a sidecar Docker daemon. Adds complexity (privileged mode or alternatives like sysbox). | Minimal — just filesystem access. Simpler to set up. |
| **Cleanup** | Destroy the container. Clean and reliable. | Delete the working directory. Risk of leftover processes or temp files. |
| **Best for** | Production / security-sensitive customers. | Development, quick iteration, or trusted environments. |

The DinD approach is the stronger default for production. The working directory mode is useful for simpler setups or during development of the platform itself.

### The AI Agent

Each task spawns an ephemeral agent process. The agent is expected to:

1. Clone the relevant repository (or receive a pre-cloned working copy).
2. Read and understand the task description.
3. Make the required code changes — editing files, running tests, fixing lint errors, etc.
4. Commit changes to a new branch.
5. Exit with a success/failure code.

The agent's stdout is its log stream — everything it prints is captured by the worker and relayed to the API server. This means the agent should be reasonably verbose about what it's doing (e.g. "Reading file X", "Running tests", "Test failed, retrying with fix...").

The agent could be backed by any capable coding LLM (Claude, etc.) and wrapped in a framework that gives it tool access — shell commands, file read/write, git operations, and so on.

### Log Streaming Detail

The worker uses **fsnotify** (Go's filesystem notification library) to watch the agent's log file for changes. When new data is written:

1. Read the new bytes from the last-known offset.
2. Buffer them until a flush condition is met (time interval or byte threshold).
3. POST the batch to the API server (`POST /tasks/{id}/logs`).
4. The API server appends to the database and fans out to connected UI clients.

This gives near-real-time log visibility without requiring the agent itself to know anything about the API server.

---

## Task Lifecycle (End to End)

1. **User creates a task** in the UI — provides a description, selects a project/repo, and optionally sets priority.
2. **Task enters the queue** with status `pending`.
3. **Worker picks it up** via long-poll → status becomes `running`.
4. **Agent works** — logs stream to the UI in real time.
5. **Agent finishes** — worker pushes branch and opens PR.
6. **Worker reports completion** → status becomes `completed` (or `failed`).
7. **User gets notified** — "PR ready for review" with a direct link.
8. **User reviews and merges** the PR through their normal Git workflow.

---

## Open Questions / Things to Decide

- **Task schema** — what metadata does a task carry? Just a description, or structured fields like target files, acceptance criteria, test commands?
- **Retry / failure handling** — if an agent fails mid-task, do we retry automatically? How many times? Do we resume or start fresh?
- **Concurrency** — how many agents can a single worker run in parallel? Is this configurable per customer?
- **Agent framework** — what specific LLM and tooling framework powers the agent? Is this pluggable?
- **Git workflow** — do we enforce a branch naming convention? Who owns the PR template? Do we auto-request reviewers?
- **Security** — how do we handle secrets the agent might need (e.g. API keys for external services)? Are these injected into the agent's environment?
- **Observability** — beyond logs, do we want metrics (agent duration, token usage, success rate)?
- **Multi-repo tasks** — can a single task span changes across multiple repositories?
- **Customer onboarding** — what's the minimal setup for a new customer? Docker image pull + config file + API key?
