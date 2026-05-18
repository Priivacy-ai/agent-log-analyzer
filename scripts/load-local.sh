#!/usr/bin/env sh
set -eu

COUNT="${1:-25}"
URL="${CLAUDE_ANALYZER_URL:-http://127.0.0.1:8080}"

i=1
while [ "$i" -le "$COUNT" ]; do
  curl -fsS -F "log=@testdata/fixtures/sample-claude.jsonl" "$URL/api/jobs" >/dev/null &
  i=$((i + 1))
done
wait

echo "submitted $COUNT jobs"

