#!/bin/bash
# epic_plan.sh — Epic planning agent that runs as a long-lived container.
# Clones the repo for context, runs Claude to generate proposed tasks,
# submits them to the API, and enters a feedback loop waiting for user
# input or confirmation.

# Depends on: log.sh, validate.sh, git.sh, claude.sh (sourced by entrypoint.sh)

IDLE_TIMEOUT=900  # 15 minutes in seconds
POLL_URL="${API_URL}/api/v1/agent/epics/${EPIC_ID}"

run_epic_planning() {
    log_header "Verve Epic Planning Agent Starting"
    echo "Epic ID: ${EPIC_ID}"
    echo "Repository: ${GITHUB_REPO}"
    echo "Title: ${EPIC_TITLE}"
    echo "Description: ${EPIC_DESCRIPTION}"
    [ -n "${EPIC_PLANNING_PROMPT}" ] && echo "Planning Prompt: ${EPIC_PLANNING_PROMPT}"
    log_blank

    # Validate required env vars
    if [ -z "$EPIC_ID" ] || [ -z "$API_URL" ]; then
        log_error "EPIC_ID and API_URL are required for epic planning"
        exit 1
    fi

    if [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$CLAUDE_CODE_OAUTH_TOKEN" ]; then
        log_error "ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN must be set"
        exit 1
    fi

    # Clone repo for context (read-only)
    configure_git
    clone_repo
    log_agent "Repository cloned for context analysis"
    log_blank

    # Initial planning
    _epic_send_log "system: Planning session started. Analyzing epic and generating task breakdown..."
    _epic_run_planning ""

    # Enter feedback loop
    _epic_feedback_loop
}

_epic_run_planning() {
    local feedback="$1"

    local prompt
    prompt=$(_build_epic_prompt "$feedback")

    log_agent "Running Claude for epic planning..."
    local output
    output=$(claude --output-format json --verbose --dangerously-skip-permissions \
        --model "sonnet" "$prompt" 2>&1 | _extract_claude_result)

    if [ -z "$output" ]; then
        log_error "Claude produced no output"
        _epic_send_log "system: Planning failed — Claude produced no output"
        return 1
    fi

    # Parse proposed tasks from Claude's output
    local tasks_json
    tasks_json=$(_extract_proposed_tasks "$output")

    if [ -z "$tasks_json" ] || [ "$tasks_json" = "null" ] || [ "$tasks_json" = "[]" ]; then
        log_error "Could not extract proposed tasks from Claude output"
        _epic_send_log "system: Planning produced no tasks. The agent's response has been logged."
        return 1
    fi

    local task_count
    task_count=$(echo "$tasks_json" | jq 'length')
    log_agent "Generated ${task_count} proposed tasks"

    # Submit proposed tasks to server
    _epic_propose_tasks "$tasks_json"
    _epic_send_log "system: Planning complete. Proposed ${task_count} tasks."
}

_build_epic_prompt() {
    local feedback="$1"

    local prompt="You are a technical project planner. Your job is to analyze a software epic and break it down into concrete, actionable implementation tasks.

## Epic Details

**Title:** ${EPIC_TITLE}

**Description:**
${EPIC_DESCRIPTION}"

    if [ -n "${EPIC_PLANNING_PROMPT}" ]; then
        prompt="${prompt}

**Additional Planning Instructions:**
${EPIC_PLANNING_PROMPT}"
    fi

    if [ -n "$feedback" ]; then
        prompt="${prompt}

**User Feedback on Previous Plan:**
${feedback}

Please revise the task breakdown based on this feedback."
    fi

    prompt="${prompt}

## Repository Context

You have access to the repository at $(pwd). Use the tools available to explore the codebase structure, read key files, and understand the architecture before creating your plan.

## Output Requirements

After analyzing the codebase and the epic, output a JSON array of proposed tasks. Each task should have:
- \`temp_id\`: A unique identifier like \"task_1\", \"task_2\", etc.
- \`title\`: A concise, actionable title (imperative form, e.g. \"Add user authentication middleware\")
- \`description\`: Detailed description of what needs to be done, including relevant file paths and implementation details
- \`depends_on_temp_ids\`: Array of temp_ids this task depends on (empty array if none)
- \`acceptance_criteria\`: Array of specific, testable criteria for completion

Output the tasks as a JSON array wrapped in a markdown code block with the language tag \`verve-tasks\`. Example:

\`\`\`verve-tasks
[
  {
    \"temp_id\": \"task_1\",
    \"title\": \"Create database migration for users table\",
    \"description\": \"Add a new migration file...\",
    \"depends_on_temp_ids\": [],
    \"acceptance_criteria\": [\"Migration creates users table with required columns\", \"Migration is reversible\"]
  },
  {
    \"temp_id\": \"task_2\",
    \"title\": \"Implement user model and repository\",
    \"description\": \"Create the user domain model...\",
    \"depends_on_temp_ids\": [\"task_1\"],
    \"acceptance_criteria\": [\"User CRUD operations work\", \"Input validation implemented\"]
  }
]
\`\`\`

Order tasks by dependency (tasks with no dependencies first). Be thorough but practical — each task should be independently completable by an AI coding agent."

    echo "$prompt"
}

_extract_claude_result() {
    # Read Claude JSON output and extract the result text
    local last_result=""
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        local event_type
        event_type=$(echo "$line" | jq -r '.type // empty' 2>/dev/null)
        if [ "$event_type" = "result" ]; then
            last_result=$(echo "$line" | jq -r '.result // empty' 2>/dev/null)
        fi
    done
    echo "$last_result"
}

_extract_proposed_tasks() {
    local output="$1"

    # Try to extract from ```verve-tasks code block
    local tasks
    tasks=$(echo "$output" | sed -n '/```verve-tasks/,/```/p' | sed '1d;$d')

    if [ -n "$tasks" ]; then
        echo "$tasks" | jq '.' 2>/dev/null && return
    fi

    # Fallback: try to extract any JSON array from the output
    tasks=$(echo "$output" | grep -o '\[.*\]' | jq '.' 2>/dev/null)
    if [ -n "$tasks" ]; then
        echo "$tasks"
        return
    fi

    echo "[]"
}

_epic_propose_tasks() {
    local tasks_json="$1"

    local body
    body=$(jq -n --argjson tasks "$tasks_json" '{"tasks": $tasks}')

    local response
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${POLL_URL}/propose")

    local http_code
    http_code=$(echo "$response" | tail -1)

    if [ "$http_code" != "200" ]; then
        log_error "Failed to submit proposed tasks (HTTP ${http_code})"
    else
        log_agent "Proposed tasks submitted successfully"
    fi
}

_epic_send_log() {
    local message="$1"
    local body
    body=$(jq -n --arg line "$message" '{"lines": [$line]}')

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${POLL_URL}/logs" > /dev/null 2>&1 || true
}

_epic_feedback_loop() {
    local last_activity
    last_activity=$(date +%s)

    log_agent "Entering feedback loop — waiting for user input..."

    while true; do
        local now
        now=$(date +%s)
        local elapsed=$(( now - last_activity ))

        # Check idle timeout
        if [ "$elapsed" -ge "$IDLE_TIMEOUT" ]; then
            log_agent "Idle timeout reached (${IDLE_TIMEOUT}s). Releasing agent."
            _epic_send_log "system: Planning session timed out due to inactivity."
            exit 0
        fi

        # Long-poll for feedback
        local response
        response=$(curl -s -w "\n%{http_code}" \
            "${POLL_URL}/poll-feedback" 2>/dev/null)

        local http_code
        http_code=$(echo "$response" | tail -1)
        local body
        body=$(echo "$response" | sed '$d')

        if [ "$http_code" != "200" ]; then
            log_error "Failed to poll feedback (HTTP ${http_code})"
            sleep 5
            continue
        fi

        local feedback_type
        feedback_type=$(echo "$body" | jq -r '.type // "timeout"' 2>/dev/null)

        case "$feedback_type" in
            feedback)
                local feedback_text
                feedback_text=$(echo "$body" | jq -r '.feedback // ""' 2>/dev/null)
                log_agent "Received feedback: ${feedback_text}"
                _epic_send_log "system: Re-planning based on user feedback..."
                last_activity=$(date +%s)
                _epic_run_planning "$feedback_text"
                ;;
            confirmed)
                log_agent "Epic confirmed by user. Exiting."
                _epic_send_log "system: Epic confirmed. Agent exiting."
                exit 0
                ;;
            closed)
                log_agent "Epic closed by user. Exiting."
                _epic_send_log "system: Epic closed. Agent exiting."
                exit 0
                ;;
            timeout)
                # Long-poll timeout, just loop again
                ;;
            *)
                log_agent "Unknown feedback type: ${feedback_type}"
                sleep 5
                ;;
        esac
    done
}
