#!/usr/bin/env sh
set -eu

PORT="${CLAUDE_ANALYZER_SCREENSHOT_PORT:-18081}"
BASE_URL="http://127.0.0.1:${PORT}"
DATA_DIR="$(pwd)/.data/report-fixture-screenshots"
OUT_DIR="${CLAUDE_ANALYZER_SCREENSHOT_OUT:-$(pwd)/tmp/report-fixture-screenshots}"
API_LOG="${DATA_DIR}/api.log"
PLAYWRIGHT="${PLAYWRIGHT:-/opt/homebrew/bin/playwright}"

cleanup() {
  [ -n "${API_PID:-}" ] && kill "$API_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

rm -rf "$DATA_DIR" "$OUT_DIR"
mkdir -p "$DATA_DIR" "$OUT_DIR"

CLAUDE_ANALYZER_DATA_DIR="$DATA_DIR" CLAUDE_ANALYZER_ADDR=":${PORT}" go run ./cmd/api >"$API_LOG" 2>&1 &
API_PID=$!

for _ in $(seq 1 80); do
  if curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep .25
done

curl -fsS "${BASE_URL}/healthz" >/dev/null

"$PLAYWRIGHT" screenshot --browser=chromium --viewport-size=1440,1100 \
  "${BASE_URL}/" "${OUT_DIR}/landing-desktop.png"
"$PLAYWRIGHT" screenshot --browser=chromium --viewport-size=390,844 \
  "${BASE_URL}/" "${OUT_DIR}/landing-mobile.png"

for fixture in testdata/fixtures/reports/*.json; do
  name="$(basename "$fixture" .json)"
  report_url="$(curl -fsS -X POST -H 'Content-Type: application/json' \
    --data-binary "@${fixture}" "${BASE_URL}/api/client-reports" |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["report_url"])')"
  "$PLAYWRIGHT" screenshot --browser=chromium --viewport-size=1440,1100 \
    "$report_url" "${OUT_DIR}/${name}-desktop.png"
  "$PLAYWRIGHT" screenshot --browser=chromium --viewport-size=390,844 \
    "$report_url" "${OUT_DIR}/${name}-mobile.png"
done

echo "report fixture screenshots written to ${OUT_DIR}"
