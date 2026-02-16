FROM node:22-alpine

LABEL org.opencontainers.image.title="Verve Agent"
LABEL org.opencontainers.image.description="Base agent image for Verve AI task orchestrator"
LABEL org.opencontainers.image.source="https://github.com/joshjon/verve"
LABEL org.opencontainers.image.vendor="Verve"
LABEL verve.extensible="true"

# Install system dependencies
RUN apk add --no-cache \
    git \
    curl \
    bash \
    jq \
    ca-certificates

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user (Claude Code refuses --dangerously-skip-permissions as root)
RUN adduser -D -s /bin/bash agent

# Create workspace directory with proper ownership
RUN mkdir -p /workspace && chown agent:agent /workspace
WORKDIR /workspace

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to non-root user
USER agent

# Environment variables (set at runtime by worker)
ENV TASK_ID=""
ENV TASK_DESCRIPTION=""
ENV GITHUB_TOKEN=""
ENV GITHUB_REPO=""
ENV ANTHROPIC_API_KEY=""

ENTRYPOINT ["/entrypoint.sh"]
