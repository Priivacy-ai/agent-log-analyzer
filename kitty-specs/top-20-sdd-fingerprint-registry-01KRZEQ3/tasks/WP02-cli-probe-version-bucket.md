---
work_package_id: WP02
title: CLI probe abstraction + version-bucket normalizer
dependencies:
- WP01
requirement_refs:
- FR-008
- FR-009
- FR-016
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T006
- T007
- T008
phase: Phase 1 — Foundation
agent: "claude:opus-4.7:implementer-ivan:implementer"
shell_pid: "5177"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/
execution_mode: code_change
owned_files:
- internal/analyzer/sdd/probe.go
- internal/analyzer/sdd/version_bucket.go
- internal/analyzer/sdd/probe_test.go
- internal/analyzer/sdd/version_bucket_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP02 — CLI probe abstraction + version-bucket normalizer

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

If no profile is specified, run `spec-kitty agent profile list` and select the best
match for this work package's `task_type` and `authoritative_surface`.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.
- If human instructions contradict these fields, stop and resolve.

## Objectives & Success Criteria

- A `sdd.CLIProbe` interface and two implementations: `RealProbe` (production) and `FakeProbe` (test).
- A `normalizeVersionBucket(raw string) string` helper following research R-04.
- A registry-loader extension that rejects unsafe `version_args` per `contracts/cli-probe.md`.
- Unit tests prove the resolved CLI path NEVER appears in any value returned by `RealProbe` and that raw `--version` output is never returned to external callers.

## Context & Constraints

- Read: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/cli-probe.md` (this is the **contract**).
- Read: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md` §R-03, §R-04.
- Read: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md` (CLIProbe).
- C-003: probes target only allowlisted binary names; no network, no auth, no file modification, no shell.
- NFR-002: 2-second timeout per probe.

## Subtasks & Detailed Guidance

### Subtask T006 — `sdd/probe.go`

- **Purpose**: Encapsulate the CLI probe behind a tight interface so privacy invariants live in one place.
- **Steps**:
  1. Create `internal/analyzer/sdd/probe.go`.
  2. Declare:
     ```go
     type CLIProbe interface {
         LookPath(name string) (found bool)
         Version(ctx context.Context, name string, args []string) (rawOutput string, ok bool)
     }
     ```
  3. Implement `RealProbe`:
     - `LookPath(name string) bool`:
       ```go
       _, err := exec.LookPath(name)
       return err == nil
       ```
       Do **not** capture, return, or log the resolved path. Use the underscore explicitly.
     - `Version(ctx, name, args) (string, bool)`:
       - `cmd := exec.CommandContext(ctx, name, args...)`
       - `cmd.Env = []string{}` (empty env)
       - `cmd.Stdin = nil`
       - `out, err := cmd.Output()` — combined `stdout`; `stderr` is discarded by default. If you need to bound `stdout` size, copy into a 4 KiB-bounded `bytes.Buffer` and cancel on overflow.
       - On error or non-zero exit: return `("", false)`.
       - On success: return `(string(out), true)` — but the caller (the evaluator) MUST normalize before storing.
     - Document with a comment that callers MUST normalize the returned raw string before any storage.
  4. Implement `FakeProbe`:
     ```go
     type FakeProbe struct {
         Installed map[string]bool
         Versions  map[string]string
     }
     func (f FakeProbe) LookPath(name string) bool {
         return f.Installed[name]
     }
     func (f FakeProbe) Version(_ context.Context, name string, _ []string) (string, bool) {
         v, ok := f.Versions[name]
         return v, ok && v != ""
     }
     ```
- **Files**: `internal/analyzer/sdd/probe.go` (new, ~110 lines).

### Subtask T007 — `sdd/version_bucket.go`

- **Purpose**: Bound the cardinality and prevent leakage of raw version output.
- **Steps**:
  1. Create `internal/analyzer/sdd/version_bucket.go`.
  2. Implement:
     ```go
     func normalizeVersionBucket(raw string) string {
         if raw == "" {
             return ""
         }
         // 1. Strip ANSI escapes.
         raw = ansiRE.ReplaceAllString(raw, "")
         // 2. First MAJOR.MINOR match.
         m := versionRE.FindStringSubmatch(raw)
         if len(m) < 2 {
             return ""
         }
         bucket := m[1]
         // 3. Defensive cardinality bound.
         if len(bucket) > 16 {
             return ""
         }
         return bucket
     }

     var (
         ansiRE    = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
         versionRE = regexp.MustCompile(`(\d+)\.(\d+)`)
     )
     ```
     Then post-process the capture to keep just `\d+\.\d+` (use `(\d+\.\d+)` with a single capture group and strip any trailing `.0+` if present).
  3. Crucially: the function MUST NOT return the raw input under any circumstance. It returns only the bucket (or empty string).
- **Files**: `internal/analyzer/sdd/version_bucket.go` (new, ~50 lines).

### Subtask T008 — Tests

- **Files**: `internal/analyzer/sdd/probe_test.go` (new), `internal/analyzer/sdd/version_bucket_test.go` (new).

**`version_bucket_test.go`** (~80 lines):

- Table-driven cases:
  - `"openspec 1.2.3"` → `"1.2"`
  - `"openspec 1.2.3 built /private/path"` → `"1.2"` (path NOT in result)
  - `"openspec v0.4.1\n"` → `"0.4"`
  - `"openspec 1.2.3 jdoe@host"` → `"1.2"` (email/host NOT in result)
  - `""` → `""`
  - `"no version here"` → `""`
  - `"\x1b[31mopenspec 1.2.3\x1b[0m"` → `"1.2"` (ANSI stripped)
  - Pathological long input (10 KiB of `x`) → `""` (does not echo back input).

**`probe_test.go`** (~120 lines):

- `TestFakeProbe_BasicBehavior`: trivial happy-path on `FakeProbe`.
- `TestRealProbe_DoesNotLeakPath`:
  - Use `RealProbe{}`. `LookPath("go")` returns `true`. Assert the test's full output (capture via `t.Logf`) and the returned `bool` carry no slash and no instance of `/usr/local/go/bin/go` (or the system equivalent). Reflectively check that the returned type has no `string` field exposing a path. (Since `LookPath` only returns a bool by signature, this is structural.)
- `TestRealProbe_VersionHonorsContextDeadline`:
  - `ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)`
  - Run `RealProbe{}.Version(ctx, "go", []string{"version"})` — likely times out → returns `("", false)`. Assert.
- `TestRealProbe_VersionEmptyEnv`:
  - Run against `go version`. Assert `ok == true` (the binary should exist on PATH in CI). Assert `rawOutput != ""`. This is the **only** test that touches the real raw output; explicitly verify it's an in-package consumer (no external assertion on raw contents).

**Note**: every other unit test in the `sdd` package MUST use `FakeProbe`. Real exec is exercised only here.

### (T009 removed)

The `version_args` deny-list is now implemented in WP01 (inside `registry.go`) so this WP does not cross file-ownership boundaries.

## Test Strategy

See T008. Run `go test ./internal/analyzer/sdd/...` and confirm:

- 100% of paths-or-secrets-leak tests assert *absence* of canary strings, not just presence of expected values.
- The integration test against `go version` is the only `exec`-touching test in the suite. Mark it with `t.Skip` if `exec.LookPath("go")` returns false (e.g., minimal CI).

## Risks & Mitigations

- **`exec.Output()` includes stderr unexpectedly** — Go's `exec.Cmd.Output` does not include stderr by default; verify in the implementation.
- **PATH on the running developer machine differs from CI** — handled by `t.Skip` guard.
- **Future contributors call `RealProbe.LookPath` from outside the `sdd` package** — mitigated by keeping `RealProbe` unexported if practical (return as `CLIProbe` interface from a constructor `NewRealProbe()`).

## Review Guidance

- Confirm `RealProbe.LookPath` discards the resolved path. Search for any string-typed return that could carry the path.
- Confirm `RealProbe.Version` runs with empty env, nil stdin, no shell, bounded by ctx.
- Confirm `normalizeVersionBucket` never returns its input verbatim.
- Confirm the version-args deny-list test covers each forbidden flag and a `/`-containing value.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T07:32:12Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=5177 – Started implementation via action command
