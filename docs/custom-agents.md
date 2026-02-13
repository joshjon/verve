# Custom Agent Images

The base `verve-agent:latest` image includes Node.js, Git, and GitHub CLI. If your project requires additional dependencies (Python, Go, Rust, etc.), you can extend the base image.

## Creating a Custom Agent Image

Create a Dockerfile that extends the base image:

```dockerfile
# Dockerfile.custom
FROM verve-agent:latest

USER root

# Install your dependencies
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

USER agent
```

Build and use your custom image:

```bash
# Build the base image first
make build-agent

# Build your custom image
docker build -f Dockerfile.custom -t verve-agent:custom .

# Run the worker with your custom image
AGENT_IMAGE=verve-agent:custom make run-worker
```

## Example Dockerfiles

Pre-built examples are available in `agent/examples/`:

| File | Description |
|------|-------------|
| `Dockerfile.python` | Python 3 with pip and venv |
| `Dockerfile.golang` | Go 1.22 |
| `Dockerfile.full` | Python, Go, and Rust (larger image) |

Build an example:

```bash
docker build -f agent/examples/Dockerfile.python -t verve-agent:python ./agent
AGENT_IMAGE=verve-agent:python make run-worker
```
