#!/usr/bin/env bash
set -euo pipefail

PROJECT="${COMPOSE_PROJECT_NAME:-claude-log-analyzer-aws}"
export COMPOSE_PROJECT_NAME="$PROJECT"
COMPOSE=(docker compose -p "$PROJECT" -f docker-compose.aws.yml)

cleanup() {
  rc=$?
  if [ "$rc" -ne 0 ]; then
    "${COMPOSE[@]}" ps || true
    "${COMPOSE[@]}" logs --no-color || true
  fi
  "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
  exit "$rc"
}
trap cleanup EXIT

./scripts/init-localstack.sh
"${COMPOSE[@]}" up --build -d api worker

for _ in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:8081/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

CLAUDE_ANALYZER_URL=http://127.0.0.1:8081 ./scripts/load-local.sh 3
CLAUDE_ANALYZER_URL=http://127.0.0.1:8081 \
  CLAUDE_ANALYZER_UPLOAD_URL_REWRITE_FROM=http://localstack:4566 \
  CLAUDE_ANALYZER_UPLOAD_URL_REWRITE_TO=http://127.0.0.1:4566 \
  go run ./cmd/direct-upload-smoke
echo "aws local smoke ok"
