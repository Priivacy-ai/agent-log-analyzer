# Phase 0: Research — Launch Correctness Fixes

Grounded reading of the current code for each of the three bug sites, the chosen fix shape, and alternatives considered. Every claim names a file path.

## Bug #74 — CLI silently ignores positional log path

### Current shape

- Parsing: `cmd/claude-analyzer/main.go:36..86` defines `runAnalyze(args []string)`. It uses **stdlib `flag`** via `flag.NewFlagSet("analyze", flag.ContinueOnError)`, then `fs.Parse(args)`.
- Inputs registered: `--log`, `--out`, and other analyze options.
- After `fs.Parse(args)`, the code reads the `--log` value; if empty, it calls `latestClaudeLog()` (`main.go:143..175`) which globs `~/.claude/projects/*.jsonl` by mtime.
- `fs.Args()` (the positional residue) is **not consulted anywhere**. Extra positional arguments are silently discarded.
- Usage text: `usage()` at `main.go:183` documents only `--log` and `--out`.
- No tests adjacent to `cmd/claude-analyzer/main.go`. The package has no `_test.go` files.

### Decision

Accept exactly one positional log path as an alias for `--log` when `--log` is empty. Refuse all other combinations with a clear, named error.

Match the spec's invariants:
- `len(fs.Args()) == 0` + `--log` empty → fall through to `latestClaudeLog()` (unchanged behavior).
- `len(fs.Args()) == 0` + `--log` set → use `--log` (unchanged behavior).
- `len(fs.Args()) == 1` + `--log` empty → treat the positional as the log path (FR-001, NEW behavior).
- `len(fs.Args()) == 1` + `--log` set → refuse with error naming the conflict and exit non-zero (FR-002, NEW behavior).
- `len(fs.Args()) >= 2` → refuse with error naming the unexpected extra argument(s) and exit non-zero (FR-003, NEW behavior).

### Alternatives considered

1. **Cobra migration**: rewrite the CLI on Cobra and use `cobra.ExactArgs`/`MaximumNArgs`. **Rejected** — adds a non-stdlib dependency for a four-line fix; violates `locality-of-change` and `easy-to-change` (charter tactics). Mission scope is correctness, not framework refactor.
2. **Always treat positional as override (even when `--log` is set)**: tempting because it feels "natural", but it lets a stale `--log` in a script silently win or lose depending on argument order. The brief's recommended behavior is explicit refusal on conflict.
3. **Treat extra positionals as a list to scan in sequence**: out of scope — there is no batch-analyze CLI verb today, and adding one is a Phase 4 / paid-scan concern, not a correctness fix.

### Risk surface

- `latestClaudeLog()` semantics unchanged. Existing scripted users who never pass a positional argument see zero behavior change.
- The new CLI error path needs to set the same exit code currently used for "invalid input" elsewhere in `runAnalyze` (non-zero).

## Bug #70 — MCP call detector double-counts exposure-header tokens

### Current shape

- File: `internal/analyzer/tooling_detect.go`.
- Struct `mcpExposure` (`tooling_detect.go:19..26`): `KnownIDs []string`, `UnknownCount int`, `ExposedToolCount int`, `SchemaTextBytes int`, `ExposedToolKnown bool`, `InferenceSource string`. **No byte offsets** for the header block.
- `detectMCPExposureFromHeaders` (around `tooling_detect.go:114`): regex `mcpCallPairRe.FindAllSubmatch(block, -1)` scans the matched header block to count exposed tools. Header block boundaries are computed locally and discarded.
- `detectMCPCallsFromToolUse` (`tooling_detect.go:212..271`): the bug site.
  - `line 242`: `mcpCallPairRe.FindAllSubmatch(rawBytes, -1)` scans the **entire raw input** for `mcp__server__tool` tokens and counts each match as one MCP call. Tokens that live inside an exposure-header block (already accounted for as *exposures*, not calls) are double-counted here as *invocations*.
  - `lines 250..263`: also adds parsed tool-use records whose `ToolName` starts with `mcp__`. This path is correct and not the bug.
- Parallel `skillExposure` struct (`tooling_detect.go:28..35`) uses bullet-style entries only, so the regex-rescan bug is MCP-specific today. But the same mask-out approach should apply to `skillExposure` for defense-in-depth (charter `premortem-risk-identification`: a future skill exposure-header schema change could reintroduce the same class of bug).

### Decision

1. Add a `HeaderRanges []byteRange` field to `mcpExposure` (and an analogous field to `skillExposure`), where `byteRange = struct { Start, End int }`. These are **in-memory only** (never serialized) — they live alongside `mcpExposure` during detection and are discarded before the report is written.
2. During exposure header detection, record the matched header block's byte offsets into `HeaderRanges`.
3. In `detectMCPCallsFromToolUse`, before counting raw-byte matches, **mask** any match whose start offset falls inside any recorded header range. The masked counter ignores those tokens.
4. The parsed-line scan (`ToolName` startswith `mcp__`) needs no change — it doesn't see header text.

### Alternatives considered

1. **Replace raw-byte regex scan with parsed-line scan only**: cleaner, but the comment at `tooling_detect.go:242` exists because parsed-line coverage was historically incomplete. Removing the raw rescan risks regressing call counts for legacy log shapes. Mask-then-rescan keeps the safety net and fixes the bug.
2. **Strip header blocks from the input before the rescan (zero them out)**: same observable result, but mutates the parsed bytes and risks shifting offsets used elsewhere downstream. Mask-then-skip leaves the input immutable.
3. **Count exposures as calls deliberately, document it**: rejected — violates `language-driven-design` (the terms *exposure* and *call* must remain distinct) and contradicts FR-005.

### Risk surface

- Fixtures `00-empty.log` through `07-mixed-known-unknown.log` (`internal/analyzer/testdata/tooling/`): most contain no exposure-header tokens, so the change is a strict no-op (C-006 enforces this). Fixture `07-mixed-known-unknown.log` is the most likely to shift; updating its golden assertion is expected, not regression.
- A new fixture `08-header-only-zero-calls.log` (FR-006) provides the load-bearing assertion that proves the fix.

## Bug #72 — Paid aggregate merge drops fingerprints + utilization

### Current shape

- `AggregateReports(jobID string, reports []Report, inputSize int)` (`internal/analyzer/aggregate.go:8`) loops inputs and calls `mergeEcosystems(ecosystem, report.Ecosystem)` (`aggregate.go:30`).
- `mergeEcosystems` (`aggregate.go:128..143`) handles 13 classic ecosystem fields (Client, OS, Shell, MCPServersKnown, KnownSkills, KnownPlugins, package managers, version control, etc.) but **skips two newer fields**:
  - `Ecosystem.ToolingUtilization` (`types.go:65`, a `ToolingUtilization` containing `MCP MCPUtilization` and `Skill SkillUtilization`).
  - `Ecosystem.WorkflowFingerprints` (`types.go:66`, `[]EcosystemFingerprint`).
- Aggregate consumer paths:
  - `paidscan/bundle.go:44` invokes `AggregateReports` for paid scans; the result feeds `aggregateEvent` and the remediation `Generate()` pipeline.
  - `internal/remediation/artifact.go:502` calls `safeKnownEcosystem(report.Ecosystem)` but reads only simple string fields. After the merge is fixed, `toolingRecommendations` (`artifact.go:120`) will finally see merged ToolingUtilization values — which is the goal of FR-009.
- Golden coverage: `internal/analyzer/golden_test.go:55..59` nils both `WorkflowFingerprints` copies before golden comparison, indicating the fields exist but are not yet exercised in the merged path.
- Privacy canary: `internal/analyzer/leak_test.go` serializes the full `Report` and `AggregateEvent` and asserts no leak strings present. After the merge fix, the canary input must include private/unknown names so the assertion has bite.

### Decision

Extend `mergeEcosystems` to cover both fields using the FR-007 / FR-008 semantics encoded in `spec.md`:

- **WorkflowFingerprints**: merge by `id`. `sources` unioned. `evidence_count` summed (C-007). `confidence` held to max by `confidence rank` (high > medium > low). `active`, `installed` OR'd. `version_bucket` retained when all inputs agree, otherwise emptied (deferring `mixed` enum addition).
- **ToolingUtilization.MCP**:
  - `KnownServerIDs`, `UniqueKnownCalledIDs` unioned.
  - `UnknownServerCount`, `CallCount`, `KnownCallCount` summed.
  - `ServerCountBucket`, `ExposedToolCountBucket`, `ContextTokenBucket` recomputed from summed counts when bucket boundaries permit, else held to max-rank.
  - `UtilizationRatioPct` recomputed from `KnownCallCount / max(1, KnownExposedToolCount)`-style formula (using summed counts), or set to a deterministic placeholder when inputs disagree — exact formula is implementation detail confirmed in data-model.md.
  - `WarningBand` held to max by warning-band rank (severe > high > watch > normal > unknown).
- **ToolingUtilization.Skill**: analogous to MCP — `KnownExposedIDs`, `KnownExecutedIDs` unioned; `UnknownExposedCount`, `ExecutedCount` summed; `ContextEfficiencyBucket` recomputed or max-rank-held; `WarningBand` max-rank.

### Alternatives considered

1. **Take the most recent (last-write-wins) value**: rejected — discards information across N-1 inputs and breaks SC-4 (every input fingerprint ID must appear in the merged report).
2. **Drop fields entirely from aggregate output (status quo)**: rejected by the spec — FR-009 requires the paid plugin artifact to consume merged utilization data.
3. **`evidence_count = max` instead of sum**: the brief allowed either; spec.md C-007 chose sum. Switching is a follow-up that updates spec + plan + golden together.

### Risk surface

- Largest risk: privacy regression by accidentally retaining a private name during union semantics. Mitigation: every union operates on **public-allowlisted ID lists only**. Unknown counts merge as integers. Privacy canary in `leak_test.go` is extended to include private/unknown name strings in input reports and assert their absence in the merged output (NFR-002).
- Golden fixture drift: `golden_test.go:55..59` currently nils fingerprint fields. After the fix, that nilling stops; instead the test asserts the merged shape. This is a test-update, not a regression.

## Cross-cutting privacy ground truth

- The charter privacy stance lists 21 forbidden categories. The mission introduces zero new fields in serialized payloads. In-memory-only `HeaderRanges` cannot leak because it is never serialized.
- The aggregate merge changes operate on data that already exists in per-report serialization and already passes the privacy canary at the per-report layer. Extending the canary across the merged output is additive coverage, not new field surface.

## Decisions log (for traceable-decisions tactic)

| Decision | Choice | Trade-off accepted | Why now |
|----------|--------|--------------------|---------|
| Header range storage | In-memory `[]byteRange` on `mcpExposure`/`skillExposure` | Slight memory overhead per parse | Cleanest place to record + apply mask without changing serialized shape |
| Skill exposure mask | Implement defensively even though no current bug | Tiny amount of extra code | `premortem-risk-identification`: future skill exposure schema change could reintroduce identical bug class |
| `evidence_count` aggregation | sum | Loses "this was seen rarely vs often" granularity; max would preserve a "best evidence" signal | Brief allowed either; sum is the more conservative honest count; spec.md C-007 makes reversal a follow-up |
| `version_bucket` on disagreement | empty | A new `mixed` enum value would be more informative | Adding enum values is a schema decision belonging to the paid analytics design mission, not this one |
| CLI parser library | Keep stdlib `flag` | Cobra would centralize positional rules but adds a dep | `locality-of-change`: out of scope; four-line patch in existing parser site |

## Open `[NEEDS CLARIFICATION]`

None. All ambiguities resolved before Phase 1 begins (per plan governance guideline).
