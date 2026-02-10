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

    # Create Pull Request (gh CLI uses GITHUB_TOKEN env var automatically)
    echo "[agent] Creating pull request..."

    # Get diff summary for PR description context
    DIFF_SUMMARY=$(git diff origin/main...HEAD --stat 2>/dev/null | tail -20 || echo "Changes made by Verve Agent")

    # Generate PR title and description using Claude
    PR_PROMPT="Generate a pull request title and description for the following task and changes.

Task: ${TASK_DESCRIPTION}

Files changed:
${DIFF_SUMMARY}

Respond with ONLY valid JSON in this exact format (no markdown, no code blocks, no extra text):
{\"title\": \"Short descriptive title (max 72 chars)\", \"description\": \"## Summary\\n\\nBrief description of changes.\\n\\n## Changes\\n\\n- Bullet points of what was done\"}"

    echo "[agent] Generating PR description with Claude..."
    PR_JSON=$(claude --print --model "${CLAUDE_MODEL}" "${PR_PROMPT}" 2>/dev/null || echo "")

    # Extract title and description, with fallbacks
    if [ -n "${PR_JSON}" ]; then
        PR_TITLE=$(echo "${PR_JSON}" | jq -r '.title // empty' 2>/dev/null || echo "")
        PR_BODY=$(echo "${PR_JSON}" | jq -r '.description // empty' 2>/dev/null || echo "")
    fi

    # Fallback if Claude/jq failed
    if [ -z "${PR_TITLE}" ]; then
        PR_TITLE="${TASK_DESCRIPTION}"
    fi
    if [ -z "${PR_BODY}" ]; then
        PR_BODY="## Summary

Automated implementation of: ${TASK_DESCRIPTION}

## Changes

${DIFF_SUMMARY}"
    fi

    # Append task metadata to PR body
    PR_BODY="${PR_BODY}

---
*Implemented by [Verve AI Agent](https://github.com/verve-ai/verve)*
**Task ID:** \`${TASK_ID}\`"

    # Create the PR
    PR_OUTPUT=$(gh pr create --title "${PR_TITLE}" --body "${PR_BODY}" --head "${BRANCH}" 2>&1)
    PR_EXIT_CODE=$?

    if [ ${PR_EXIT_CODE} -eq 0 ]; then
        # Extract the PR URL from output
        PR_URL=$(echo "${PR_OUTPUT}" | grep -oE 'https://github.com/[^[:space:]]+/pull/[0-9]+' | head -1)
        if [ -n "${PR_URL}" ]; then
            PR_NUMBER=$(echo "${PR_URL}" | grep -oE '[0-9]+$')
            echo "[agent] Pull request created: ${PR_URL}"
            # Output structured marker for worker to parse
            echo "VERVE_PR_CREATED:{\"url\":\"${PR_URL}\",\"number\":${PR_NUMBER}}"
        else
            echo "[agent] Pull request created but could not parse URL from: ${PR_OUTPUT}"
        fi
    else
        echo "[agent] Warning: Failed to create pull request: ${PR_OUTPUT}"
    fi
fi

echo ""
echo "=== Task Completed Successfully ==="
echo "Branch: ${BRANCH}"
