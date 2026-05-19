# Top-20 SDD Fingerprint Registry

| Field | Value |
| --- | --- |
| Mission ID | `01KRZEQ372C78Z0KGN69MXRXZV` |
| Mission slug | `top-20-sdd-fingerprint-registry-01KRZEQ3` |
| Mission type | `software-dev` |
| Target branch | `main` |
| Status | Draft |
| Source description | Brief at `../start-here.md` (Epic #38) |

## Purpose

**TL;DR.** Make the analyzer a privacy-safe profiler that deterministically identifies which of the top-20 spec-driven-development (SDD) tools are present and active in a Claude Code workspace, with zero raw private content leaving the local machine.

**Context.** Spec-driven development is becoming a crowded ecosystem and users carry growing piles of MCP servers, skills, and CLI tooling. The analyzer needs to know which named tools (Spec Kitty, GitHub Spec Kit, OpenSpec, Kiro, BMAD-METHOD, GSD, and the long tail) are actually configured — without ever exfiltrating private identifiers, paths, or content. This mission seeds a verified detector registry for the top-20 SDD tools so future bloat and adoption analytics rest on deterministic, aggregate-safe evidence rather than guessed names.

## User Scenarios & Testing

### Primary scenario — analyzer reports detected SDD tooling

1. **Actor.** An engineer running the analyzer over their local Claude Code workspace.
2. **Trigger.** They invoke the analyzer (CLI run, smoke test, or programmatic call) on a workspace that contains one or more SDD tools (configs, skills, MCP entries, generated artifacts, or installed CLI binaries).
3. **Action.** The analyzer scans sanitized inputs, runs allowlisted CLI presence/version probes, and matches evidence against the seeded detector registry.
4. **Success outcome.** The generated report contains, for each tool whose evidence threshold was reached, a typed `EcosystemFingerprint` record carrying an allowlisted public `id`, a deterministic confidence level, contributing source classes, an evidence count, optional `installed` boolean, and optional bounded `version_bucket`. No raw private string appears anywhere in the aggregate output.

### Exception path — workspace mixes Spec Kitty, GitHub Spec Kit, and OpenSpec markers

1. **Trigger.** The workspace happens to contain a generic `specs/` directory and a `tasks.md` file plus genuine Spec Kitty artifacts (e.g., `.kittify/`, Spec Kitty-specific slash command names).
2. **Expected behavior.**
   - Spec Kitty is detected because tool-specific markers are present.
   - GitHub Spec Kit is **not** detected because no GitHub Spec Kit-specific marker is present (generic `specs/` and `tasks.md` do not count by themselves).
   - OpenSpec is **not** detected for the same reason.
3. **Why it matters.** The product's trust story depends on the registry being boring and deterministic. Conflating these three would be a credibility-destroying false positive.

### Exception path — unknown private tool present

1. **Trigger.** The workspace contains an MCP server, skill, or plugin whose name is not in any allowlisted registry.
2. **Expected behavior.** The unknown item contributes to an integer "unknown count" bucket. Its name, identifier, and any derivable hash are not stored locally beyond the user's own logs and never appear in the aggregate report or aggregate event stream.

### Exception path — CLI present but raw path is sensitive

1. **Trigger.** An allowlisted public SDD CLI binary exists on `PATH`, resolved by `exec.LookPath` to a path that includes the user's home directory or a private project path.
2. **Expected behavior.** The fingerprint records `installed: true` and, if the optional version probe runs successfully, a bounded `version_bucket` (e.g., `"1.2"`). The resolved executable path, raw `--version` output, and any stderr text are never emitted into the report or aggregate event stream.

### Acceptance scenarios (capsule form)

- AS-01 Spec Kitty fixture → Spec Kitty detected; GitHub Spec Kit and OpenSpec not detected.
- AS-02 GitHub Spec Kit fixture → GitHub Spec Kit detected; Spec Kitty and OpenSpec not detected.
- AS-03 OpenSpec fixture → OpenSpec detected; Spec Kitty and GitHub Spec Kit not detected.
- AS-04 Fixture with only generic `specs/`, `tasks.md`, `design.md`, `STATE.md`, `hooks`, `requirements.md` → no SDD tool-specific detector fires.
- AS-05 Fixture with unknown MCP/skill/plugin names → unknown counts increment; names are absent from aggregate output.
- AS-06 Fixture with installed allowlisted CLI on `PATH` → `installed: true`, no resolved path in output.
- AS-07 CLI version probe returns free-form text containing a path and an email → normalized `version_bucket` is recorded; raw output is suppressed.
- AS-08 Serialization-leak test serializes a fully populated report and asserts none of the brief's forbidden raw strings appear.

## Domain Language

| Canonical term | Avoid (or use carefully) | Reason |
| --- | --- | --- |
| Spec Kitty | "the kitty tool", "Spec Kit" | Spec Kit refers to a different product (GitHub Spec Kit). Spec Kitty is this project's own family. |
| GitHub Spec Kit | "Spec Kit", "spec-kit" | Distinct product. Must never be conflated with Spec Kitty. |
| OpenSpec | "open spec", "openspec.md" | Distinct product. Must not be conflated with Spec Kitty or GitHub Spec Kit. |
| SDD tool | "agentic tool", "framework" | "SDD tool" is the registry's scope; broader categories live elsewhere. |
| Source class | "evidence type", "signal" | Source class is an enum-like, typed value (e.g., `config_dir`, `mcp_server_name`). |
| Confidence level | "score", "probability" | Confidence is a discrete `high`/`medium`/`low` value driven by deterministic rules. |
| Version bucket | "version", "semver" | A bounded normalized form of a version string, not the raw output. |
| Detector status | "state", "phase" | Discrete value among `verified`, `research_needed`, `blocked`. |
| Unknown count | "unknown name", "private name" | Always an integer; names are never stored or uploaded. |

## Functional Requirements

| ID | Status | Requirement |
| --- | --- | --- |
| FR-001 | Proposed | The analyzer MUST detect Spec Kitty as a distinct tool using verified public fingerprints from official sources. |
| FR-002 | Proposed | The analyzer MUST detect GitHub Spec Kit as a distinct tool. A Spec Kitty fixture MUST NOT trigger GitHub Spec Kit detection and vice versa. |
| FR-003 | Proposed | The analyzer MUST detect OpenSpec as a distinct tool. An OpenSpec fixture MUST NOT trigger Spec Kitty or GitHub Spec Kit detection and vice versa. |
| FR-004 | Proposed | The analyzer MUST provide a typed detector schema with fields `id`, `display_name`, `aliases`, `category`, `competitor_priority`, `status`, `source_references`, source-class entries, confidence rules, and evidence-count rules. |
| FR-005 | Proposed | The registry MUST seed entries for the final list of 15 SDD tools that reached `verified` status after the post-mission re-research pass: Spec Kitty, GitHub Spec Kit, OpenSpec, Kiro, BMAD-METHOD, GSD, Spec Workflow MCP, SDD Pilot, spec2ship, ChatDev, PAUL, fspec, Cognition/Devin, Microsoft Agent Framework, Tessl. Five tools from the original top-20 brief (Spec-Driven Develop, whenwords, Intent, Agentic Code, CodeSpeak) were removed entirely because no canonical upstream or public fingerprintable artifact surface could be identified; the rationale is documented in `docs/research/sdd-fingerprints/README.md` under "Removed tools". A tool's "seed entry" is satisfied by a verified production detector in `internal/analyzer/signatures/sdd_detectors*.json` with citations in its per-tool research file. Per FR-013, production detectors only ship for tools whose detector record has `status: verified`. |
| FR-006 | Proposed | Each detector MUST express its evidence using the enumerated source classes: `config_dir`, `config_file`, `package_manifest`, `command_name`, `slash_command`, `mcp_server_name`, `skill_name`, `plugin_manifest`, `cli_binary`, `cli_version_probe`. |
| FR-007 | Proposed | The analyzer MUST assign one of three discrete confidence levels (`high`, `medium`, `low`) using deterministic rules: high = public marker + tool-specific marker, or official CLI + official artifact, or known MCP name + tool-specific config; medium = known CLI presence, known slash command usage, known MCP name, or package manifest dependency; low = allowlisted public textual mention only. |
| FR-008 | Proposed | The analyzer MUST probe only allowlisted public CLI binary names using `exec.LookPath` (or an injectable equivalent) and emit an `installed` boolean. The resolved executable path MUST NOT appear in any output, report, log line, or aggregate event. |
| FR-009 | Proposed | The analyzer MAY run allowlisted version commands (e.g., `--version`, `version`) with a short timeout, a sanitized environment, no stdin, no shell expansion, and no network access. Raw version output MUST be normalized into a bounded `version_bucket` (or dropped) before recording. |
| FR-010 | Proposed | The analyzer MUST emit `EcosystemFingerprint` records carrying `id`, `confidence`, `sources`, `evidence_count`, and optional `active`, `installed`, `version_bucket` fields. The `Ecosystem` type MUST expose these via a new `WorkflowFingerprints` collection while preserving the existing `WorkflowFrameworks []string` field. |
| FR-011 | Proposed | Unknown or private MCP / skill / plugin / tool names MUST be counted only. Their names, identifiers, and any derivable hashes MUST NOT appear in aggregate output or aggregate events. |
| FR-012 | Proposed | Generic file or directory names — `specs/`, `tasks.md`, `design.md`, `STATE.md`, `hooks`, `requirements.md` — appearing alone MUST NOT trigger any specific SDD tool detector. Tool-specific markers are required for a match. |
| FR-013 | Proposed | Detectors for tools that cannot be verified from public sources MUST carry status `research_needed`; production detectors MUST NOT be emitted for them. Per C-001 this mission ships zero detectors in that state. |
| FR-014 | Proposed | Every verified detector MUST record `source_references` pointing to the public artifacts (official repo, docs, release source) from which its fingerprints were derived. |
| FR-015 | Proposed | The detector status field MUST support the values `verified`, `research_needed`, and `blocked`. |
| FR-016 | Proposed | The CLI probe (`exec.LookPath` + version probe) MUST be expressed as an injectable abstraction (interface or function variable) so unit tests can simulate `LookPath` results and version output without executing real binaries. |

## Non-Functional Requirements

| ID | Status | Requirement | Measurable threshold |
| --- | --- | --- | --- |
| NFR-001 | Proposed | Aggregate report output and aggregate event stream MUST NOT contain forbidden raw strings (user prompts, task descriptions, raw transcripts, raw tool inputs/outputs, raw file paths, repo URLs, branch names, usernames, hostnames, emails, session IDs, transcript paths, private MCP / skill / plugin names, raw `which` / `exec.LookPath` paths, raw `--version` output, stable hashes of private strings). | Serialization-leak test asserts none of the listed string categories appear in any fully populated report or aggregate event payload. Test must cover at least one representative instance from each of the 16 forbidden categories. |
| NFR-002 | Proposed | CLI version probes MUST complete within a bounded timeout. | ≤ 2 seconds per binary, wall-clock, in test harness. Timeout causes the probe to record `installed: true` (if `LookPath` succeeded) with no version bucket; the analyzer continues. |
| NFR-003 | Proposed | Aggregate output cardinality MUST be bounded. | No JSON map whose keys are derived from raw user strings. All map keys are either allowlisted IDs from the registry or enum-like constants. Verified by a static structural test over a representative report. |
| NFR-004 | Proposed | Spec Kitty, GitHub Spec Kit, and OpenSpec each MUST have explicit cross-negative tests proving they do not trigger one another. | At minimum 3 dedicated test cases per direction (Spec-Kitty-only fixture, GitHub-Spec-Kit-only fixture, OpenSpec-only fixture) asserting only the intended detector fires, for a total of at least 9 cross-negative assertions. |

## Constraints

| ID | Status | Constraint |
| --- | --- | --- |
| C-001 | Accepted | All 20 named SDD tools MUST be `verified` from public sources before this mission is considered done. No detector ships in the `research_needed` state in this mission. |
| C-002 | Accepted | Fingerprints MUST be derived only from public sources (official repositories, public documentation, install/init flows in disposable directories, public demo fixtures). Private user logs and private repositories MUST NOT be inspected to discover fingerprints. |
| C-003 | Accepted | CLI probes MUST only target allowlisted public binary names from the registry. Unknown CLI names discovered from logs MUST NOT be probed. CLI probes MUST NOT contact networks, require auth, modify files, read project data, or start servers. |
| C-004 | Accepted | The work MUST extend the existing `internal/analyzer/signatures/` registry and `Ecosystem` type rather than replace them. The existing `WorkflowFrameworks []string` field MUST be preserved so existing report consumers (web UI, golden tests) keep working. |
| C-005 | Accepted | Low-confidence textual mentions MUST NOT count as installed tools in aggregate adoption metrics. They may surface in local-only views if useful. |
| C-006 | Accepted | The analyzer MUST remain locally deterministic. No dashboards, accounts, or cloud-dependent behavior may be introduced as part of this mission. |

## Success Criteria

- SC-01 The analyzer can distinguish Spec Kitty, GitHub Spec Kit, and OpenSpec on representative fixtures with zero cross-trigger false positives (verified by the cross-negative test suite from NFR-004).
- SC-02 An end user running the analyzer on a workspace containing the 20 named SDD tools sees, in the report, exactly one fingerprint record per tool that is present, each carrying a deterministic confidence level and a bounded set of source classes.
- SC-03 Serializing any fully populated report produces no occurrence of any of the 16 forbidden raw-string categories from NFR-001.
- SC-04 An end user with no SDD tools installed sees zero fingerprint records and no unknown-name leakage; the unknown count bucket may be `0`.
- SC-05 An end user with an installed but private/unknown MCP server, skill, or plugin sees the unknown count increment by 1 and never sees the private name in any aggregate output.
- SC-06 Researchers and reviewers can trace every verified detector to a public source reference recorded in the registry.

## Key Entities

- **SDD Detector** — a typed registry record describing how to identify one named SDD tool. Carries `id`, `display_name`, `aliases`, `category`, `competitor_priority`, `status`, `source_references`, a set of detector source-class entries (each with the markers it matches), and confidence rules. Status is `verified`, `research_needed`, or `blocked`.
- **Source Class** — an enum-like value that classifies a piece of evidence: `config_dir`, `config_file`, `package_manifest`, `command_name`, `slash_command`, `mcp_server_name`, `skill_name`, `plugin_manifest`, `cli_binary`, `cli_version_probe`.
- **Ecosystem Fingerprint** — the report-shaped record emitted per detected tool: `id`, `confidence`, `sources` (list of source-class IDs that contributed), `evidence_count`, optional `active`, `installed`, `version_bucket`. Carries no raw private content.
- **Confidence Level** — a discrete `high`/`medium`/`low` value derived from deterministic rules over the set of contributing source classes.
- **Version Bucket** — a normalized, bounded string derived from raw `--version` output (e.g., `"1.2"`, `"unknown"`, or absent). Never the raw output.
- **Unknown Count** — an integer bucket per category (MCP / skill / plugin / other) summarizing how many items the analyzer saw whose names are not in the allowlist. Never carries names or hashes of names.
- **CLI Probe** — an injectable abstraction wrapping `exec.LookPath` and a safe version-command runner. Implementations may be real or test fakes; the result objects carry only `installed`, `version_bucket`, and a contributing-source list.

## Assumptions

- A-01 The brief at `../start-here.md` is authoritative for scope, privacy stance, and the 20-tool list.
- A-02 The user's resolution that "all 20 must be verified" applies to this mission's Definition of Done. Tools that cannot be verified within mission scope become a scope/timeline issue, not a privacy or scope shortcut (i.e., we do not ship `research_needed` detectors to bypass the bar).
- A-03 The existing analyzer architecture (`internal/analyzer/signatures/`, `Ecosystem`, `AggregateSafeEvent`) is the right home for this work; replacing it is out of scope.
- A-04 Public sources are sufficient to verify all 20 tools' fingerprints, possibly with isolated install/init runs in disposable directories. If a tool genuinely has no public fingerprintable surface, scope will be revisited with the user before downgrading C-001.
- A-05 The new typed schema and the legacy `WorkflowFrameworks []string` field can co-exist on `Ecosystem` for the lifetime of this mission. Migration of legacy consumers is out of scope.
- A-06 Smoke testing via `./scripts/smoke-local.sh` is the project's smoke convention; if it is unrelated or blocked, the planning phase documents the exact reason and still runs `go test ./...`.

## Out of Scope (this mission)

- Issue #39 (MCP / skill bloat utilization analytics beyond unknown-count plumbing already present).
- Issue #40 (broader aggregate ecosystem intelligence beyond the fingerprint registry itself).
- Issue #41 (report UX surfacing of fingerprints; the report data shape must be ready, but UX work is separate).
- Issue #58 (bounded ecosystem aggregate fields beyond what NFR-003 requires for this registry).
- Issue #65 (CI privacy and cardinality gates beyond the tests that this mission introduces).
- Building dashboards, account systems, or any cloud-side ingest extensions.

## Dependencies

- DEP-01 Existing `internal/analyzer/signatures/` embedded JSON registries.
- DEP-02 Existing `internal/analyzer/registry.go`, `ecosystem.go`, `types.go`, `analyzer.go`, and the report serialization paths.
- DEP-03 Existing privacy invariants asserted in `analyzer_test.go` for unknown MCP/skill counts (these must continue to hold).
- DEP-04 Access to public sources (official repos, docs, release pages) for each of the 20 tools.
- DEP-05 `go test ./...` and `./scripts/smoke-local.sh` as the verification surfaces.

## Open Questions

None at spec time. (Brief-intake mode resolved the one open question via direct user decision: C-001 "all 20 must be verified.")
