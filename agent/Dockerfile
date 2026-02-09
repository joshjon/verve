FROM node:20-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user (Claude Code refuses --dangerously-skip-permissions as root)
RUN useradd -m -s /bin/bash agent

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
ENV CLAUDE_MODEL="haiku"

ENTRYPOINT ["/entrypoint.sh"]
