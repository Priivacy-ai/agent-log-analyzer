# Contract: `claude-analyzer analyze` CLI Argument Surface

**Backs:** FR-001, FR-002, FR-003, FR-004
**Implementation site:** `cmd/claude-analyzer/main.go` (`runAnalyze`)
**Test site (NEW):** `cmd/claude-analyzer/main_test.go`

## Surface

```
claude-analyzer analyze [<log-path>] [--log <path>] [--out <path>] [other flags]
```

`<log-path>` is the optional positional argument. `--log` is the existing named flag. They are mutually exclusive.

## Argument resolution table

| `len(args.positional)` | `--log` set | Behavior | Exit | FR backing |
|------------------------|-------------|----------|------|------------|
| 0 | empty | Use `latestClaudeLog()` (unchanged). | 0 (success path) | — |
| 0 | set | Analyze the file named by `--log` (unchanged). | 0 (success path) | — |
| 1 | empty | Analyze the positional path. | 0 (success path) | FR-001 |
| 1 | set | Refuse with error: `claude-analyzer analyze: cannot combine positional log path with --log flag` and exit non-zero. | non-zero | FR-002 |
| ≥ 2 | empty | Refuse with error: `claude-analyzer analyze: unexpected extra argument <name>` (naming the second positional) and exit non-zero. | non-zero | FR-003 |
| ≥ 2 | set | Refuse with error preferring the FR-002 message (positional + --log conflict reported first). | non-zero | FR-002 (precedence over FR-003) |

## Error message stability

Error messages above are part of the contract. Tests assert the substring (case-sensitive) for these two checked phrases:
- `"cannot combine positional log path with --log"` (FR-002)
- `"unexpected extra argument"` (FR-003)

Tests should not assert the entire message string to allow future copyediting without test churn, but the substrings are stable.

## Help / usage text

`usage()` in `cmd/claude-analyzer/main.go` (around `main.go:183`) must list the positional form:

```
usage: claude-analyzer analyze [<log-path>] [--log <path>] [--out <path>] ...

  <log-path>     path to a Claude Code JSONL log; mutually exclusive with --log.
                 if neither is supplied, the latest log in ~/.claude/projects/
                 is used.
  --log <path>   explicit log path; mutually exclusive with a positional <log-path>.
  --out <path>   output path for the sanitized report JSON (default: ./claude-analyzer-report.json).
```

## Doc surface (FR-004)

The following docs must reference the positional form:

- `README.md` — the local runthrough block (around the existing `claude-analyzer analyze` example).
- `docs/testing-plan.md` — the developer verification section that mentions the CLI invocation.
- `web/app.js` — the local command generator; emit a positional form (or `--log` form) consistently, with a brief comment that both work.

## Tests (`cmd/claude-analyzer/main_test.go` — NEW)

Required cases (each isolated; build the binary once with `go test -c` or invoke `runAnalyze` directly):

1. `TestAnalyze_NoArgs_UsesLatest` — passes when `latestClaudeLog()` is shimmed to return a known fixture; assertion: the parsed report's `inputFile` field equals the shim path. (Sanity, not a new behavior.)
2. `TestAnalyze_PositionalOnly_UsesPositional` — passes when invoked with a single positional path; assertion: analyzed file equals positional. (FR-001)
3. `TestAnalyze_LogFlagOnly_UsesLogFlag` — passes when invoked with `--log <path>`; assertion: analyzed file equals `--log`. (Sanity.)
4. `TestAnalyze_PositionalPlusLog_Refuses` — invoke with both `<path>` and `--log`; assertion: non-zero exit and error message contains the FR-002 substring; no report written. (FR-002)
5. `TestAnalyze_TwoPositionals_Refuses` — invoke with two positionals; assertion: non-zero exit and error message contains the FR-003 substring; no report written. (FR-003)
6. `TestAnalyze_PositionalNonExistent_Refuses` — positional path to a file that does not exist; assertion: non-zero exit; error message exists (text not asserted strictly). (Edge: no regression vs `--log` behavior.)

## Out of scope for this contract

- No subcommand renaming (`analyze` stays).
- No new flags.
- No JSON output format changes.
- No batch / multi-path analyze (would expand surface beyond the spec).
