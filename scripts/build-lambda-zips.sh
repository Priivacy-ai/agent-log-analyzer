#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT/.data/lambda}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-arm64}"

mkdir -p "$OUT_DIR"

build_lambda() {
  local package="$1"
  local name="$2"
  local workdir
  workdir="$(mktemp -d)"
  trap 'rm -rf "$workdir"' RETURN
  (
    cd "$ROOT"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -tags lambda -o "$workdir/bootstrap" "$package"
  )
  (
    cd "$workdir"
    zip -q -9 "$OUT_DIR/$name.zip" bootstrap
  )
  rm -rf "$workdir"
  trap - RETURN
}

build_lambda ./cmd/api api
build_lambda ./cmd/worker worker
build_lambda ./cmd/sweeper sweeper
build_lambda ./cmd/email-events email-events

printf 'built lambda zips in %s\n' "$OUT_DIR"
