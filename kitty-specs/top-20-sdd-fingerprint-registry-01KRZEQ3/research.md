# Phase 0 — Research

**Mission**: `top-20-sdd-fingerprint-registry-01KRZEQ3`
**Date**: 2026-05-19
**Scope**: Resolve open technical and procedural questions before Phase 1 design.

## R-01 Public-source fingerprint methodology

**Decision.** For each of the 20 SDD tools, researchers populate one file in `docs/research/sdd-fingerprints/<tool-id>.md` using the template below. The methodology is implemented as task WP02 (research gate). The resulting per-tool fingerprints feed `internal/analyzer/signatures/sdd_detectors.json`.

**Per-tool template** (one Markdown file per tool):

```markdown
# <Display Name> (<id>)

- Status: verified | research_needed | blocked
- Category: <category>
- Competitor priority: <integer; 1 = highest>
- Official repository: <url>
- Official docs: <url>
- Release / package source: <url>
- Aliases: [...]

## Markers (public-source only)

### config_dir
- <path or glob, with citation>

### config_file
- <file name, with citation>

### package_manifest
- <package name, registry, link>

### command_name
- <CLI binary name>

### slash_command
- <slash command name>

### mcp_server_name
- <mcp__<server>__ form>

### skill_name
- <skill identifier>

### plugin_manifest
- <plugin manifest filename / id>

### cli_binary
- <binary name allowlisted for `LookPath`>

### cli_version_probe
- <flag(s): `--version` | `version`>
- <expected output pattern; bucket scheme>

## Negative-test markers (must NOT trigger this detector alone)
- <generic name>

## Confidence wiring
- High: <which combinations>
- Medium: <which combinations>
- Low: <textual mention pattern>

## Source references (citations)
- <url> — <what it shows>
```

**Rationale.** Forces every fingerprint to be auditable against a public source before it ships. Forbids guessing.

**Alternatives considered.**
- Embedding citations inline in `sdd_detectors.json`. Rejected because JSON is the wrong format for prose evidence and PR review surface for citation drift.
- Storing references only as a flat list. Rejected because reviewers need per-marker citation, not per-tool.

## R-02 Cloning, installing, and inspecting tools

**Decision.** Cloning and disposable install runs happen **outside** the repo, under:

```
/Users/robert/code-analyzer-dev/claude-code-analyzer-20260519-082245-0QWuF7/research/<tool-id>/
```

Researchers may run `init` / `new` commands in disposable scratch directories under that tree to observe the artifacts the tool generates. The repo never absorbs the cloned source. Fingerprints derived from running the tool are equivalent in trust to fingerprints derived from the tool's own docs — both are public-source.

**Rationale.** Some tools document their config layout only loosely; running their `init` in a sandbox is often the cheapest way to see what files they actually generate. Keeping the research clones outside the repo prevents accidental commits of third-party source and keeps the diff clean.

**Alternatives considered.**
- Cloning into a `vendor/` or `third_party/` directory in the repo. Rejected — license/redistribution issues, repo bloat, and no need.
- Docs-only research. Rejected — too easy to miss the actual generated artifacts.

## R-03 CLI probe safety model

**Decision.** The CLI probe is expressed as a small Go interface (`sdd.CLIProbe`) with two methods:

```go
type CLIProbe interface {
    LookPath(name string) (found bool)
    Version(ctx context.Context, name string, args []string) (rawOutput string, ok bool)
}
```

A `RealProbe` implementation wraps `exec.LookPath` and `exec.CommandContext` with the following hard rules:

- The argument `name` must come from the allowlisted binary list in the loaded registry. Callers may not pass arbitrary strings; the evaluator only calls `LookPath` / `Version` for IDs whose detector entry has a `cli_binary` marker.
- `LookPath` returns only a boolean. The resolved path is **never** returned to callers or logged anywhere. The path lives inside the function and is discarded.
- `Version` runs `exec.CommandContext(ctx, binary, args...)` with `cmd.Env = []string{}` (sanitized env), `cmd.Stdin = nil`, no shell. `args` must come from the detector entry's `cli_version_probe.args`.
- Context carries a 2-second deadline (NFR-002). Timeout = `installed: true, version_bucket: ""` (or no bucket field) and no probe-derived value other than installed presence.
- `Version` returns `(rawOutput, ok)`. The raw output is passed only to `normalizeVersionBucket`, which extracts a bounded bucket and the **raw string is then discarded**. The raw output is never returned to callers outside `sdd` package internals and never stored in any value emitted to the report.

A `FakeProbe` implementation is used by every test. Real `exec` calls do not occur in unit tests.

**Rationale.** Centralizing the safety rules in one type makes them auditable in one place and prevents accidental leakage at every call site.

**Alternatives considered.**
- Free-function `LookPath` / `Version` wrappers. Rejected — harder to inject in tests, easier to accidentally call the unsafe version.
- Shelling out via `os/exec.Cmd` directly inside the evaluator. Rejected — couples privacy invariants to every call site.

## R-04 Version-bucket normalization

**Decision.** `normalizeVersionBucket(raw string) string` follows these rules in order:

1. Strip ANSI escape sequences.
2. Extract the **first** `MAJOR.MINOR` (or `MAJOR.MINOR.PATCH` truncated to `MAJOR.MINOR`) regex hit using `^\s*[^\d]*(\d+\.\d+)`. Bucket = that capture group.
3. If no match, bucket = `""` (empty string, omitted from JSON via `omitempty`).
4. The raw output is never returned to the caller; only the bucket value (or empty string) is.
5. The bucket is further constrained to a max length of 16 characters as a defense-in-depth bound on cardinality.

**Rationale.** Most CLIs print `<tool> 1.2.3` on `--version`. Truncating to `MAJOR.MINOR` keeps cardinality bounded across users while preserving useful adoption signal. Discarding any unparseable raw output prevents path/email/hostname leakage through unusual `--version` formats.

**Alternatives considered.**
- Keep `MAJOR.MINOR.PATCH`. Rejected — finer granularity, higher cardinality, no obvious analytics benefit at this stage.
- Bucket into ranges (`<1.0`, `1.x`, `2.x`). Rejected — loses too much detail; can be done downstream.

## R-05 Confidence-rule rationale

**Decision.** Three-tier discrete confidence following the brief:

- **High**: ≥1 marker from a tool-specific class (`config_dir` matching a tool-specific path or `slash_command` / `mcp_server_name` / `skill_name` / `plugin_manifest`) **plus** ≥1 corroborating marker from a different source class (e.g., `config_file`, `package_manifest`, or `cli_binary`). Special case: `cli_binary` installed + `config_dir` or `config_file` tool-specific.
- **Medium**: exactly one of `cli_binary` installed, `slash_command`, `mcp_server_name`, or `package_manifest` matches, **without** corroborating tool-specific markers.
- **Low**: only `command_name` regex hits via free-text mention (e.g., "I tried openspec yesterday"). Treated as evidence of awareness, not installation.

A detector entry declares which combinations earn which tier; the evaluator returns whichever tier is highest given the matched markers. The evaluator never invents an `Active` boolean — `Active: true` requires high confidence **and** at least one runtime-touchable marker (slash command usage, MCP usage, or CLI presence).

**Rationale.** Predictable, deterministic, fast to test. Aligns with the brief verbatim. The `Active` boolean lets downstream analytics distinguish "installed but unused" from "actually invoked".

**Alternatives considered.**
- A numeric score with thresholds. Rejected — harder to test cross-detector consistency; loses determinism story.
- Per-tool ad-hoc rules. Rejected — non-portable, drifts, and is hard to audit.

## R-06 Forbidden-string leakage test design

**Decision.** A `leak_test.go` file in the `sdd` package builds a fully populated `Report` carrying every kind of fingerprint state, then serializes it via `encoding/json.Marshal` and asserts that none of the following appear:

| Category | Representative test fixture |
| --- | --- |
| user prompts | `"FORBIDDEN-PROMPT-CANARY"` injected into upstream input |
| task descriptions | `"FORBIDDEN-TASK-CANARY"` |
| raw transcripts | `"FORBIDDEN-TRANSCRIPT-CANARY"` |
| raw tool inputs | `"FORBIDDEN-TOOLIN-CANARY"` |
| raw tool outputs | `"FORBIDDEN-TOOLOUT-CANARY"` |
| raw file paths | `"/Users/robert/private-project"` |
| repo URLs | `"git@github.com:private-org/private-repo.git"` |
| branch names | `"customer-private-branch"` |
| usernames | `"jdoe-private"` |
| hostnames | `"corp-internal-host.local"` |
| emails | `"private.user@example.internal"` |
| session IDs | `"sess_PRIVATESESSION"` |
| transcript paths | `"/Users/robert/.claude/projects/PRIVATE/transcript.jsonl"` |
| private MCP names | `"mcp__private__internal"` |
| raw `LookPath` paths | `"/opt/homebrew/bin/openspec"` |
| raw `--version` output | `"openspec 1.2.3 built /private/path"` |

The test runs both `json.Marshal(report)` and `json.Marshal(report.AggregateEvent)` and asserts every canary is **absent** from the serialized output.

**Rationale.** Section presence at the schema level alone is insufficient. A canary test on a fully populated report catches schema drift that accidentally introduces a new string field.

**Alternatives considered.**
- Reflection-based whitelist of allowed string fields. Considered — useful but harder to maintain; the canary test is cheaper and more obvious for review.

## R-07 Cross-negative test design

**Decision.** Three fixture files plus a 3×3 cross-product table (NFR-004):

- `testdata/fixtures/spec_kitty.txt` — contains `.kittify/`, Spec Kitty-specific slash commands, the `spec-kitty` CLI alias.
- `testdata/fixtures/github_spec_kit.txt` — contains the GitHub Spec Kit init artifacts, the `spec-kit` CLI alias, and its specific slash commands.
- `testdata/fixtures/openspec.txt` — contains the OpenSpec init artifacts and its specific slash commands.

Cross-product test: each fixture is run through `Evaluate`. Assertion matrix:

| Fixture | spec_kitty | github_spec_kit | openspec |
| --- | --- | --- | --- |
| spec_kitty.txt | ✓ detected | ✗ not detected | ✗ not detected |
| github_spec_kit.txt | ✗ not detected | ✓ detected | ✗ not detected |
| openspec.txt | ✗ not detected | ✗ not detected | ✓ detected |

Plus a fourth fixture `generic_only.txt` that contains `specs/`, `tasks.md`, `design.md`, `STATE.md`, `hooks`, `requirements.md` and asserts **none** of the three (or any other detector) fires.

**Rationale.** This is the brief's explicit test bar. The 9-assertion floor in NFR-004 derives from this matrix.

**Alternatives considered.**
- Single fixture with markers from all three tools and assert all three fire. Rejected — does not exercise the negative case which is where confusion lives.

## R-08 Bulk-edit classification

**Decision.** `change_mode: feature` (not `bulk_edit`). The mission adds new code (a new `sdd` package), a new JSON registry file, new Go types, and one new field on `Ecosystem`. No existing identifier, path, key, or label is renamed in many files.

The Bulk-Edit Detection check from `/spec-kitty.specify` confirmed this during specify-time discovery.

**Rationale.** Avoids producing an `occurrence_map.yaml` that has no real occurrences to track.

## R-09 Final SDD tool list — post re-research

This table reflects the **final 15-tool registry** after the post-mission
re-research pass. Each entry is backed by public-source citations in the
per-tool research file under `docs/research/sdd-fingerprints/`. The original
brief listed 20 candidates; 5 were removed because no canonical upstream or
public fingerprintable artifact surface could be identified (see
`docs/research/sdd-fingerprints/README.md` "Removed tools" for rationale).

| # | id | Display name | Public anchor |
| --- | --- | --- | --- |
| 1 | `spec_kitty` | Spec Kitty | `.kittify/` and `kitty-specs/` (this repo's own workflow) |
| 2 | `github_spec_kit` | GitHub Spec Kit | github.com/github/spec-kit |
| 3 | `openspec` | OpenSpec | github.com/Fission-AI/OpenSpec |
| 4 | `kiro` | Kiro | kiro.dev (AWS) |
| 5 | `bmad` | BMAD-METHOD | github.com/bmad-code-org/bmad-method |
| 6 | `gsd` | GSD | github.com/gsd-build/get-shit-done |
| 7 | `spec_workflow_mcp` | Spec Workflow MCP | github.com/Pimzino/spec-workflow-mcp |
| 8 | `sdd_pilot` | SDD Pilot | github.com/attilaszasz/sdd-pilot |
| 9 | `spec2ship` | spec2ship | github.com/spec2ship/spec2ship |
| 10 | `chatdev` | ChatDev | github.com/OpenBMB/ChatDev |
| 11 | `paul` | PAUL | github.com/ChristopherKahler/paul |
| 12 | `fspec` | fspec | github.com/sengac/fspec |
| 13 | `cognition_devin` | Cognition / Devin | docs.devin.ai |
| 14 | `microsoft_agent_framework` | Microsoft Agent Framework | github.com/microsoft/agent-framework |
| 15 | `tessl` | Tessl | docs.tessl.io |

**Removed during re-research:** `spec_driven_develop`, `whenwords`, `intent`,
`agentic_code`, `codespeak` — no canonical upstream / no public artifact
surface. See `docs/research/sdd-fingerprints/README.md` for per-tool reasons.

**Note on naming collision.** `bmad` already exists as a `framework` ID in `frameworks.json`. The new `sdd_detectors.json` uses the same canonical ID, and the legacy `frameworks.json` entry is left in place per C-004 (back-compat). Same applies to `spec_kit`, `spec_kitty`, and `openspec`.

## R-10 Decision: where ecosystem.go calls the evaluator

**Decision.** `analyzer.DetectEcosystem` is the only call site. It instantiates a `sdd.RealProbe`, calls `sdd.Evaluate(text, lines, probe, registry)`, and stores the result on `Ecosystem.WorkflowFingerprints`. `Ecosystem.WorkflowFrameworks` keeps its existing logic (legacy `frameworks.json`-based detection).

**Rationale.** One call site, no analyzer-wide refactor, no new public API surface for the analyzer package.

**Alternatives considered.**
- Calling the evaluator from `Analyze()`. Rejected — `DetectEcosystem` is the natural seam, and `Analyze` shouldn't know about CLI probing.

## Outstanding clarifications

None. All Phase 0 questions resolved.
