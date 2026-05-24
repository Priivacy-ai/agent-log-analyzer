#!/usr/bin/env bash
set -euo pipefail

target="${SPEC_KITTY_BENCHMARK_TARGET:-/tmp/agent-analyzer-spec-kitty-target}"
repo_url="${SPEC_KITTY_REPO_URL:-https://github.com/Priivacy-ai/spec-kitty.git}"
ref="${SPEC_KITTY_BENCHMARK_REF:-38abeebf6fab2215fb52a099bfad707a7a503ad7}"

if [[ ! -d "$target/.git" ]]; then
  rm -rf "$target"
  git clone "$repo_url" "$target"
fi

git -C "$target" fetch --tags origin
git -C "$target" checkout --detach "$ref"
git -C "$target" clean -fdx
git -C "$target" reset --hard "$ref"
printf '%s\n' "$target"
