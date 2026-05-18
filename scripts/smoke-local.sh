#!/usr/bin/env sh
set -eu

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-claude-log-analyzer-smoke}"
export COMPOSE_PROJECT_NAME
FIXTURE="${CLAUDE_ANALYZER_FIXTURE:-testdata/fixtures/sample-claude.jsonl}"

cleanup() {
  status=$?
  if [ "$status" -ne 0 ]; then
    docker compose ps || true
    docker compose logs --no-color || true
  fi
  docker compose down -v >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT

docker compose up --build -d

for _ in $(seq 1 60); do
  if wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

SESSION=$(curl -fsS -X POST http://127.0.0.1:8080/api/analysis-sessions)
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
  "http://127.0.0.1:8080/api/uploads/${JOB_ID}" >/dev/null

curl -fsS \
  -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  "http://127.0.0.1:8080/api/uploads/${JOB_ID}/finalize" >/dev/null

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

REPORT_API=$(echo "$REPORT_PATH" | sed 's#^/r/#/api/public-reports/#')
REPORT=$(curl -fsS "http://127.0.0.1:8080$REPORT_API")
echo "$REPORT" | grep -q '"raw_transcript_sent_to_llm":false'
echo "$REPORT" | grep -q '"spec_kitty"'
if echo "$REPORT" | grep -q 'sk-ant-'; then
  echo "secret leaked in report"
  exit 1
fi

echo "smoke ok: $JOB_ID"
