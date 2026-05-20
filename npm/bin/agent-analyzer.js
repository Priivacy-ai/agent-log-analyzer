#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");
const path = require("node:path");

const target = `${process.platform}-${process.arch}`;
const exe = process.platform === "win32" ? ".exe" : "";
const binary = path.join(__dirname, `agent-analyzer-${target}${exe}`);

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  if (result.error.code === "ENOENT") {
    console.error(`agent-analyzer does not include a binary for ${target}.`);
    console.error("Supported targets: darwin-x64, darwin-arm64, linux-x64, linux-arm64, win32-x64.");
    process.exit(1);
  }
  console.error(result.error.message);
  process.exit(1);
}

if (result.signal) {
  process.kill(process.pid, result.signal);
}

process.exit(result.status ?? 1);
