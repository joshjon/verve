#!/bin/bash
# git.sh — Git configuration, cloning, and branch management

# Depends on: log.sh (sourced by entrypoint.sh)

configure_git() {
    log_agent "Configuring git..."
    git config --global credential.helper store
    echo "https://${GITHUB_TOKEN}@github.com" > /home/agent/.git-credentials
    git config --global user.name "Verve Agent"
    git config --global user.email ""
}

clone_repo() {
    log_agent "Cloning repository: ${GITHUB_REPO}..."
    git clone "https://${GITHUB_TOKEN}@github.com/${GITHUB_REPO}.git" /workspace/repo
    cd /workspace/repo || exit 1
}

detect_default_branch() {
    DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@' || echo "main")
    log_agent "Default branch: ${DEFAULT_BRANCH}"
}

setup_branch() {
    BRANCH="verve/task-${TASK_ID}"
    # Track whether the branch already existed on the remote. Used later to
    # decide between force-push vs first push, and whether a PR still needs
    # to be created.
    BRANCH_EXISTS_ON_REMOTE=false

    if [ "${ATTEMPT:-1}" -gt 1 ]; then
        log_agent "Retry attempt ${ATTEMPT}: checking out existing branch ${BRANCH}"
        if git fetch origin "${BRANCH}" 2>/dev/null; then
            BRANCH_EXISTS_ON_REMOTE=true
            git checkout "${BRANCH}"

            if echo "$RETRY_REASON" | grep -qi "merge conflict"; then
                log_agent "Rebasing on ${DEFAULT_BRANCH} to resolve merge conflicts..."
                git fetch origin "${DEFAULT_BRANCH}"
                # Don't fail if rebase has conflicts — Claude will resolve them
                git rebase "origin/${DEFAULT_BRANCH}" || true
            fi
        else
            log_agent "Branch ${BRANCH} not found on remote (previous attempt may have failed before pushing)"
            log_agent "Creating branch: ${BRANCH}"
            git checkout -b "${BRANCH}"
        fi
    else
        log_agent "Creating branch: ${BRANCH}"
        git checkout -b "${BRANCH}"
    fi
}

commit_and_push() {
    log_agent "Checking for changes..."
    git add -A

    if ! git diff --cached --quiet; then
        log_agent "Committing changes..."
        local commit_title="${TASK_TITLE:-${TASK_DESCRIPTION}}"
        git commit -m "${commit_title}"
    else
        log_agent "No new changes to commit"
    fi

    # Check for any commits ahead of the default branch
    local changes
    changes=$(git log "origin/${DEFAULT_BRANCH}..HEAD" --oneline 2>/dev/null)
    if [ -z "$changes" ]; then
        log_agent "No changes were made — nothing to push or PR"
        echo 'VERVE_STATUS:{"files_modified":[],"tests_status":"skip","confidence":"low","blockers":["No changes were made"],"criteria_met":[],"notes":"Agent did not produce any code changes"}'
        exit 1
    fi

    if [ "${BRANCH_EXISTS_ON_REMOTE}" = "true" ]; then
        log_agent "Pushing fixes to existing branch..."
        git push --force-with-lease origin "${BRANCH}"
    else
        log_agent "Pushing branch to origin..."
        if ! git push -u origin "${BRANCH}" 2>&1; then
            log_agent "Push was rejected (branch may already exist on remote), retrying with --force-with-lease..."
            git push --force-with-lease -u origin "${BRANCH}"
        fi
    fi
    log_agent "Branch pushed successfully: ${BRANCH}"
}
