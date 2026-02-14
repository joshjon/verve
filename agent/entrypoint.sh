#!/bin/bash
set -e

echo "=== Verve Agent Starting ==="
echo "Task ID: ${TASK_ID}"
echo "Repository: ${GITHUB_REPO}"
echo "Description: ${TASK_DESCRIPTION}"
if [ "${ATTEMPT:-1}" -gt 1 ]; then
    echo "Attempt: ${ATTEMPT} (retry)"
    echo "Retry Reason: ${RETRY_REASON}"
fi
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

if [ "$DRY_RUN" != "true" ] && [ -z "$ANTHROPIC_API_KEY" ]; then
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

# Branch handling: retry uses existing branch, first attempt creates new
BRANCH="verve/task-${TASK_ID}"
if [ "${ATTEMPT:-1}" -gt 1 ]; then
    echo "[agent] Retry attempt ${ATTEMPT}: checking out existing branch ${BRANCH}"
    git fetch origin "${BRANCH}"
    git checkout "${BRANCH}"

    # For merge conflicts, attempt rebase on main
    if echo "$RETRY_REASON" | grep -qi "merge conflict"; then
        echo "[agent] Rebasing on main to resolve merge conflicts..."
        git fetch origin main
        # Don't fail if rebase has conflicts - Claude will resolve them
        git rebase origin/main || true
    fi
else
    echo "[agent] Creating branch: ${BRANCH}"
    git checkout -b "${BRANCH}"
fi

# Check prerequisites before running Claude Code
echo "[agent] Checking project prerequisites..."

check_prereqs() {
    local missing=()
    local detected=()
    local desc_lower
    desc_lower=$(echo "$TASK_DESCRIPTION" | tr '[:upper:]' '[:lower:]')

    # Go — detect from files or task description
    if [ -f "go.mod" ] || [ -f "go.sum" ] || echo "$desc_lower" | grep -qE '\bgolang\b|\bgo (app|api|server|service|module|project|program|binary|cli)\b'; then
        detected+=("go")
        if ! command -v go &>/dev/null; then
            local reason="go.mod found but go is not installed"
            if [ ! -f "go.mod" ] && [ ! -f "go.sum" ]; then
                reason="Task description references Go but go is not installed"
            fi
            missing+=("{\"tool\":\"go\",\"reason\":\"${reason}\",\"install\":\"Install Go or use a Go-based agent image\"}")
        fi
    fi

    # Python — detect from files or task description
    if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ] || [ -f "setup.py" ] || [ -f "Pipfile" ] || [ -f "poetry.lock" ] || echo "$desc_lower" | grep -qE '\bpython\b|\bdjango\b|\bflask\b|\bfastapi\b|\bpip\b'; then
        detected+=("python")
        if ! command -v python3 &>/dev/null && ! command -v python &>/dev/null; then
            local reason="Python project detected but python3/python not available"
            if [ ! -f "requirements.txt" ] && [ ! -f "pyproject.toml" ] && [ ! -f "setup.py" ] && [ ! -f "Pipfile" ] && [ ! -f "poetry.lock" ]; then
                reason="Task description references Python but python3/python not available"
            fi
            missing+=("{\"tool\":\"python\",\"reason\":\"${reason}\",\"install\":\"Install Python or use a Python-based agent image\"}")
        fi
    fi

    # Rust — detect from files or task description
    if [ -f "Cargo.toml" ] || echo "$desc_lower" | grep -qE '\brust\b|\bcargo\b'; then
        detected+=("rust")
        if ! command -v cargo &>/dev/null; then
            local reason="Cargo.toml found but cargo is not installed"
            if [ ! -f "Cargo.toml" ]; then
                reason="Task description references Rust but cargo is not installed"
            fi
            missing+=("{\"tool\":\"cargo\",\"reason\":\"${reason}\",\"install\":\"Install Rust or use a Rust-based agent image\"}")
        fi
    fi

    # Java/Kotlin (Gradle) — detect from files or task description
    if [ -f "build.gradle" ] || [ -f "build.gradle.kts" ] || echo "$desc_lower" | grep -qE '\bgradle\b|\bkotlin\b'; then
        detected+=("gradle")
        if ! command -v gradle &>/dev/null && [ ! -f "gradlew" ]; then
            missing+=('{"tool":"gradle","reason":"Gradle build file found but gradle not available and no gradlew wrapper","install":"Install Gradle or include gradlew in the repo"}')
        fi
    fi

    # Java (Maven) — detect from files or task description
    if [ -f "pom.xml" ] || echo "$desc_lower" | grep -qE '\bmaven\b|\bjava\b|\bspring\b'; then
        detected+=("maven")
        if ! command -v mvn &>/dev/null && [ ! -f "mvnw" ] && ! command -v java &>/dev/null; then
            local reason="pom.xml found but mvn/java not available and no mvnw wrapper"
            if [ ! -f "pom.xml" ]; then
                reason="Task description references Java but java/mvn not available"
            fi
            missing+=("{\"tool\":\"java\",\"reason\":\"${reason}\",\"install\":\"Install Java/Maven or use a Java-based agent image\"}")
        fi
    fi

    # Ruby — detect from files or task description
    if [ -f "Gemfile" ] || echo "$desc_lower" | grep -qE '\bruby\b|\brails\b'; then
        detected+=("ruby")
        if ! command -v ruby &>/dev/null; then
            local reason="Gemfile found but ruby is not installed"
            if [ ! -f "Gemfile" ]; then
                reason="Task description references Ruby but ruby is not installed"
            fi
            missing+=("{\"tool\":\"ruby\",\"reason\":\"${reason}\",\"install\":\"Install Ruby or use a Ruby-based agent image\"}")
        fi
    fi

    # PHP — detect from files or task description
    if [ -f "composer.json" ] || echo "$desc_lower" | grep -qE '\bphp\b|\blaravel\b|\bsymfony\b'; then
        detected+=("php")
        if ! command -v php &>/dev/null; then
            local reason="composer.json found but php is not installed"
            if [ ! -f "composer.json" ]; then
                reason="Task description references PHP but php is not installed"
            fi
            missing+=("{\"tool\":\"php\",\"reason\":\"${reason}\",\"install\":\"Install PHP or use a PHP-based agent image\"}")
        fi
    fi

    # .NET — detect from files or task description
    if compgen -G "*.csproj" >/dev/null 2>&1 || compgen -G "*.fsproj" >/dev/null 2>&1 || compgen -G "*.sln" >/dev/null 2>&1 || echo "$desc_lower" | grep -qE '\b\.net\b|\bdotnet\b|\bcsharp\b|\bc#\b'; then
        detected+=("dotnet")
        if ! command -v dotnet &>/dev/null; then
            missing+=('{"tool":"dotnet","reason":".NET project detected but dotnet CLI not available","install":"Install .NET SDK or use a .NET-based agent image"}')
        fi
    fi

    # Swift — detect from files or task description
    if [ -f "Package.swift" ] || echo "$desc_lower" | grep -qE '\bswift\b|\bswiftui\b'; then
        detected+=("swift")
        if ! command -v swift &>/dev/null; then
            local reason="Package.swift found but swift is not installed"
            if [ ! -f "Package.swift" ]; then
                reason="Task description references Swift but swift is not installed"
            fi
            missing+=("{\"tool\":\"swift\",\"reason\":\"${reason}\",\"install\":\"Install Swift or use a Swift-based agent image\"}")
        fi
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        # Build JSON array of missing tools
        local missing_json="["
        local first=true
        for item in "${missing[@]}"; do
            if [ "$first" = true ]; then
                first=false
            else
                missing_json+=","
            fi
            missing_json+="$item"
        done
        missing_json+="]"

        # Build detected array
        local detected_json
        detected_json=$(printf '"%s",' "${detected[@]}")
        detected_json="[${detected_json%,}]"

        echo ""
        echo "[agent] PREREQUISITE CHECK FAILED"
        echo "[agent] Detected project types: ${detected[*]}"
        echo "[agent] Missing tools:"
        for item in "${missing[@]}"; do
            local tool
            tool=$(echo "$item" | jq -r '.tool')
            local reason
            reason=$(echo "$item" | jq -r '.reason')
            echo "[agent]   - ${tool}: ${reason}"
        done
        echo ""
        echo "VERVE_PREREQ_FAILED:{\"detected\":${detected_json},\"missing\":${missing_json}}"
        exit 1
    fi

    if [ ${#detected[@]} -gt 0 ]; then
        echo "[agent] Prerequisite check passed for: ${detected[*]}"
    else
        echo "[agent] No specific runtime requirements detected"
    fi
}

check_prereqs

if [ "$DRY_RUN" = "true" ]; then
    # Dry run mode - skip Claude, make a dummy change
    echo "[agent] DRY RUN mode - skipping Claude Code"
    echo ""

    cat > "verve-dry-run.md" <<DRYEOF
# Verve Dry Run

- **Task ID:** ${TASK_ID}
- **Description:** ${TASK_DESCRIPTION}
- **Attempt:** ${ATTEMPT:-1}
- **Timestamp:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")
DRYEOF

    echo "[agent] Created dummy file: verve-dry-run.md"

    # Commit and push
    echo "[agent] Committing changes..."
    git add -A
    git commit -m "dry-run: ${TASK_DESCRIPTION}

Implemented by Verve AI Agent (dry run)
Task ID: ${TASK_ID}"

    if [ "${ATTEMPT:-1}" -gt 1 ]; then
        echo "[agent] Pushing fixes to existing branch..."
        git push --force-with-lease origin "${BRANCH}"
    else
        echo "[agent] Pushing branch to origin..."
        git push -u origin "${BRANCH}"
    fi
    echo "[agent] Branch pushed successfully: ${BRANCH}"

    # Only create PR on first attempt
    if [ "${ATTEMPT:-1}" -le 1 ]; then
        echo "[agent] Creating pull request..."
        PR_TITLE="[Dry Run] ${TASK_DESCRIPTION}"
        PR_BODY="## Dry Run

This PR was created in dry-run mode (no Claude API calls).

**Task ID:** \`${TASK_ID}\`
**Description:** ${TASK_DESCRIPTION}

---
*Implemented by Verve AI Agent (dry run)*"

        PR_OUTPUT=$(gh pr create --title "${PR_TITLE}" --body "${PR_BODY}" --head "${BRANCH}" 2>&1)
        PR_EXIT_CODE=$?

        if [ ${PR_EXIT_CODE} -eq 0 ]; then
            PR_URL=$(echo "${PR_OUTPUT}" | grep -oE 'https://github.com/[^[:space:]]+/pull/[0-9]+' | head -1)
            if [ -n "${PR_URL}" ]; then
                PR_NUMBER=$(echo "${PR_URL}" | grep -oE '[0-9]+$')
                echo "[agent] Pull request created: ${PR_URL}"
                echo "VERVE_PR_CREATED:{\"url\":\"${PR_URL}\",\"number\":${PR_NUMBER}}"
            else
                echo "[agent] Pull request created but could not parse URL from: ${PR_OUTPUT}"
            fi
        else
            echo "[agent] Warning: Failed to create pull request: ${PR_OUTPUT}"
        fi
    else
        echo "[agent] Retry: pushed fixes to existing PR branch"
    fi

    echo ""
    echo "=== Task Completed Successfully (Dry Run) ==="
    echo "Branch: ${BRANCH}"
    exit 0
fi

# Build the prompt with retry context
PROMPT="${TASK_DESCRIPTION}"

if [ "${ATTEMPT:-1}" -gt 1 ]; then
    echo "[agent] Building retry-aware prompt..."

    PROMPT="IMPORTANT: This is retry attempt ${ATTEMPT}. The previous attempt created a PR but it needs fixes.
Reason for retry: ${RETRY_REASON}
"

    if echo "$RETRY_REASON" | grep -qi "ci_failure"; then
        PROMPT="${PROMPT}
Please examine the existing code changes on this branch, review the CI failure details below, and fix the issues. Do NOT create a new PR - just fix the code and commit to this branch."
    elif echo "$RETRY_REASON" | grep -qi "merge_conflict"; then
        PROMPT="${PROMPT}
The branch had merge conflicts with main. A rebase was attempted. Please resolve any remaining conflicts, ensure the code works correctly with the latest main branch, and commit. Do NOT create a new PR."
    fi

    # Include detailed CI failure logs if available
    if [ -n "$RETRY_CONTEXT" ]; then
        PROMPT="${PROMPT}

=== CI Failure Output ===
${RETRY_CONTEXT}
=== End CI Output ==="
    fi

    # Include previous iteration's status/notes if available
    if [ -n "$PREVIOUS_STATUS" ]; then
        PROMPT="${PROMPT}

=== Previous Iteration Notes ===
${PREVIOUS_STATUS}
=== End Notes ==="
    fi

    PROMPT="${PROMPT}

Original task: ${TASK_DESCRIPTION}"
fi

# Add acceptance criteria if provided
if [ -n "$ACCEPTANCE_CRITERIA" ]; then
    PROMPT="${PROMPT}

ACCEPTANCE CRITERIA (report which are met in your VERVE_STATUS output):
${ACCEPTANCE_CRITERIA}"
fi

# Add status output instruction
PROMPT="${PROMPT}

IMPORTANT: Before you finish, output a status line in this exact format on its own line:
VERVE_STATUS:{\"files_modified\":[],\"tests_status\":\"pass|fail|skip\",\"confidence\":\"high|medium|low\",\"blockers\":[],\"criteria_met\":[],\"notes\":\"Any context for future retry attempts\"}"

# Run Claude Code
echo "[agent] Starting Claude Code session..."
echo "[agent] Task: ${TASK_DESCRIPTION}"
echo ""

# Run Claude Code in non-interactive mode
# --output-format stream-json: Stream JSON events for real-time output
# --verbose: Required for stream-json, shows thinking
# --dangerously-skip-permissions: Skip permission prompts (we trust the agent)
# --model: Use specified model (defaults to haiku for cost efficiency)
CLAUDE_MODEL="${CLAUDE_MODEL:-haiku}"
echo "[agent] Using model: ${CLAUDE_MODEL}"

# Use stream-json for real-time output, parse with jq
# Events include: assistant (text/thinking), tool_use, tool_result, result
claude --output-format stream-json --verbose --dangerously-skip-permissions --model "${CLAUDE_MODEL}" "${PROMPT}" 2>&1 | while IFS= read -r line; do
    # Skip empty lines
    [ -z "$line" ] && continue

    # Try to parse as JSON
    if echo "$line" | jq -e . >/dev/null 2>&1; then
        TYPE=$(echo "$line" | jq -r '.type // empty' 2>/dev/null)

        case "$TYPE" in
            "assistant")
                # Extract content from assistant messages
                CONTENT_TYPE=$(echo "$line" | jq -r '.message.content[0].type // empty' 2>/dev/null)
                case "$CONTENT_TYPE" in
                    "thinking")
                        THINKING=$(echo "$line" | jq -r '.message.content[0].thinking // empty' 2>/dev/null)
                        if [ -n "$THINKING" ]; then
                            echo "[thinking] $THINKING"
                        fi
                        ;;
                    "text")
                        TEXT=$(echo "$line" | jq -r '.message.content[0].text // empty' 2>/dev/null)
                        if [ -n "$TEXT" ]; then
                            echo "[claude] $TEXT"
                        fi
                        ;;
                    "tool_use")
                        TOOL_NAME=$(echo "$line" | jq -r '.message.content[0].name // empty' 2>/dev/null)
                        if [ -n "$TOOL_NAME" ]; then
                            echo "[tool] Using: $TOOL_NAME"
                        fi
                        ;;
                esac
                ;;
            "result")
                # Final result
                RESULT_TEXT=$(echo "$line" | jq -r '.result // empty' 2>/dev/null)
                if [ -n "$RESULT_TEXT" ] && [ "$RESULT_TEXT" != "null" ]; then
                    echo "[result] $RESULT_TEXT"
                fi
                # Extract cost from result event
                COST=$(echo "$line" | jq -r '.total_cost_usd // empty' 2>/dev/null)
                if [ -n "$COST" ] && [ "$COST" != "null" ] && [ "$COST" != "0" ]; then
                    echo "VERVE_COST:${COST}"
                fi
                ;;
        esac
    else
        # Not JSON, print as-is (might be stderr or other output)
        echo "$line"
    fi
done

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

    if [ "${ATTEMPT:-1}" -gt 1 ]; then
        echo "[agent] Pushing fixes to existing branch..."
        git push --force-with-lease origin "${BRANCH}"
    else
        echo "[agent] Pushing branch to origin..."
        git push -u origin "${BRANCH}"
    fi
    echo "[agent] Branch pushed successfully: ${BRANCH}"

    # Only create PR on first attempt
    if [ "${ATTEMPT:-1}" -le 1 ]; then
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
        PR_RAW=$(claude --print --model "${CLAUDE_MODEL}" "${PR_PROMPT}" 2>/dev/null || echo "")

        # Extract JSON from response (may be wrapped in markdown code blocks)
        # First try to extract JSON from code blocks, then try raw response
        PR_JSON=""
        if [ -n "${PR_RAW}" ]; then
            # Try to extract JSON from ```json ... ``` or ``` ... ``` blocks
            PR_JSON=$(echo "${PR_RAW}" | sed -n '/^```/,/^```$/p' | sed '1d;$d' | tr -d '\n' || echo "")
            # If that didn't work, try to find raw JSON object
            if [ -z "${PR_JSON}" ] || ! echo "${PR_JSON}" | jq -e . >/dev/null 2>&1; then
                PR_JSON=$(echo "${PR_RAW}" | grep -o '{[^}]*}' | head -1 || echo "")
            fi
            # Last resort: use raw output if it's valid JSON
            if [ -z "${PR_JSON}" ] || ! echo "${PR_JSON}" | jq -e . >/dev/null 2>&1; then
                if echo "${PR_RAW}" | jq -e . >/dev/null 2>&1; then
                    PR_JSON="${PR_RAW}"
                fi
            fi
        fi

        # Extract title and description, with fallbacks
        PR_TITLE=""
        PR_BODY=""
        if [ -n "${PR_JSON}" ] && echo "${PR_JSON}" | jq -e . >/dev/null 2>&1; then
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
*Implemented by Verve AI Agent*
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
    else
        echo "[agent] Retry: pushed fixes to existing PR branch"
    fi
fi

echo ""
echo "=== Task Completed Successfully ==="
echo "Branch: ${BRANCH}"
