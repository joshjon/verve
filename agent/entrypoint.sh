#!/bin/sh
set -e

echo "=== Verve Agent Starting ==="
echo "Task ID: ${TASK_ID}"
echo "Description: ${TASK_DESCRIPTION}"
echo ""

echo "[1/5] Initializing workspace..."
sleep 3

echo "[2/5] Analyzing task requirements..."
echo "  - Parsing task description"
sleep 2
echo "  - Identifying affected files"
sleep 2

echo "[3/5] Executing task: ${TASK_DESCRIPTION}"
echo "  - Making code changes"
sleep 3
echo "  - Running linter"
sleep 2

echo "[4/5] Validating results..."
echo "  - Running tests"
sleep 3
echo "  - All tests passed"

echo "[5/5] Cleaning up..."
sleep 1

echo ""
echo "=== Task Completed Successfully ==="
exit 0
