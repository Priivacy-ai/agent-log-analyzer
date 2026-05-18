#!/usr/bin/env sh
set -eu

DATA_DIR="$(pwd)/.data/native-smoke"
API_LOG="$(pwd)/.data/native-api.log"
WORKER_LOG="$(pwd)/.data/native-worker.log"
rm -rf "$DATA_DIR"
mkdir -p "$(pwd)/.data"

cleanup() {
  [ -n "${API_PID:-}" ] && kill "$API_PID" >/dev/null 2>&1 || true
  [ -n "${WORKER_PID:-}" ] && kill "$WORKER_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

CLAUDE_ANALYZER_DATA_DIR="$DATA_DIR" CLAUDE_ANALYZER_ADDR=:18080 go run ./cmd/api >"$API_LOG" 2>&1 &
API_PID=$!
CLAUDE_ANALYZER_DATA_DIR="$DATA_DIR" CLAUDE_ANALYZER_WORKER_INTERVAL=250ms go run ./cmd/worker >"$WORKER_LOG" 2>&1 &
WORKER_PID=$!

for _ in $(seq 1 40); do
  if curl -fsS http://127.0.0.1:18080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep .25
done

JOB_ID=$(
  curl -fsS \
    -F "log=@testdata/fixtures/sample-claude.jsonl" \
    http://127.0.0.1:18080/api/jobs |
    sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p'
)

for _ in $(seq 1 40); do
  STATUS=$(curl -fsS "http://127.0.0.1:18080/api/jobs/$JOB_ID" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && exit 1
  sleep .25
done

REPORT=$(curl -fsS "http://127.0.0.1:18080/api/reports/$JOB_ID")
echo "$REPORT" | grep -q '"raw_transcript_sent_to_llm":false'
echo "$REPORT" | grep -q '"spec_kitty"'
if echo "$REPORT" | grep -q 'sk-ant-'; then
  echo "secret leaked in report"
  exit 1
fi

echo "native smoke ok: $JOB_ID"

