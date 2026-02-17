#!/bin/bash
set -e

# ── Load libraries ──────────────────────────────────────────────────
LIB_DIR="$(dirname "$0")/lib"
source "${LIB_DIR}/log.sh"
source "${LIB_DIR}/validate.sh"
source "${LIB_DIR}/git.sh"
source "${LIB_DIR}/prereqs.sh"
source "${LIB_DIR}/github.sh"
source "${LIB_DIR}/prompt.sh"
source "${LIB_DIR}/claude.sh"
source "${LIB_DIR}/dryrun.sh"

# ── Banner ──────────────────────────────────────────────────────────
log_header "Verve Agent Starting"
echo "Task ID: ${TASK_ID}"
echo "Repository: ${GITHUB_REPO}"
[ -n "${TASK_TITLE}" ] && echo "Title: ${TASK_TITLE}"
echo "Description: ${TASK_DESCRIPTION}"
if [ "${ATTEMPT:-1}" -gt 1 ]; then
    echo "Attempt: ${ATTEMPT} (retry)"
    echo "Retry Reason: ${RETRY_REASON}"
fi
log_blank

# ── Setup ───────────────────────────────────────────────────────────
validate_env
configure_git
clone_repo
detect_default_branch
setup_branch

# ── Prerequisites ───────────────────────────────────────────────────
check_prereqs

# ── Dry run shortcut ────────────────────────────────────────────────
if [ "$DRY_RUN" = "true" ]; then
    run_dry_run
    exit 0
fi

# ── Run Claude Code ─────────────────────────────────────────────────
if [ "${ATTEMPT:-1}" -gt 1 ]; then
    log_agent "Building retry-aware prompt..."
fi
build_prompt
run_claude "$PROMPT"

# ── Commit, push, and create PR ────────────────────────────────────
commit_and_push

if [ "$SKIP_PR" = "true" ]; then
    log_agent "Skip PR mode: branch pushed, skipping PR creation"
    echo "VERVE_BRANCH_PUSHED:{\"branch\":\"${BRANCH}\"}"
elif [ "${ATTEMPT:-1}" -le 1 ] || [ "${BRANCH_EXISTS_ON_REMOTE}" != "true" ]; then
    generate_and_create_pr "${BRANCH}" "${DEFAULT_BRANCH}"
else
    log_agent "Retry: pushed fixes to existing PR branch"
fi

log_blank
log_header "Task Completed Successfully"
echo "Branch: ${BRANCH}"
