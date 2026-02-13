# API Reference

Base path: `/api/v1`

## Endpoints

| Method | Endpoint              | Description                          |
|--------|-----------------------|--------------------------------------|
| POST   | `/tasks`              | Create a new task                    |
| GET    | `/tasks`              | List all tasks                       |
| GET    | `/tasks/:id`          | Get task details with logs           |
| GET    | `/tasks/poll`         | Long-poll for pending tasks (worker) |
| POST   | `/tasks/:id/logs`     | Append logs (worker)                 |
| POST   | `/tasks/:id/complete` | Mark task complete (worker)          |
| POST   | `/tasks/:id/close`    | Close task with optional reason      |
| POST   | `/tasks/:id/sync`     | Refresh PR merge status from GitHub  |
| POST   | `/tasks/sync`         | Sync all tasks in review status      |

## Create Task

```bash
curl -X POST http://localhost:7400/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"description": "Add a hello world function to main.py"}'
```

### With Dependencies

Tasks can depend on other tasks. A dependent task stays `pending` until all parent tasks reach a terminal state (
`merged` or `closed`).

```bash
curl -X POST http://localhost:7400/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Add unit tests for the hello world function",
    "depends_on": ["tsk_abc123"]
  }'
```

## Task Response

```json
{
  "id": "tsk_xyz789",
  "description": "Add a hello world function",
  "status": "review",
  "pull_request_url": "https://github.com/owner/repo/pull/42",
  "pr_number": 42,
  "depends_on": [
    "tsk_abc123"
  ],
  "logs": [
    "[agent] Starting...",
    "..."
  ],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T11:45:00Z"
}
```

## Task Statuses

| Status    | Description                                 |
|-----------|---------------------------------------------|
| `pending` | Task created, waiting to be claimed         |
| `running` | Task claimed by worker, agent executing     |
| `review`  | PR created, awaiting human review/merge     |
| `merged`  | PR has been merged                          |
| `closed`  | Task closed (manually or no changes needed) |
| `failed`  | Task failed with error                      |
