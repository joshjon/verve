#!/bin/bash
# github.sh — GitHub API helpers (PR creation)

# Depends on: log.sh (sourced by entrypoint.sh)

# Check whether an open pull request already exists for a given head branch.
# Returns 0 (true) if a PR exists, 1 (false) otherwise.
# Sets PR_URL to the HTML URL of the existing PR when found.
# Usage: pr_exists_for_branch <head_branch>
pr_exists_for_branch() {
    local head="$1"

    local response
    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/${GITHUB_REPO}/pulls?head=${GITHUB_REPO%%/*}:${head}&state=open")

    local http_code response_body
    http_code=$(echo "$response" | tail -1)
    response_body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "200" ]; then
        local count
        count=$(echo "$response_body" | jq 'length' 2>/dev/null || echo "0")
        if [ "$count" -gt 0 ]; then
            PR_URL=$(echo "$response_body" | jq -r '.[0].html_url // empty' 2>/dev/null || echo "")
            return 0
        fi
    fi
    return 1
}

# Create a pull request via the GitHub API.
# Usage: create_pr <title> <body> <head_branch> <base_branch>
create_pr() {
    local title="$1" body="$2" head="$3" base="$4"

    local json_title json_body
    json_title=$(printf '%s' "$title" | jq -Rs .)
    json_body=$(printf '%s' "$body" | jq -Rs .)

    local response
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/${GITHUB_REPO}/pulls" \
        -d "{\"title\":${json_title},\"body\":${json_body},\"head\":\"${head}\",\"base\":\"${base}\"}")

    local http_code response_body
    http_code=$(echo "$response" | tail -1)
    response_body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "201" ]; then
        local pr_url pr_number
        pr_url=$(echo "$response_body" | jq -r '.html_url // empty')
        pr_number=$(echo "$response_body" | jq -r '.number // empty')
        if [ -n "$pr_url" ] && [ -n "$pr_number" ]; then
            log_agent "Pull request created: ${pr_url}"
            echo "VERVE_PR_CREATED:{\"url\":\"${pr_url}\",\"number\":${pr_number}}"
        else
            log_agent "Pull request created but could not parse response"
        fi
        return 0
    else
        local error_msg errors_detail
        error_msg=$(echo "$response_body" | jq -r '.message // empty' 2>/dev/null || echo "unknown error")
        errors_detail=$(echo "$response_body" | jq -r '.errors[]?.message // empty' 2>/dev/null || echo "")
        log_agent "Failed to create pull request (HTTP ${http_code}): ${error_msg}"
        if [ -n "$errors_detail" ]; then
            log_agent "Details: ${errors_detail}"
        fi
        return 1
    fi
}

# Generate PR title and description using Claude, then create the PR.
# Usage: generate_and_create_pr <branch> <default_branch>
generate_and_create_pr() {
    local branch="$1" default_branch="$2"

    log_agent "Creating pull request..."

    local diff_summary
    diff_summary=$(git diff "origin/${default_branch}...HEAD" --stat 2>/dev/null | tail -20 || echo "Changes made by Verve Agent")

    local pr_prompt="Generate a pull request title and description for the following task and changes.

Task Title: ${TASK_TITLE:-${TASK_DESCRIPTION}}
Task Description: ${TASK_DESCRIPTION}

Files changed:
${diff_summary}

Respond with ONLY valid JSON in this exact format (no markdown, no code blocks, no extra text):
{\"title\": \"Short descriptive title (max 72 chars)\", \"description\": \"## Summary\\n\\nBrief description of changes.\\n\\n## Changes\\n\\n- Bullet points of what was done\"}"

    log_agent "Generating PR description with Claude..."
    local model="${CLAUDE_MODEL:-sonnet}"
    local pr_raw
    pr_raw=$(claude --print --model "${model}" "${pr_prompt}" 2>/dev/null || echo "")

    local pr_json=""
    if [ -n "${pr_raw}" ]; then
        # Try to extract JSON from ```json ... ``` or ``` ... ``` blocks
        pr_json=$(echo "${pr_raw}" | sed -n '/^```/,/^```$/p' | sed '1d;$d' | tr -d '\n' || echo "")
        # If that didn't work, try to find raw JSON object
        if [ -z "${pr_json}" ] || ! echo "${pr_json}" | jq -e . >/dev/null 2>&1; then
            pr_json=$(echo "${pr_raw}" | grep -o '{[^}]*}' | head -1 || echo "")
        fi
        # Last resort: use raw output if it's valid JSON
        if [ -z "${pr_json}" ] || ! echo "${pr_json}" | jq -e . >/dev/null 2>&1; then
            if echo "${pr_raw}" | jq -e . >/dev/null 2>&1; then
                pr_json="${pr_raw}"
            fi
        fi
    fi

    local pr_title="" pr_body=""
    if [ -n "${pr_json}" ] && echo "${pr_json}" | jq -e . >/dev/null 2>&1; then
        pr_title=$(echo "${pr_json}" | jq -r '.title // empty' 2>/dev/null || echo "")
        pr_body=$(echo "${pr_json}" | jq -r '.description // empty' 2>/dev/null || echo "")
    fi

    # Fallbacks
    if [ -z "${pr_title}" ]; then
        pr_title="${TASK_TITLE:-${TASK_DESCRIPTION}}"
    fi
    if [ -z "${pr_body}" ]; then
        pr_body="## Summary

Automated implementation of: ${TASK_DESCRIPTION}

## Changes

${diff_summary}"
    fi

    if ! create_pr "${pr_title}" "${pr_body}" "${branch}" "${default_branch}"; then
        log_agent "PR creation failed"
        exit 1
    fi
}
