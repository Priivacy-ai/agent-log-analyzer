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

CLIENT_REPORT_JOB_ID=$(printf '%s' "$CLIENT_REPORT_URL" | sed -n 's#^.*/r/\([^/]*\)/.*#\1#p')
CLIENT_REPORT_TOKEN=$(printf '%s' "$CLIENT_REPORT_URL" | sed -n 's#^.*/r/[^/]*/\([^/?]*\).*#\1#p')
curl -fsS \
  -X POST \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "Accept: text/html" \
  --data "email=smoke%40example.com&marketing_opt_in=1&source_report_job_id=${CLIENT_REPORT_JOB_ID}&source_report_token=${CLIENT_REPORT_TOKEN}" \
  "http://127.0.0.1:8080/api/email-unlocks" >/dev/null

CONFIRM_EMAIL=$(docker compose exec -T api sh -c 'cat "$(ls -1 /data/emails/*.eml | sort | tail -1)"')
CONFIRM_PATH=$(printf '%s\n' "$CONFIRM_EMAIL" | sed -n 's#^http://127.0.0.1:8080\(/email/confirm/.*\)#\1#p' | tail -1)
if [ -z "$CONFIRM_PATH" ]; then
  echo "failed to find confirmation link in local email sink"
  exit 1
fi
CONFIRM_PAGE=$(curl -fsS "http://127.0.0.1:8080$CONFIRM_PATH")
echo "$CONFIRM_PAGE" | grep -q 'npx --yes agent-analyzer@latest full-scan --token'
COMMAND_EMAIL=$(docker compose exec -T api sh -c 'cat "$(ls -1 /data/emails/*.eml | sort | tail -1)"')
FULL_SCAN_TOKEN=$(printf '%s\n' "$COMMAND_EMAIL" | sed -n "s/.*full-scan --token '\\([^']*\\)'.*/\\1/p" | tail -1)
if [ -z "$FULL_SCAN_TOKEN" ]; then
  echo "failed to find full-scan token in local email sink"
  exit 1
fi
FULL_SCAN_HOME="$(mktemp -d)"
mkdir -p "$FULL_SCAN_HOME/.claude/projects/smoke" "$FULL_SCAN_HOME/.codex/sessions/2026/05/21"
for _ in $(seq 1 80); do cat "$FIXTURE"; done > "$FULL_SCAN_HOME/.claude/projects/smoke/session-1.jsonl"
for _ in $(seq 1 80); do cat "$FIXTURE"; done > "$FULL_SCAN_HOME/.codex/sessions/2026/05/21/rollout-smoke.jsonl"
FULL_SCAN_REPORT="$(mktemp -t agent-analyzer-full-scan.XXXXXX.json)"
HOME="$FULL_SCAN_HOME" go run ./cmd/agent-analyzer full-scan \
  --token "$FULL_SCAN_TOKEN" \
  --base-url http://127.0.0.1:8080 \
  --out "$FULL_SCAN_REPORT" \
  --limit 1 \
  --no-open >/tmp/agent-analyzer-full-scan.out
FULL_SCAN_REPORT_URL=$(sed -n 's/^Report: //p' /tmp/agent-analyzer-full-scan.out)
if [ -z "$FULL_SCAN_REPORT_URL" ]; then
  echo "failed to upload email-unlocked full scan"
  cat /tmp/agent-analyzer-full-scan.out
  exit 1
fi
FULL_SCAN_REPORT_API=$(printf '%s' "$FULL_SCAN_REPORT_URL" | sed 's#^http://127.0.0.1:8080/r/#/api/public-reports/#')
FULL_SCAN_BODY=$(curl -fsS "http://127.0.0.1:8080$FULL_SCAN_REPORT_API")
echo "$FULL_SCAN_BODY" | grep -q '"parser_type":"full_scan_bundle"'
FULL_SCAN_ARTIFACT_API=$(echo "$FULL_SCAN_REPORT_URL" | sed 's#^http://127.0.0.1:8080/r/#/api/public-artifacts/#')/plugin.zip
curl -fsS "http://127.0.0.1:8080$FULL_SCAN_ARTIFACT_API" -o "$FULL_SCAN_HOME/plugin.zip"
if [ "$(dd if="$FULL_SCAN_HOME/plugin.zip" bs=2 count=1 2>/dev/null)" != "PK" ]; then
  echo "Expected full-scan plugin artifact to be a zip file" >&2
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
