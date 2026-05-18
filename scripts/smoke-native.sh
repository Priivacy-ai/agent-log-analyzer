#!/usr/bin/env sh
set -eu

DATA_DIR="$(pwd)/.data/native-smoke"
API_LOG="$(pwd)/.data/native-api.log"
WORKER_LOG="$(pwd)/.data/native-worker.log"
FIXTURE="${CLAUDE_ANALYZER_FIXTURE:-testdata/fixtures/sample-claude.jsonl}"
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

SESSION=$(curl -fsS -X POST http://127.0.0.1:18080/api/analysis-sessions)
JOB_ID=$(echo "$SESSION" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')
TOKEN=$(echo "$SESSION" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
REPORT_PATH=$(echo "$SESSION" | sed -n 's/.*"report_path":"\([^"]*\)".*/\1/p')

if [ -z "$JOB_ID" ] || [ -z "$TOKEN" ] || [ -z "$REPORT_PATH" ]; then
  echo "failed to create analysis session"
  exit 1
fi

curl -fsS \
  -X PUT \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary "@${FIXTURE}" \
  "http://127.0.0.1:18080/api/uploads/${JOB_ID}" >/dev/null

curl -fsS \
  -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  "http://127.0.0.1:18080/api/uploads/${JOB_ID}/finalize" >/dev/null

for _ in $(seq 1 40); do
  STATUS=$(curl -fsS "http://127.0.0.1:18080/api/jobs/$JOB_ID" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && exit 1
  sleep .25
done

REPORT_API=$(echo "$REPORT_PATH" | sed 's#^/r/#/api/public-reports/#')
REPORT=$(curl -fsS "http://127.0.0.1:18080$REPORT_API")
echo "$REPORT" | grep -q '"raw_transcript_sent_to_llm":false'
echo "$REPORT" | grep -q '"spec_kitty"'
if echo "$REPORT" | grep -q 'sk-ant-'; then
  echo "secret leaked in report"
  exit 1
fi

echo "native smoke ok: $JOB_ID"
