FROM node:22-alpine

LABEL org.opencontainers.image.title="Verve Base"
LABEL org.opencontainers.image.description="Base agent image (verve:base) for Verve AI task orchestrator"
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

# Copy entrypoint and library scripts
COPY entrypoint.sh /entrypoint.sh
COPY lib/ /lib/
RUN chmod +x /entrypoint.sh /lib/*.sh

# Switch to non-root user
USER agent

# Environment variables (set at runtime by worker)
ENV TASK_ID=""
ENV TASK_DESCRIPTION=""
ENV GITHUB_TOKEN=""
ENV GITHUB_REPO=""
ENV ANTHROPIC_API_KEY=""

# Dependency cache directory (mounted as a host volume by worker).
# Language-specific cache paths point to subdirectories under /cache
# so that a single volume mount caches all package managers.

# Go
ENV GOMODCACHE="/cache/go/mod"
ENV GOPATH="/cache/go/path"

# Node.js / npm / pnpm / yarn
ENV NPM_CONFIG_CACHE="/cache/npm"
ENV PNPM_HOME="/cache/pnpm"
ENV YARN_CACHE_FOLDER="/cache/yarn"

# Python / pip
ENV PIP_CACHE_DIR="/cache/pip"

# Rust / Cargo
ENV CARGO_HOME="/cache/cargo"

# Java / Maven / Gradle
ENV MAVEN_OPTS="-Dmaven.repo.local=/cache/maven"
ENV GRADLE_USER_HOME="/cache/gradle"

# C# / NuGet
ENV NUGET_PACKAGES="/cache/nuget"

ENTRYPOINT ["/entrypoint.sh"]
