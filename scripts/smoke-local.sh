#!/usr/bin/env sh
set -eu

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-claude-log-analyzer-smoke}"
export COMPOSE_PROJECT_NAME

cleanup() {
  docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker compose up --build -d

for _ in $(seq 1 60); do
  if wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

JOB_ID=$(
  curl -fsS \
    -F "log=@testdata/fixtures/sample-claude.jsonl" \
    http://127.0.0.1:8080/api/jobs |
    sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p'
)

if [ -z "$JOB_ID" ]; then
  echo "failed to create job"
  exit 1
fi

for _ in $(seq 1 60); do
  STATUS=$(curl -fsS "http://127.0.0.1:8080/api/jobs/$JOB_ID" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
  if [ "$STATUS" = "completed" ]; then
    break
  fi
  if [ "$STATUS" = "failed" ]; then
    curl -fsS "http://127.0.0.1:8080/api/jobs/$JOB_ID"
    exit 1
  fi
  sleep 1
done

REPORT=$(curl -fsS "http://127.0.0.1:8080/api/reports/$JOB_ID")
echo "$REPORT" | grep -q '"raw_transcript_sent_to_llm":false'
echo "$REPORT" | grep -q '"spec_kitty"'
if echo "$REPORT" | grep -q 'sk-ant-'; then
  echo "secret leaked in report"
  exit 1
fi

echo "smoke ok: $JOB_ID"

