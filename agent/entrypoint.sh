#!/bin/bash
set -e

echo "=== Verve Agent Starting ==="
echo "Task ID: ${TASK_ID}"
echo "Repository: ${GITHUB_REPO}"
echo "Description: ${TASK_DESCRIPTION}"
echo ""

# Validate required environment variables
if [ -z "$GITHUB_TOKEN" ]; then
    echo "[error] GITHUB_TOKEN is not set"
    exit 1
fi

if [ -z "$GITHUB_REPO" ]; then
    echo "[error] GITHUB_REPO is not set"
    exit 1
fi

if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "[error] ANTHROPIC_API_KEY is not set"
    exit 1
fi

# Configure git (running as non-root 'agent' user)
echo "[agent] Configuring git..."
git config --global credential.helper store
echo "https://${GITHUB_TOKEN}@github.com" > /home/agent/.git-credentials
git config --global user.email "verve-agent@verve.ai"
git config --global user.name "Verve Agent"

# Clone repository (embed token in URL for push access)
echo "[agent] Cloning repository: ${GITHUB_REPO}..."
git clone "https://${GITHUB_TOKEN}@github.com/${GITHUB_REPO}.git" /workspace/repo
cd /workspace/repo

# Create task branch
BRANCH="verve/task-${TASK_ID}"
echo "[agent] Creating branch: ${BRANCH}"
git checkout -b "${BRANCH}"

# Run Claude Code
echo "[agent] Starting Claude Code session..."
echo "[agent] Task: ${TASK_DESCRIPTION}"
echo ""

# Run Claude Code in non-interactive mode
# --print: Output to stdout (for log streaming)
# --dangerously-skip-permissions: Skip permission prompts (we trust the agent)
# --model: Use specified model (defaults to haiku for cost efficiency)
CLAUDE_MODEL="${CLAUDE_MODEL:-haiku}"
echo "[agent] Using model: ${CLAUDE_MODEL}"
claude --print --dangerously-skip-permissions --model "${CLAUDE_MODEL}" "${TASK_DESCRIPTION}"

echo ""
echo "[agent] Claude Code session completed"

# Commit and push changes
echo "[agent] Checking for changes..."
git add -A

if git diff --cached --quiet; then
    echo "[agent] No changes to commit"
else
    echo "[agent] Committing changes..."
    git commit -m "feat: ${TASK_DESCRIPTION}

Implemented by Verve AI Agent
Task ID: ${TASK_ID}"

    echo "[agent] Pushing branch to origin..."
    git push -u origin "${BRANCH}"
    echo "[agent] Branch pushed successfully: ${BRANCH}"
fi

echo ""
echo "=== Task Completed Successfully ==="
echo "Branch: ${BRANCH}"
