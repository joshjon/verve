#!/bin/bash
# log.sh — Structured logging helpers for Verve Agent

log_agent()  { echo "[agent] $*"; }
log_error()  { echo "[error] $*"; }
log_tool()   { echo "[tool] $*"; }
log_claude() { echo "[claude] $*"; }
log_think()  { echo "[thinking] $*"; }
log_result() { echo "[result] $*"; }

log_header() {
    echo "=== $* ==="
}

log_blank() {
    echo ""
}
