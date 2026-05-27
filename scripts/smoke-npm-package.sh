#!/usr/bin/env bash
set -euo pipefail

required_binaries=(
  "npm/bin/agent-analyzer-darwin-x64"
  "npm/bin/agent-analyzer-darwin-arm64"
  "npm/bin/agent-analyzer-linux-x64"
  "npm/bin/agent-analyzer-linux-arm64"
  "npm/bin/agent-analyzer-win32-x64.exe"
)

for binary in "${required_binaries[@]}"; do
  test -f "$binary"
done

test -z "$(node -e 'const p=require("./package.json"); const s=p.scripts||{}; const bad=["preinstall","install","postinstall","prepare"].filter(k => s[k]); console.log(bad.join("\n"))')"
npm pack --dry-run

node npm/bin/agent-analyzer.js version | grep -q "^agent-analyzer "

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cat >"$tmpdir/sample.jsonl" <<'JSONL'
{"type":"user","message":"hello"}
{"type":"assistant","message":"world"}
JSONL

node npm/bin/agent-analyzer.js analyze --log "$tmpdir/sample.jsonl" --out "$tmpdir/report.json" >/dev/null
node -e '
const fs = require("node:fs");
const crypto = require("node:crypto");
const logPath = process.argv[1];
const reportPath = process.argv[2];
const log = fs.readFileSync(logPath);
const report = JSON.parse(fs.readFileSync(reportPath, "utf8"));
if (!report.security_receipt || report.security_receipt.raw_transcript_sent_to_llm) process.exit(1);
const want = crypto.createHash("sha256").update(log).digest("hex");
const got = report.source_reports?.[0]?.log_refs?.[0]?.content_hash_sha256;
if (got !== want) {
  console.error(`expected content_hash_sha256 ${want}, got ${got || "<missing>"}`);
  process.exit(1);
}
' "$tmpdir/sample.jsonl" "$tmpdir/report.json"
