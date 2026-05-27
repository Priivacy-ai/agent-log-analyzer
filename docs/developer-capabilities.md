# Developer Capability Reference

This document describes the current Agent Analyzer implementation from a
developer and maintainer perspective. It covers the local CLI, source discovery,
source-specific readers, normalization, privacy boundaries, aggregation, and the
test surface that protects those capabilities.

## System Shape

Agent Analyzer is a local-first profiler for agentic coding logs.

The public flow is:

1. A user runs the local CLI.
2. The CLI discovers supported agent logs on the user's machine.
3. The CLI reads raw logs locally, redacts secrets, normalizes events, and
   writes a sanitized report JSON.
4. The user reviews that JSON.
5. Only the sanitized report JSON is uploaded.

The server-side app stores and renders sanitized reports. It does not need raw
transcripts for the public launch path.

Important entry points:

- CLI: `cmd/agent-analyzer/main.go`
- Core analyzer: `internal/analyzer/analyzer.go`
- Source-specific normalized events: `internal/analyzer/normalized_events.go`
- Ecosystem allowlists: `internal/analyzer/signatures/*.json`
- Aggregate analytics contract: `internal/analytics`
- Report/remediation artifact safety: `internal/remediation`

## Supported Source IDs

The source IDs used by discovery, normalization, source reports, analytics
allowlists, and remediation allowlists are:

| Source ID | Product / surface |
| --- | --- |
| `claude_code` | Claude Code JSONL sessions |
| `claude_desktop` | Claude Desktop local agent/session JSON and audit logs |
| `codex` | Codex CLI / Codex desktop session JSONL and bounded diagnostic SQLite logs |
| `copilot` | GitHub Copilot CLI sessions/logs and VS Code Copilot Chat sessions |
| `opencode` | OpenCode session directories |
| `claude_desktop_mcp` | Claude Desktop MCP logs |
| `cursor` | Cursor agent transcripts and optional state DB rows |
| `kiro_cli` | Kiro CLI chat logs |
| `kiro_ide` | Kiro IDE logs, workspace sessions, and optional state DB rows |
| `antigravity` | Google Antigravity transcripts and optional state DB rows |

These IDs are registered in `internal/analyzer/signatures/coding_agents.json`.
When adding a new source, keep the discovery source ID, normalizer source ID,
and coding-agent registry ID aligned. Any ID not in the registry is filtered
out by analytics/remediation allowlist code.

## Discovery Model

`recentSupportedLogsWithBounds` is the main discovery coordinator. It collects
candidates from the source registry, Codex session index discovery, OpenCode
session discovery, Kiro workspace session discovery, Codex diagnostic SQLite
discovery, and VS Code-style SQLite source discovery. After collection, it
applies a final largest-recent per-source cap so a source cannot exceed the
requested limit by contributing multiple reader families.

The selection model favors files that are both large enough to be meaningful and
recent enough to represent current workflows. The scoring function is
`largestRecentScore`, with a 14 day half-life.

Discovery boundaries:

- Missing roots are skipped.
- Permission-denied roots and entries are skipped.
- Unexpected filesystem errors are returned with source/root context.
- Unreadable candidates discovered later are skipped at read time if the error
  is permission-related; valid siblings still analyze.

The free scan uses bounded candidates to keep the one-line command responsive.
The full scan can analyze up to 10 largest-recent logs per supported source.

## OS-Aware Roots

Root helpers are deliberately centralized in `cmd/agent-analyzer/main.go`.

General helpers:

- `homeDir()` reads the current user's home directory.
- `appDataDir()` reads `%APPDATA%`, falling back to
  `$HOME/AppData/Roaming`.
- `appSupportDirFor(goos, home, appData, xdgConfig, app)` resolves:
  - macOS: `$HOME/Library/Application Support/<app>`
  - Windows: `%APPDATA%/<app>`
  - Linux: `$XDG_CONFIG_HOME/<app>` or `$HOME/.config/<app>`

Environment variables used by discovery:

- `$HOME`
- `%APPDATA%`
- `%TEMP%` / Go `os.TempDir()`
- `$XDG_CONFIG_HOME`
- `$XDG_RUNTIME_DIR`
- `$CODEX_HOME`
- `$COPILOT_HOME`
- `$KIRO_HOME`
- `$KIRO_CHAT_LOG_FILE`
- `$CLAUDE_CONFIG_DIR`

Source roots:

| Source | Roots |
| --- | --- |
| Claude Code | `$CLAUDE_CONFIG_DIR/projects` or `$HOME/.claude/projects` |
| Claude Desktop | app-support `Claude/local-agent-mode-sessions`, `Claude/claude-code-sessions`, and `Claude/audit.jsonl` |
| Codex | `$CODEX_HOME/session_index.jsonl`, `$CODEX_HOME/sessions`, `$CODEX_HOME/archived_sessions`, `$CODEX_HOME/logs*.sqlite`, or `$HOME/.codex/...` |
| GitHub Copilot | `$COPILOT_HOME/session-state`, `$COPILOT_HOME/logs`, or `$HOME/.copilot/...`; VS Code `Code/User/workspaceStorage/*/chatSessions` and `Code/User/globalStorage/emptyWindowChatSessions` |
| OpenCode | `$HOME/.local/share/opencode/storage/message` plus associated part files |
| Claude Desktop MCP | macOS `$HOME/Library/Logs/Claude`; Windows `%APPDATA%/Claude/logs` |
| Cursor JSONL | `$HOME/.cursor/projects`, plus Cursor app-support workspace/global storage roots |
| Kiro CLI | exact `$KIRO_CHAT_LOG_FILE` when set, `$KIRO_HOME/logs`, temp/runtime Kiro log roots |
| Kiro IDE | Kiro app-support `logs` plus Kiro workspace-session storage |
| Antigravity JSONL | `$HOME/.gemini/antigravity*` plus Antigravity app-support workspace/global storage roots |
| SQLite state | app-support `User/globalStorage` and `User/workspaceStorage` for Cursor, Kiro, and Antigravity |

Linux Kiro runtime discovery only uses `$XDG_RUNTIME_DIR/kiro-log` when
`XDG_RUNTIME_DIR` is non-empty; otherwise it falls back to an absolute temp root
instead of scanning a relative `kiro-log` directory.

## Readers

### Plain Path Reader

Most file-backed sources use `recentPathLogs`, which walks configured roots and
accepts files using a source-specific predicate.

Examples:

- Codex accepts `.jsonl`.
- GitHub Copilot accepts Copilot CLI `events.jsonl`, Copilot CLI `.log` files, and VS Code `chatSessions/*.json`.
- Claude Desktop accepts `local_*.json` session files and `audit.jsonl`.
- Claude Desktop MCP accepts `mcp.log` and `mcp-server-*.log`.
- Cursor accepts JSONL under `agent-transcripts`.
- Kiro CLI accepts the exact `KIRO_CHAT_LOG_FILE` path when configured, or
  files named `kiro-chat.log`.
- Antigravity accepts `transcript.jsonl`.

### Claude Desktop MCP Server Header Reader

Claude Desktop server logs are often named `mcp-server-<server>.log`. For those
logs, the reader prepends a bounded synthetic availability header:

```text
Available MCP servers:
- <server>
```

The server name is sanitized to a bounded identifier. This lets the existing
MCP utilization detector count known public MCP servers without emitting raw
paths or private server metadata.

### OpenCode Session Reader

OpenCode uses a directory/session layout rather than a single JSONL file. The
reader:

- selects `ses_*` message directories,
- reads message JSON files in stable order,
- follows message IDs to associated part files,
- appends message and part JSON as synthetic JSONL for the analyzer.

### Kiro Workspace Session Reader

Kiro IDE workspace sessions live under app-support global storage:

```text
Kiro/User/globalStorage/kiro.kiroagent/workspace-sessions
```

The reader accepts session `.json` files and skips `sessions.json`. The
normalizer walks nested arrays/maps, so hook events inside session `history`
arrays are converted into tool calls/results.

### Claude Desktop Session Reader

Claude Desktop local-agent/session logs can live in app-support session trees:

```text
Claude/local-agent-mode-sessions
Claude/claude-code-sessions
```

The reader accepts `local_*.json` session files and `audit.jsonl`. Session JSON
is passed through as JSONL-compatible input, and top-level `enabledMcpTools`
metadata is converted into a bounded synthetic MCP availability header using
only sanitized server IDs.

### Codex Session Index And Diagnostic Reader

Codex discovery prefers `$CODEX_HOME/session_index.jsonl` when present. The
index is scanned for JSONL path references, then each resolved session is scored
by size and recency. If the index is missing or produces no readable candidates,
discovery falls back to `$CODEX_HOME/sessions` and
`$CODEX_HOME/archived_sessions`.

Codex also discovers `$CODEX_HOME/logs*.sqlite`. Those databases are opened in
SQLite read-only/query-only mode, and only bounded rows from a `logs` table are
converted to synthetic diagnostic JSONL. The reader never emits source paths,
raw session IDs, or unbounded diagnostic text.

### SQLite State Reader

Cursor, Kiro, and Antigravity can store conversation state in VS Code-style
SQLite databases named `state.vscdb`.

SQLite extraction is part of normal automatic discovery. The CLI should read
every supported store it can read safely, while preserving the invariant that
it does not change source files. The only normal filesystem write made by the
CLI is the sanitized report path requested by the user.

The SQLite reader:

- discovers `state.vscdb` under app-support `User/globalStorage` and
  `User/workspaceStorage`,
- includes `state.vscdb-wal` and `state.vscdb-shm` sizes in candidate bounds,
- opens databases through a SQLite `mode=ro` URI with `query_only` enabled,
- skips missing or unreadable WAL/SHM files,
- reads only `ItemTable` and `cursorDiskKV`,
- pushes key-prefix filtering into SQL before `LIMIT`,
- uses deterministic `ORDER BY key`,
- emits bounded synthetic JSONL rows,
- caps emitted JSONL by the source byte budget when one is provided,
- skips unsupported keys, empty values, invalid UTF-8/protobuf blobs, and
  unknown state rows.

Supported key prefixes:

| Source | Prefixes |
| --- | --- |
| Cursor | `bubbleId:`, `composerData:`, `composer.composerData`, `agentKv:`, `agentKv:blob:`, `messageRequestContext:`, `aiService.prompts`, `aiService.generations`, `workbench.panel.aichat.view.aichat.chatdata`, `workbench.panel.chat.view.chat.chatdata` |
| Kiro IDE | `kiro.kiroAgent`, `kiro:`, `chat`, `session` |
| Antigravity | `agent`, `chat`, `conversation`, `task`, `transcript` |

Synthetic SQLite JSONL rows contain only:

- `type`
- `kind`
- sanitized `key_type`
- bounded `content`
- `truncated`

Raw DB keys are never emitted. The normalizers parse stringified JSON in
SQLite `content` when it is valid JSON, so a supported row containing a tool
call still contributes tool-call signals.

## Normalization

The analyzer first scrubs raw input, then `normalizeEvents` converts supported
source shapes into bounded `normalizedEvent` records.

`normalizedEvent` is an internal type. It carries counts, closed-enum roles and
kinds, bounded tool identifiers, and hashes of call IDs/tool arguments. It does
not carry prompts, command arguments, raw outputs, paths, DB keys, workspace
names, or transcript IDs.

### Codex

Codex supports session selection through `session_index.jsonl`, older direct
JSONL, newer rollout-style records with a top-level `payload`, and synthetic
diagnostic rows from bounded `logs*.sqlite` reads.

The Codex normalizer:

- unwraps `payload`,
- handles `session_meta`, `turn_context`, `compacted`, `event_msg`, and
  `response_item`,
- reads token counts from nested `event_msg` / `last_token_usage`,
- maps `response_item` item types:
  - `function_call`
  - `local_shell_call`
  - `custom_tool_call`
  - `tool_search_call`
  into `tool_call`,
- maps corresponding `*_output` item types into `tool_result`,
- hashes call IDs and tool arguments instead of emitting raw values,
- extracts patch statistics from patch events.

Diagnostic SQLite rows contribute bounded error/signal counts through synthetic
JSONL and do not emit raw file paths, session IDs, or diagnostic bodies.

### Claude Desktop

Claude Desktop support covers local-agent/session JSON files and audit JSONL.
The normalizer:

- maps initial session metadata and enabled MCP tools to bounded message/tool
  availability signals,
- recognizes JSON-RPC tool/resource calls and matching results,
- maps embedded hook/tool objects to tool calls/results,
- recursively walks nested arrays/maps for supported event shapes,
- hashes IDs and arguments instead of emitting raw values.

### Claude Desktop MCP

Claude Desktop MCP logs are text logs containing JSON-RPC payloads. The raw-line
preprocessor strips timestamp/log-level prefixes by extracting the JSON suffix.

The MCP normalizer:

- recognizes JSON-RPC `tools/call` and `resources/read` methods as tool calls,
- records request IDs only for those tool/resource requests,
- emits a tool result only when a later JSON-RPC `result` has a matching tracked
  request ID,
- ignores `initialize`, `tools/list`, and similar non-call results for tool
  result counts.

### Cursor

Cursor support covers transcript JSONL first, plus SQLite synthetic
JSONL.

The Cursor normalizer:

- maps direct transcript objects with `tool`, `args`, `arguments`, or `input`
  into tool calls,
- handles SQLite synthetic rows with `key_type`,
- parses JSON stored as a string in SQLite `content` and normalizes nested tool
  calls where present,
- hashes argument payloads.

### Kiro CLI And Kiro IDE

Kiro logs can include timestamp/level prefixes followed by JSON. The raw-line
preprocessor extracts valid JSON suffixes.

The Kiro normalizer:

- maps `PreToolUse` style hook events to tool calls,
- maps `PostToolUse`, `tool_response`, and `output` shapes to tool results,
- hashes session/call IDs,
- walks nested session structures such as workspace `history` arrays.

### Antigravity

Antigravity transcript support maps:

- `USER_INPUT` / `user` to user messages,
- terminal/tool/command events to tool calls,
- result/output events to tool results.

Raw command text and tool output are counted or hashed, not emitted.

## Report And Aggregation Capabilities

The analyzer emits `Report` with:

- score and waste range,
- token and workflow metrics,
- deterministic findings,
- ecosystem signals,
- tooling utilization,
- timeline buckets,
- source reports,
- security receipt,
- aggregate-safe event,
- recommendation set.

When multiple sources are analyzed, source reports are grouped by source ID.
Each source report includes bounded metrics/findings/timeline/signals and
ordinal log references. Each log reference may include `content_hash_sha256`,
the SHA-256 hash of the exact local bytes analyzed for that candidate. This is
not a path hash and it is not derived from workspace, thread, session, or DB key
material.

`AnalyzedLogRef.LocalRef` is intentionally ordinal only:

```text
<source-id>-log-<ordinal>
```

It is not a hash of a path, thread ID, workspace name, session ID, DB key, or
any other private value.

`AnalyzedLogRef.ContentHashSHA256` is allowed to contain a SHA-256 content hash
of the exact local bytes analyzed for that log candidate. Analytics may retain
that hash to dedupe repeated scans of the same analyzed log payload. Do not
replace it with a path hash or any hash derived from a local path, workspace,
session ID, DB key, or thread identifier.

## Privacy Invariants

Raw logs and raw state stores are toxic inputs.

The public flow must not upload or emit:

- raw transcript lines,
- raw paths,
- thread IDs,
- workspace names,
- tool arguments,
- prompts or drafts,
- DB keys,
- protobuf blobs,
- OAuth/session material,
- unknown private tool names,
- stable hashes of private strings.

Allowed report/analytics surfaces are bounded to:

- known public allowlist IDs,
- counts,
- buckets,
- booleans,
- closed enums,
- redaction totals,
- token/tool/output byte counts,
- ordinal local references.

Unknown private names are counted, not stored.

## Tests Protecting These Capabilities

Important tests:

- `cmd/agent-analyzer/main_test.go`
  - source discovery for desktop and agent sources,
  - explicit `--source` override for explicit `--log` paths,
  - cross-platform root helpers,
  - exact `KIRO_CHAT_LOG_FILE` discovery,
  - app-support Cursor/Antigravity transcripts,
  - Claude Desktop local-session and audit discovery,
  - Claude Desktop MCP server-log header synthesis,
  - Codex `session_index.jsonl` preference and diagnostic SQLite reads,
  - permission-denied discovery/read behavior,
  - Kiro workspace session reads and nested tool signal extraction,
  - SQLite default discovery, read-only source behavior, empty stores,
    Cursor legacy/current keys, Kiro/Antigravity stores, filtering before
    limit, output bounds, invalid UTF-8/protobuf skipping,
  - ordinal-only local refs.
- `internal/analyzer/normalized_events_test.go`
  - Codex payload token/tool normalization,
  - desktop-agent source-specific signals,
  - MCP request/result pairing,
  - registered coding-agent source IDs.
- `internal/analyzer/leak_test.go`
  - end-to-end serialization canaries,
  - hostile fixtures for new desktop source normalizers.

Verification used for the current implementation:

```sh
go test ./cmd/agent-analyzer ./internal/analyzer
go test ./...
git diff --check
```

## Adding A New Source

Use this checklist:

1. Add the source ID to `representativeSourceOrder` when it should appear in
   representative local scans.
2. Add the source ID to `internal/analyzer/signatures/coding_agents.json`.
3. Add a source definition in `logSourceDefinitions()` or a dedicated reader
   if the source is directory/database-backed.
4. Keep root resolution OS-aware. Prefer helper functions that can be unit
   tested without changing `runtime.GOOS`.
5. Bound discovery by file size and final per-source limit.
6. Skip permission-denied roots and unreadable candidates without aborting valid
   siblings.
7. Normalize into counts, enums, and hashes only. Do not add raw string fields
   to `normalizedEvent` or `Report`.
8. Add synthetic fixtures for:
   - path discovery,
   - parser mapping,
   - leak canaries,
   - permission/read failure behavior,
   - read-only database/state extraction behavior for raw-sensitive stores.
9. Run `go test ./...` and `git diff --check`.

## Known Human Validation Gap

Synthetic coverage is broad, but real logs from desktop tools can vary across
versions. Before calling a new source production-complete, validate against
real logs from macOS, Windows, and Linux machines and add sanitized fixtures for
any new shapes found.
