# Configuration

## Server Environment Variables

| Variable             | Required | Default | Description                                                    |
|----------------------|----------|---------|----------------------------------------------------------------|
| `DATABASE_URL`       | No       | -       | Set to any value to use PostgreSQL instead of in-memory SQLite |
| `POSTGRES_USER`      | No       | -       | PostgreSQL username                                            |
| `POSTGRES_PASSWORD`  | No       | -       | PostgreSQL password                                            |
| `POSTGRES_HOST_PORT` | No       | -       | PostgreSQL host:port (e.g. `localhost:5432`)                   |
| `POSTGRES_DATABASE`  | No       | -       | PostgreSQL database name                                       |
| `GITHUB_TOKEN`       | No       | -       | GitHub token for PR status sync                                |
| `GITHUB_REPO`        | No       | -       | Repository for PR status sync (format: `owner/repo`)           |

The server uses GitHub credentials to check if PRs have been merged (background sync every 30 seconds). If not set, PR
sync is disabled.

If `DATABASE_URL` is not set, the server falls back to an in-memory SQLite database (data will not persist across
restarts).

## Worker Environment Variables

| Variable               | Required | Default                 | Description                                        |
|------------------------|----------|-------------------------|----------------------------------------------------|
| `GITHUB_TOKEN`         | Yes      | -                       | GitHub Personal Access Token with repo permissions |
| `GITHUB_REPO`          | Yes      | -                       | Target repository (format: `owner/repo`)           |
| `ANTHROPIC_API_KEY`    | Yes      | -                       | Anthropic API key for Claude Code                  |
| `API_URL`              | No       | `http://localhost:7400` | Verve API server URL                               |
| `CLAUDE_MODEL`         | No       | `haiku`                 | Claude model to use (`haiku`, `sonnet`, `opus`)    |
| `AGENT_IMAGE`          | No       | `verve-agent:latest`    | Docker image for agent                             |
| `MAX_CONCURRENT_TASKS` | No       | `3`                     | Maximum tasks to process in parallel               |
| `DRY_RUN`              | No       | -                       | Set to `true` to skip Claude and make dummy changes |

## Getting a GitHub Token

1. Go to GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate a new token with the `repo` permission (full control of private repositories)
3. Copy the token and set it as `GITHUB_TOKEN`

## Getting an Anthropic API Key

1. Go to [console.anthropic.com](https://console.anthropic.com)
2. Create an API key
3. Copy the key and set it as `ANTHROPIC_API_KEY`
