#!/usr/bin/env bash
set -euo pipefail

COUNT="${1:-25}"
URL="${CLAUDE_ANALYZER_URL:-http://127.0.0.1:8080}"
FIXTURE="${CLAUDE_ANALYZER_FIXTURE:-testdata/fixtures/sample-claude.jsonl}"
TIMEOUT_SECONDS="${CLAUDE_ANALYZER_LOAD_TIMEOUT:-120}"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

if ! curl -fsS "$URL/healthz" >/dev/null; then
  echo "API is not healthy at $URL"
  exit 1
fi

REPORT_JSON="$tmpdir/client-report.json"
go run ./cmd/agent-analyzer analyze --log "$FIXTURE" --out "$REPORT_JSON" >/dev/null

submit_one() {
  local index="$1"
  local session job_id report_path
  session="$(curl -fsS -X POST -H "Content-Type: application/json" --data-binary "@$REPORT_JSON" "$URL/api/client-reports")"
  job_id="$(echo "$session" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')"
  report_path="$(echo "$session" | sed -n 's/.*"report_path":"\([^"]*\)".*/\1/p')"
  if [ -z "$job_id" ] || [ -z "$report_path" ]; then
    return 1
  fi
  printf '%s %s\n' "$job_id" "$report_path" >"$tmpdir/job-$index"
}

for i in $(seq 1 "$COUNT"); do
  submit_one "$i" &
done
wait

cat "$tmpdir"/job-* | sed '/^$/d' | sort >"$tmpdir/jobs"
submitted="$(wc -l < "$tmpdir/jobs" | tr -d ' ')"
if [ "$submitted" != "$COUNT" ]; then
  echo "expected $COUNT submitted jobs, got $submitted"
  exit 1
fi

deadline=$((SECONDS + TIMEOUT_SECONDS))
while [ "$SECONDS" -lt "$deadline" ]; do
  completed=0
  failed=0
  while read -r job_id report_path; do
    status="$(curl -fsS "$URL/api/jobs/$job_id" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
    case "$status" in
      completed) completed=$((completed + 1)) ;;
      failed) failed=$((failed + 1)) ;;
    esac
  done <"$tmpdir/jobs"
  printf 'load status: completed=%s failed=%s total=%s\n' "$completed" "$failed" "$COUNT"
  if [ "$failed" -gt 0 ]; then
    exit 1
  fi
  if [ "$completed" -eq "$COUNT" ]; then
    break
  fi
  sleep 1
done

if [ "$completed" -ne "$COUNT" ]; then
  echo "timed out waiting for jobs to complete"
  exit 1
fi

while read -r job_id report_path; do
  report_api="$(echo "$report_path" | sed 's#^/r/#/api/public-reports/#')"
  report="$(curl -fsS "$URL$report_api")"
  echo "$report" | grep -q '"raw_transcript_sent_to_llm":false'
  echo "$report" | grep -q '"spec_kitty"'
  if echo "$report" | grep -q 'sk-ant-'; then
    echo "secret leaked in report for $job_id"
    exit 1
  fi
done <"$tmpdir/jobs"

echo "load ok: completed $COUNT jobs without report leaks"
