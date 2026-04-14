#!/usr/bin/env bash
set -euo pipefail

APP_PORT="${APP_PORT:-18080}"
BASE_URL="http://localhost:${APP_PORT}"

cleanup() {
  docker compose down -v --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT

wait_for_http() {
  local url="$1"
  local attempts="${2:-30}"

  for ((i=1; i<=attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "Service did not become ready: $url" >&2
  return 1
}

wait_for_completed_task() {
  local task_id="$1"

  for ((i=1; i<=30; i++)); do
    local status_response
    status_response=$(curl -fsS "${BASE_URL}/api/tasks/${task_id}")

    if printf '%s' "$status_response" | grep -q '"status":"completed"'; then
      return 0
    fi

    if printf '%s' "$status_response" | grep -q '"status":"failed"'; then
      echo "Task failed: $status_response" >&2
      return 1
    fi

    sleep 2
  done

  echo "Task did not complete in time" >&2
  return 1
}

echo "Cleaning previous application stack"
docker compose down -v --remove-orphans >/dev/null 2>&1 || true

echo "Starting application stack on port ${APP_PORT}"
APP_PORT="${APP_PORT}" docker compose up -d --build

echo "Waiting for gateway health endpoint"
wait_for_http "${BASE_URL}/healthz"

echo "Creating QR generation task"
create_response=$(curl -fsS -X POST "${BASE_URL}/api/tasks" \
  -H 'Content-Type: application/json' \
  -d '{"content":"https://github.com/danluki/qrgen","size":256}')

task_id=$(printf '%s' "$create_response" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
if [[ -z "$task_id" ]]; then
  echo "Failed to extract task id from response: $create_response" >&2
  exit 1
fi

echo "Waiting for task ${task_id} to complete"
wait_for_completed_task "$task_id"

echo "Downloading generated PNG"
curl -fsS "${BASE_URL}/api/tasks/${task_id}/image" -o /tmp/qr-code.png

if [[ ! -s /tmp/qr-code.png ]]; then
  echo "Generated PNG is empty" >&2
  exit 1
fi

echo "Smoke test passed"
