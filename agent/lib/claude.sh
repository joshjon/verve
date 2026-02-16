#!/bin/bash
# claude.sh — Run Claude Code and parse streaming JSON output

# Depends on: log.sh (sourced by entrypoint.sh)

run_claude() {
    local prompt="$1"

    CLAUDE_MODEL="${CLAUDE_MODEL:-sonnet}"

    log_agent "Starting Claude Code session..."
    if [ -n "${TASK_TITLE}" ]; then
        log_agent "Task: ${TASK_TITLE}"
    fi
    log_agent "Description: ${TASK_DESCRIPTION}"
    log_blank
    log_agent "Using model: ${CLAUDE_MODEL}"

    claude --output-format stream-json --verbose --dangerously-skip-permissions \
        --model "${CLAUDE_MODEL}" "${prompt}" 2>&1 | _parse_stream

    log_blank
    log_agent "Claude Code session completed"
}

_parse_stream() {
    while IFS= read -r line; do
        [ -z "$line" ] && continue

        if ! echo "$line" | jq -e . >/dev/null 2>&1; then
            echo "$line"
            continue
        fi

        local event_type
        event_type=$(echo "$line" | jq -r '.type // empty' 2>/dev/null)

        case "$event_type" in
            assistant) _handle_assistant_event "$line" ;;
            result)    _handle_result_event "$line" ;;
        esac
    done
}

_handle_assistant_event() {
    local line="$1"
    local content_type
    content_type=$(echo "$line" | jq -r '.message.content[0].type // empty' 2>/dev/null)

    case "$content_type" in
        thinking)
            local text
            text=$(echo "$line" | jq -r '.message.content[0].thinking // empty' 2>/dev/null)
            [ -n "$text" ] && log_think "$text"
            ;;
        text)
            local text
            text=$(echo "$line" | jq -r '.message.content[0].text // empty' 2>/dev/null)
            [ -n "$text" ] && log_claude "$text"
            ;;
        tool_use)
            local name
            name=$(echo "$line" | jq -r '.message.content[0].name // empty' 2>/dev/null)
            [ -n "$name" ] && log_tool "Using: $name"
            ;;
    esac
}

_handle_result_event() {
    local line="$1"
    local text
    text=$(echo "$line" | jq -r '.result // empty' 2>/dev/null)
    if [ -n "$text" ] && [ "$text" != "null" ]; then
        log_result "$text"
    fi

    local cost
    cost=$(echo "$line" | jq -r '.total_cost_usd // empty' 2>/dev/null)
    if [ -n "$cost" ] && [ "$cost" != "null" ] && [ "$cost" != "0" ]; then
        echo "VERVE_COST:${cost}"
    fi
}
