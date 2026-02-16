#!/bin/bash
# validate.sh — Validate required environment variables

# Depends on: log.sh (sourced by entrypoint.sh)

validate_env() {
    if [ -z "$GITHUB_TOKEN" ]; then
        log_error "GITHUB_TOKEN is not set"
        exit 1
    fi

    if [ -z "$GITHUB_REPO" ]; then
        log_error "GITHUB_REPO is not set"
        exit 1
    fi

    if [ "$DRY_RUN" != "true" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$CLAUDE_CODE_OAUTH_TOKEN" ]; then
        log_error "ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN must be set"
        exit 1
    fi
}
