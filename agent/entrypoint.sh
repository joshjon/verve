#!/bin/sh
set -e

echo "=== Verve Agent Starting ==="
echo "Task ID: ${TASK_ID}"
echo "Description: ${TASK_DESCRIPTION}"
echo ""

echo "[1/5] Initializing workspace..."
sleep 1

echo "[2/5] Analyzing task requirements..."
sleep 1

echo "[3/5] Executing task: ${TASK_DESCRIPTION}"
sleep 2

echo "[4/5] Validating results..."
sleep 1

echo "[5/5] Cleaning up..."
sleep 1

echo ""
echo "=== Task Completed Successfully ==="
exit 0
