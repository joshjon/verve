#!/bin/bash
# dryrun.sh — Dry run mode: skip Claude, make a dummy change, push, and optionally create a PR

# Depends on: log.sh, github.sh (sourced by entrypoint.sh)

run_dry_run() {
    log_agent "DRY RUN mode - skipping Claude Code"
    log_blank

    cat > "verve-dry-run.md" <<DRYEOF
# Verve Dry Run

- **Task ID:** ${TASK_ID}
- **Description:** ${TASK_DESCRIPTION}
- **Attempt:** ${ATTEMPT:-1}
- **Timestamp:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")
DRYEOF

    log_agent "Created dummy file: verve-dry-run.md"

    # Commit and push
    log_agent "Committing changes..."
    git add -A
    local commit_title="${TASK_TITLE:-${TASK_DESCRIPTION}}"
    git commit -m "dry-run: ${commit_title}"

    if [ "${ATTEMPT:-1}" -gt 1 ]; then
        log_agent "Pushing fixes to existing branch..."
        git push --force-with-lease origin "${BRANCH}"
    else
        log_agent "Pushing branch to origin..."
        git push -u origin "${BRANCH}"
    fi
    log_agent "Branch pushed successfully: ${BRANCH}"

    # PR handling
    if [ "$SKIP_PR" = "true" ]; then
        log_agent "Skip PR mode: branch pushed, skipping PR creation"
        echo "VERVE_BRANCH_PUSHED:{\"branch\":\"${BRANCH}\"}"
    elif [ "${ATTEMPT:-1}" -le 1 ]; then
        log_agent "Creating pull request..."
        local pr_title="[Dry Run] ${TASK_TITLE:-${TASK_DESCRIPTION}}"
        local pr_body="## Dry Run

This PR was created in dry-run mode (no Claude API calls).

**Description:** ${TASK_DESCRIPTION}"

        if ! create_pr "${pr_title}" "${pr_body}" "${BRANCH}" "${DEFAULT_BRANCH}"; then
            log_agent "PR creation failed"
            exit 1
        fi
    else
        log_agent "Retry: pushed fixes to existing PR branch"
    fi

    log_blank
    log_header "Task Completed Successfully (Dry Run)"
    echo "Branch: ${BRANCH}"
}
