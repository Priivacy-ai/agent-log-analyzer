#!/usr/bin/env sh
set -eu

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-agent-log-analyzer-smoke}"
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

CLIENT_REPORT="$(mktemp -t agent-analyzer-client.XXXXXX.json)"
go run ./cmd/agent-analyzer analyze --log "$FIXTURE" --out "$CLIENT_REPORT" >/dev/null
CLIENT_UPLOAD=$(go run ./cmd/agent-analyzer upload --base-url http://127.0.0.1:8080 "$CLIENT_REPORT")
CLIENT_REPORT_URL=$(printf '%s\n' "$CLIENT_UPLOAD" | sed -n 's/^Report: //p')
if [ -z "$CLIENT_REPORT_URL" ]; then
  echo "failed to upload sanitized client report"
  exit 1
fi
CLIENT_REPORT_API=$(printf '%s' "$CLIENT_REPORT_URL" | sed 's#^http://127.0.0.1:8080/r/#/api/public-reports/#')
CLIENT_REPORT_BODY=$(curl -fsS "http://127.0.0.1:8080$CLIENT_REPORT_API")
echo "$CLIENT_REPORT_BODY" | grep -q '"raw_log_ttl":"not uploaded"'
CLIENT_EXTENDED_API="${CLIENT_REPORT_API}/extended.md"
curl -fsS "http://127.0.0.1:8080$CLIENT_EXTENDED_API" | grep -q 'Agent Analyzer Extended Report'
CLIENT_ARTIFACT_API=$(echo "$CLIENT_REPORT_URL" | sed 's#^http://127.0.0.1:8080/r/#/api/public-artifacts/#')/plugin.zip
curl -fsS "http://127.0.0.1:8080$CLIENT_ARTIFACT_API" -o /tmp/agent-analyzer-plugin.zip
if [ "$(dd if=/tmp/agent-analyzer-plugin.zip bs=2 count=1 2>/dev/null)" != "PK" ]; then
  echo "Expected single-scan plugin artifact to be a zip file" >&2
  exit 1
fi

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
echo "$REPORT" | grep -q '"tooling_utilization"'
echo "$REPORT" | grep -q '"warning_band"'
if echo "$REPORT" | grep -q 'sk-ant-'; then
  echo "secret leaked in report"
  exit 1
fi

PAID_SESSION=$(curl -fsS \
  -X POST \
  -H "Content-Type: application/json" \
  --data '{"waiver_accepted":true,"acknowledgment":"I accept at my own risk"}' \
  'http://127.0.0.1:8080/api/paid-sessions?legacy_raw_bundle=1')
PAID_JOB_ID=$(echo "$PAID_SESSION" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')
PAID_TOKEN=$(echo "$PAID_SESSION" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
PAID_REPORT_PATH=$(echo "$PAID_SESSION" | sed -n 's/.*"report_path":"\([^"]*\)".*/\1/p')

if [ -z "$PAID_JOB_ID" ] || [ -z "$PAID_TOKEN" ] || [ -z "$PAID_REPORT_PATH" ]; then
  echo "failed to create paid session"
  exit 1
fi

PAID_ROOT="$(mktemp -d)"
mkdir -p "$PAID_ROOT/.claude/projects/smoke"
cp "$FIXTURE" "$PAID_ROOT/.claude/projects/smoke/session-1.jsonl"
cp "$FIXTURE" "$PAID_ROOT/.claude/projects/smoke/session-2.jsonl"
printf '%s\n%s\n' \
  ".claude/projects/smoke/session-1.jsonl" \
  ".claude/projects/smoke/session-2.jsonl" \
  > "$PAID_ROOT/list.txt"
tar -C "$PAID_ROOT" -czf "$PAID_ROOT/bundle.tar.gz" -T "$PAID_ROOT/list.txt"

curl -fsS \
  -X PUT \
  -H "Authorization: Bearer ${PAID_TOKEN}" \
  -H "Content-Type: application/gzip" \
  -H "X-Scan-Limit: 100" \
  --data-binary "@$PAID_ROOT/bundle.tar.gz" \
  "http://127.0.0.1:8080/api/paid-uploads/${PAID_JOB_ID}?limit=100" >/dev/null

curl -fsS \
  -X POST \
  -H "Authorization: Bearer ${PAID_TOKEN}" \
  "http://127.0.0.1:8080/api/paid-uploads/${PAID_JOB_ID}/finalize" >/dev/null

for _ in $(seq 1 60); do
  PAID_STATUS=$(curl -fsS "http://127.0.0.1:8080/api/jobs/$PAID_JOB_ID" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
  if [ "$PAID_STATUS" = "completed" ]; then
    break
  fi
  if [ "$PAID_STATUS" = "failed" ]; then
    curl -fsS "http://127.0.0.1:8080/api/jobs/$PAID_JOB_ID"
    exit 1
  fi
  sleep 1
done

PAID_REPORT_API=$(echo "$PAID_REPORT_PATH" | sed 's#^/r/#/api/public-reports/#')
PAID_REPORT=$(curl -fsS "http://127.0.0.1:8080$PAID_REPORT_API")
echo "$PAID_REPORT" | grep -q '"parser_type":"paid_bundle"'
PAID_SESSION_COUNT=$(echo "$PAID_REPORT" | sed -n 's/.*"session_count":\([0-9][0-9]*\).*/\1/p')
if [ -z "$PAID_SESSION_COUNT" ] || [ "$PAID_SESSION_COUNT" -lt 2 ]; then
  echo "Expected paid scan to report at least 2 parsed sessions, got: ${PAID_SESSION_COUNT:-missing}" >&2
  exit 1
fi
echo "$PAID_REPORT" | grep -q '"raw_transcript_sent_to_llm":false'
echo "$PAID_REPORT" | grep -q '"tooling_utilization"'
echo "$PAID_REPORT" | grep -q '"warning_band"'
PAID_ARTIFACT_API=$(echo "$PAID_REPORT_PATH" | sed 's#^/r/#/api/public-artifacts/#')/plugin.zip
curl -fsS "http://127.0.0.1:8080$PAID_ARTIFACT_API" -o "$PAID_ROOT/plugin.zip"
if [ "$(dd if="$PAID_ROOT/plugin.zip" bs=2 count=1 2>/dev/null)" != "PK" ]; then
  echo "Expected paid plugin artifact to be a zip file" >&2
  exit 1
fi

echo "smoke ok: $JOB_ID paid: $PAID_JOB_ID"
