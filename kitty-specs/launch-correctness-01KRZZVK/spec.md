# Launch Correctness Fixes — Specification

**Mission ID:** `01KRZZVKHQK6V1R4B0ZABWTFCJ`
**Slug:** `launch-correctness-01KRZZVK`
**Mission type:** software-dev
**Source brief:** `../start-here.md` (Phase 1: Correctness Bugs First)
**Scope GitHub issues:** [#74](https://github.com/robertDouglass/claude-log-analyzer/issues/74), [#70](https://github.com/robertDouglass/claude-log-analyzer/issues/70), [#72](https://github.com/robertDouglass/claude-log-analyzer/issues/72)

## Purpose

A skeptical developer should be able to install the Claude Log Analyzer CLI,
point it at one of their Claude Code log files, and trust that:

1. The tool analyzed the file they actually named — never silently substituted a
   different one.
2. The MCP and skill usage numbers in the resulting report reflect calls they
   actually made — not artifacts of how their tools advertise themselves.
3. When they buy the paid 100-log scan, every workflow fingerprint and tooling
   utilization signal that would appear in a single report survives the merge,
   with the same privacy guarantees applied across the aggregate.

Three correctness defects threaten that trust today. This mission fixes them as
one focused PR (`codex/launch-correctness`) so the rest of the launch sequence
ships on top of a correct foundation.

## User Scenarios & Testing

### Primary actors

- **Local developer (free flow):** runs `claude-analyzer analyze` against one
  Claude Code JSONL log and reads the resulting sanitized report locally or in
  the short-lived web view.
- **Paid pack purchaser (paid flow):** runs a 100-log scan that merges
  per-report signals into one aggregate report and a generated paid plugin
  artifact.

### Primary scenarios

**Scenario A — Developer analyzes a specific log file (FR-001, #74).**
The developer runs `claude-analyzer analyze ./some-session.jsonl` (no `--log`
flag). The tool analyzes exactly `./some-session.jsonl`, writes the sanitized
report, and never silently falls back to the latest log in
`~/.claude/projects`.

**Scenario B — Developer passes both positional and flag (FR-002, #74).**
The developer runs `claude-analyzer analyze ./some-session.jsonl --log ./other.jsonl`.
The tool refuses with a clear error naming the conflict and exits non-zero.

**Scenario C — Developer passes multiple positional paths (FR-003, #74).**
The developer runs `claude-analyzer analyze ./a.jsonl ./b.jsonl`. The tool
refuses with a clear error naming the unexpected extra argument and exits
non-zero.

**Scenario D — Developer reads MCP utilization on a log heavy with exposure
headers (FR-005, #70).** The log contains a system message whose MCP exposure
header advertises many `mcp__server__tool` identifiers, but the developer never
actually called any of those tools. The report shows zero MCP calls for that
session — not one per advertised identifier.

**Scenario E — Paid pack purchaser merges 100 reports (FR-007 / FR-008, #72).**
Per-report `WorkflowFingerprints` (e.g. Spec Kitty, GitHub Spec Kit, OpenSpec)
and `ToolingUtilization` (MCP and skill exposed/called counts, warning bands,
ratios) are present in each input. The aggregate paid report and generated
plugin artifact preserve those fingerprints and utilization signals across all
100 inputs. Unknown/private names remain counts only, never named.

### Exception / edge scenarios

- Positional path that does not exist on disk: same error treatment as the
  current `--log` invalid-path path (clear error, non-zero exit). No behavior
  regression for the existing `--log` flow.
- Empty paid scan (zero input reports): aggregate report contains empty
  fingerprint and utilization sections, but the section keys still exist and
  remain bounded-cardinality.
- Inputs to the paid merge that disagree on `version_bucket` for the same
  fingerprint ID: aggregate keeps the value only if all inputs agree, otherwise
  empties the field (the brief allows `mixed` only if enum-approved; this spec
  defers that enum decision until paid analytics design lands — see
  Assumptions).

### Playback

Primary scenario (free): a developer analyzes the file they named; the report
they read reflects that file's actual calls; their trust in the profiler is
preserved.

Primary exception (paid): a paid scan over 100 logs preserves every ecosystem
intelligence signal across the merge without ever leaking a private name.

Always-true rule: no input variation (CLI args, exposure-header content, merge
cardinality) can cause the tool to silently analyze the wrong log, double-count
header tokens as calls, or strip ecosystem intelligence from a paid aggregate.

## Domain Language

The following terms are canonical for this mission. Code, tests, and docs must
use them consistently.

- **analyze** — the local CLI step that parses one Claude Code JSONL log into
  a sanitized report. Synonyms to avoid: *scan*, *audit*.
- **scan** — the paid 100-log flow that merges per-report signals into one
  aggregate report and a generated plugin artifact. Synonyms to avoid:
  *batch analyze*.
- **positional log path** — a non-flag argument supplied to `claude-analyzer
  analyze`. At most one is allowed; conflicts with `--log`.
- **MCP exposure header** — a byte range inside the parsed log where the agent
  enumerates MCP servers and their tools for the model's awareness. Tokens of
  the shape `mcp__server__tool` inside these byte ranges are advertisements,
  **never** calls.
- **MCP call** — a parsed tool-use record whose tool name matches
  `mcp__server__tool`, occurring outside any exposure-header byte range.
- **skill exposure** — analogous byte range for skill enumeration; the same
  exposure-vs-call distinction applies.
- **workflow fingerprint** — an entry under `Ecosystem.WorkflowFingerprints`
  identifying a public allowlisted SDD tool (Spec Kitty, GitHub Spec Kit,
  OpenSpec). Carries `id`, `sources`, `evidence_count`, `confidence`,
  `active`/`installed`, `version_bucket`.
- **tooling utilization** — entries under `Ecosystem.ToolingUtilization`
  describing MCP and skill exposed/called counts, ratios, context-footprint
  buckets, and warning bands.
- **warning band rank (high-to-low)** — severe > high > watch > normal >
  unknown.
- **confidence rank (high-to-low)** — high > medium > low.
- **bounded-cardinality field** — a field whose value set is a closed enum, an
  allowlisted ID, a bounded bucket, or a numeric count. The mission must not
  introduce any field outside this shape.
- **private name** — any MCP server name, skill name, plugin name, or
  identifier not on the public allowlist. Counted only; never stored, logged,
  uploaded, hashed, or shown.

## Functional Requirements

| ID      | Description | Status |
|---------|-------------|--------|
| FR-001  | When the user passes exactly one positional log path and no `--log` flag, `claude-analyzer analyze` treats the positional path as the input file and analyzes it. | Required |
| FR-002  | When the user passes both a positional log path and `--log`, the CLI exits non-zero with a clear error naming the conflict between `--log` and the positional argument, and does not analyze any file. | Required |
| FR-003  | When the user passes more than one positional log path, the CLI exits non-zero with a clear error naming the unexpected extra argument(s), and does not analyze any file. | Required |
| FR-004  | The `claude-analyzer analyze --help` usage text and the public README/docs (README.md, docs/testing-plan.md, web command-generator copy) document the positional log path as a supported form alongside `--log`. | Required |
| FR-005  | MCP and skill call counts in any produced report reflect only parsed tool-use records whose byte offsets fall outside any MCP or skill exposure-header byte range. Tokens of the shape `mcp__server__tool` (or analogous skill identifiers) appearing inside exposure-header byte ranges contribute zero to call counts and zero to utilization ratios. | Required |
| FR-006  | Golden fixtures cover at least one case where an exposure header contains many `mcp__server__tool` tokens and the underlying session has zero actual calls; the analyzer reports zero calls for that fixture. | Required |
| FR-007  | The paid aggregate merge preserves `Ecosystem.WorkflowFingerprints` across all input reports by ID: `sources` is the union of source enums; `evidence_count` is the sum across inputs; `confidence` is the max by confidence rank; `active` and `installed` are the boolean OR across inputs; `version_bucket` is retained when all inputs agree on the same allowlisted value, otherwise emptied. | Required |
| FR-008  | The paid aggregate merge preserves `Ecosystem.ToolingUtilization` (MCP and skill) across all input reports: known public IDs are unioned; unknown counts are summed; exposed counts, call counts, and execution counts are summed; warning band is the max by band rank; utilization buckets and context-footprint buckets are recomputed from summed counts when feasible, else held to the max by bucket rank. | Required |
| FR-009  | The generated paid plugin artifact consumes the merged `WorkflowFingerprints` and `ToolingUtilization` (not only the classic ecosystem fields) and uses them as the basis for paid pack guidance. | Required |
| FR-010  | The PR opened for this mission posts a "starting work" comment on issues #74, #70, and #72, and a "ready for review" comment on the same issues when the PR is ready, naming files changed, tests run, and remaining work (if any). | Required |

## Non-Functional Requirements

| ID       | Description | Status |
|----------|-------------|--------|
| NFR-001  | The full charter verification baseline passes locally before the PR is opened: `gofmt -w $(find . -name '*.go' -not -path './.git/*')`, `go test ./...` (zero failures), `go vet ./...` (zero issues), `terraform -chdir=infra/aws fmt -check -recursive` (clean), `./scripts/smoke-local.sh` (exit 0). | Required |
| NFR-002  | Privacy leak tests confirm that after multi-report aggregation, zero private MCP names, private skill names, private plugin names, raw paths, repo URLs, branch names, usernames, hostnames, emails, session IDs, transcript paths, MCP server URLs, raw `which`/`exec.LookPath` paths, raw version command output, or stable hashes of private strings appear in: (a) the merged paid report JSON, (b) the generated paid plugin artifact, (c) any server-bound aggregate event payload. | Required |
| NFR-003  | After the #70 fix is in place, the MCP exposure-header call false-positive rate measured across the full bundled golden fixture set drops to exactly 0 (no fixture reports a call that is actually a header token). | Required |
| NFR-004  | The CLI changes for #74 do not increase the time-to-completion of a single-log `claude-analyzer analyze` invocation by more than 5% on the bundled `internal/analyzer/testdata` fixtures (measured as wall-clock of the existing CLI integration test). | Required |
| NFR-005  | The aggregate merge for #72 completes in under 5 seconds for a 100-input paid scan composed of the largest bundled golden fixtures, on a developer laptop equivalent to the CI runner. | Required |

## Constraints

| ID    | Description | Status |
|-------|-------------|--------|
| C-001 | No expansion of the upload schema or the paid aggregate schema beyond bounded-cardinality fields (allowlisted public IDs, closed enums, bounded buckets, numeric counts, recommendation/install-policy/confidence/source-class enums). This mission may add fields only if they fit that shape. | Required |
| C-002 | Charter privacy stance applies without exception: no private name, raw path, raw URL, raw transcript fragment, or stable hash of a private string may appear in any new field, log line, test output, or merged artifact introduced by this mission. | Required |
| C-003 | All implementation lands on a single branch `codex/launch-correctness` (timestamp suffix if the name is already taken on the remote) and is delivered as one PR that merges into `main`. | Required |
| C-004 | GitHub issues #74, #70, #72 are closed only when their per-issue acceptance criteria from start-here.md are demonstrably satisfied. Parent epics (#38, #39, #40, #41) remain open if any sibling acceptance item is unmet. | Required |
| C-005 | No Terraform `apply` is performed as part of this mission. Infra files are not expected to change; if they do, the PR ships only with `terraform plan` output reviewed first per the charter deployment policy. | Required |
| C-006 | The fix for #70 must not change the observable shape of `Ecosystem.ToolingUtilization` for any fixture that has zero exposure-header tokens (i.e. the change must be a strict no-op on fixtures where double-counting cannot occur). This preserves stability of golden reports that already exist. | Required |
| C-007 | `evidence_count` aggregation in FR-007 uses **sum** across inputs. If `max` is later judged a better fit, the change is deferred to a follow-up that updates spec and golden fixtures together; this mission does not switch semantics mid-flight. | Required |

## Success Criteria

These outcomes are measurable, user-facing, and technology-agnostic.

- **SC-1 (free flow correctness):** Across the full bundled CLI integration
  test set, the analyzer always analyzes the input the user named. There is
  no test invocation where the analyzed file differs from the user's
  positional argument or `--log` value.
- **SC-2 (free flow clarity):** Every invalid CLI argument combination from
  FR-002 and FR-003 produces an error message that names which argument
  conflicts with which other argument. Zero invalid invocations exit zero.
- **SC-3 (free flow MCP fidelity):** On every bundled golden fixture, the
  reported MCP call count equals the number of parsed tool-use records whose
  byte offsets fall outside exposure-header ranges. There is no fixture where
  the reported count exceeds the true count by even one call.
- **SC-4 (paid flow completeness):** For a paid 100-log scan composed of
  bundled fixtures covering diverse workflow fingerprints and utilization
  bands, every input fingerprint ID appears in the merged report, and the
  union of all input MCP/skill known-IDs appears in the merged
  `ToolingUtilization`.
- **SC-5 (paid flow privacy):** The privacy leak tests in NFR-002 pass on
  the merged paid report, the generated paid plugin artifact, and any
  server-bound aggregate event payload produced by the paid scan flow.
- **SC-6 (no regression):** Every test that was green at start-here.md
  baseline (`go test ./...`, `./scripts/smoke-local.sh`, CI workflows)
  remains green after the mission.

## Key Entities

- **Log file (input):** a Claude Code JSONL log on the user's local
  filesystem; raw contents never leave the machine.
- **Sanitized report:** the JSON output of `claude-analyzer analyze`,
  containing only bounded-cardinality public signals.
- **MCP exposure header:** a byte range inside the parsed log identifying
  where MCP advertisements live; produced or extended by this mission's
  parser changes.
- **Skill exposure header:** analogous byte range for skills.
- **Workflow fingerprint:** identity record for a detected public SDD tool,
  with `id`, `sources`, `evidence_count`, `confidence`, `active`,
  `installed`, `version_bucket`.
- **Tooling utilization entry:** per-tool record under `ToolingUtilization`
  describing exposed/called counts, utilization ratio, context footprint
  bucket, and warning band.
- **Paid aggregate report:** the JSON produced by merging N per-report
  inputs in the paid 100-log scan flow.
- **Paid plugin artifact:** the generated zip used by the paid install
  flow; consumes the paid aggregate report as input.

## Assumptions

- The launch handoff in `start-here.md` is the authoritative source for
  per-issue acceptance criteria. Where this spec is silent, that document
  governs.
- The current `mcpExposure` type already records server-centric exposure;
  this spec assumes equivalent or analogous data exists or can be added for
  skill exposure (`skillExposure`) without a schema migration. If schema
  change is required, it is captured during `/spec-kitty.plan`, not here.
- The paid 100-log scan flow already exists as a local-first sanitized
  upload path (per current state summary in start-here.md). This mission
  extends its merge logic; it does not change the upload contract.
- The `version_bucket` enum is already finalized and shared between
  per-report and aggregate consumers. If a `mixed` value is required for
  multi-input disagreement, that enum change is a separate follow-up.
- Developer laptops used for NFR-005 measurement are roughly equivalent to
  the GitHub Actions runner used in CI for time-budget comparison.

## Open Decisions (Deferred)

None at this time. If aggregation semantics for `evidence_count` (sum vs
max) are revisited, they will land in a follow-up mission per C-007.

## Out of Scope

- Phase 2 work (report UX for ecosystem fingerprints and tooling
  utilization — issues #62, #63, #41).
- Phase 3 work (recommendation Phase B — issues #73, #56, #64).
- Phase 4 work (paid pack completion beyond aggregate-merge correctness —
  issues #24, #27, #30, #31, #33).
- Phase 5 work (privacy analytics gates — issues #58, #59, #60, #61, #65).
- Phase 6 work (cloud launch hardening — issues #25, #36, #37).
- Trust and distribution changes (signed releases, hostname rename, WASM
  demo — issues #34, #36, #37).
- Any infrastructure `apply`, AWS resource changes, or DNS changes.

## References

- Launch brief: `../start-here.md` (Phase 1: Correctness Bugs First).
- Charter: `.kittify/charter/charter.md`.
- Issues:
  [#74](https://github.com/robertDouglass/claude-log-analyzer/issues/74),
  [#70](https://github.com/robertDouglass/claude-log-analyzer/issues/70),
  [#72](https://github.com/robertDouglass/claude-log-analyzer/issues/72),
  with parent epics [#38](https://github.com/robertDouglass/claude-log-analyzer/issues/38),
  [#39](https://github.com/robertDouglass/claude-log-analyzer/issues/39),
  [#40](https://github.com/robertDouglass/claude-log-analyzer/issues/40),
  [#41](https://github.com/robertDouglass/claude-log-analyzer/issues/41)
  remaining open until acceptance is fully demonstrated.
