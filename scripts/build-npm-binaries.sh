#!/usr/bin/env bash
set -euo pipefail

version="${AGENT_ANALYZER_VERSION:-${1:-dev}}"
commit="${AGENT_ANALYZER_COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}"
date="${AGENT_ANALYZER_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

mkdir -p npm/bin

build() {
  goos="$1"
  goarch="$2"
  node_platform="$3"
  node_arch="$4"
  suffix="$5"
  output="npm/bin/agent-analyzer-${node_platform}-${node_arch}${suffix}"
  echo "building ${output}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
    -ldflags="-s -w -X main.version=${version} -X main.commit=${commit} -X main.date=${date}" \
    -o "$output" ./cmd/agent-analyzer
  chmod +x "$output" || true
}

build darwin amd64 darwin x64 ""
build darwin arm64 darwin arm64 ""
build linux amd64 linux x64 ""
build linux arm64 linux arm64 ""
build windows amd64 win32 x64 ".exe"
