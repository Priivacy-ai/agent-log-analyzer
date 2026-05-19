---
work_package_id: WP01
title: 'Foundation: typed registry schema + loader'
dependencies: []
requirement_refs:
- FR-004
- FR-010
- FR-015
- FR-016
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-top-20-sdd-fingerprint-registry-01KRZEQ3
base_commit: 85accd889e9c02ea99b26c2d87c9826097bbc83e
created_at: '2026-05-19T07:26:07.977950+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
phase: Phase 1 — Foundation
agent: "claude:opus-4.7:reviewer-renata:reviewer"
shell_pid: "4493"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/
execution_mode: code_change
owned_files:
- internal/analyzer/sdd/detector.go
- internal/analyzer/sdd/registry.go
- internal/analyzer/sdd/detector_test.go
- internal/analyzer/signatures/sdd_detectors_base.json
- internal/analyzer/types.go
role: implementer
tags: []
---

# Work Package Prompt: WP01 — Foundation: typed registry schema + loader

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

If no profile is specified, run `spec-kitty agent profile list` and select the best
match for this work package's `task_type` and `authoritative_surface`.

---

## Branch Strategy

- **Planning/base branch at prompt creation**: `main`
- **Final merge target for completed work**: `main`
- **Implementation branch (per brief)**: `codex/sdd-fingerprint-registry` — created in WP10.
- **Actual execution workspace is resolved later** by `/spec-kitty.implement` (lane worktree).
- If human instructions contradict these fields, stop and resolve before coding.

## Objectives & Success Criteria

Land the typed registry foundation:

- `SDDDetector`, `SourceClass`, `Confidence`, `Status`, `Marker`, `ConfidenceRule`, `SourceRef` types in a new `internal/analyzer/sdd/` package.
- An embedded loader that globs `internal/analyzer/signatures/sdd_detectors*.json` and concatenates entries from every matching tier file (`sdd_detectors_base.json` ships in WP01 as the empty placeholder; later WPs add `_first_class.json`, `_second_ring.json`, `_long_tail.json`). Startup validation: enum check, regex compilation, allowlisted-binary check, `status == "verified"` ⇒ `len(SourceReferences) >= 1`, deny-listed `version_args` rejected.
- A new `EcosystemFingerprint` type in `internal/analyzer/types.go` (placed there, not in `sdd`, to avoid import cycle with `analyzer.Ecosystem`).
- A new `WorkflowFingerprints []EcosystemFingerprint` field on `analyzer.Ecosystem` (omitempty).
- All existing tests continue to pass; new sdd loader test passes.

**Success criteria**:

- `go build ./...` succeeds.
- `go test ./...` passes with no behavior change to existing reports.
- The loader panics on an invalid embedded registry at process startup (verified by a deliberately-bad test fixture loaded via an injected `[]byte`).

## Context & Constraints

- Read the spec: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md`.
- Read the plan: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/plan.md`.
- Read the data model: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md` (exact field shapes here).
- Read the schema: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/sdd-detector.schema.json`.
- C-004: `Ecosystem.WorkflowFrameworks []string` MUST remain present and unchanged.
- C-006: keep behavior deterministic; no network, no on-disk caching.
- The `EcosystemFingerprint` type lives on the report. Its fields are bounded; do not add free-text fields.

## Subtasks & Detailed Guidance

### Subtask T001 — `sdd/detector.go`

- **Purpose**: Define the registry record and its enumerated companion types.
- **Steps**:
  1. Create `internal/analyzer/sdd/detector.go`.
  2. Define:
     ```go
     type SourceClass string
     const (
         SourceConfigDir       SourceClass = "config_dir"
         SourceConfigFile      SourceClass = "config_file"
         SourcePackageManifest SourceClass = "package_manifest"
         SourceCommandName     SourceClass = "command_name"
         SourceSlashCommand    SourceClass = "slash_command"
         SourceMCPServerName   SourceClass = "mcp_server_name"
         SourceSkillName       SourceClass = "skill_name"
         SourcePluginManifest  SourceClass = "plugin_manifest"
         SourceCLIBinary       SourceClass = "cli_binary"
         SourceCLIVersionProbe SourceClass = "cli_version_probe"
     )

     type Confidence string
     const (
         ConfidenceHigh   Confidence = "high"
         ConfidenceMedium Confidence = "medium"
         ConfidenceLow    Confidence = "low"
     )

     type Status string
     const (
         StatusVerified        Status = "verified"
         StatusResearchNeeded  Status = "research_needed"
         StatusBlocked         Status = "blocked"
     )

     type SourceRef struct {
         Kind string `json:"kind"`
         URL  string `json:"url"`
         Note string `json:"note,omitempty"`
     }

     type Marker struct {
         SourceClass SourceClass    `json:"source_class"`
         Pattern     string         `json:"pattern,omitempty"`
         Binary      string         `json:"binary,omitempty"`
         VersionArgs []string       `json:"version_args,omitempty"`
         Negative    bool           `json:"negative,omitempty"`
         Note        string         `json:"note,omitempty"`
         compiled    *regexp.Regexp // private; populated by loader
     }

     type ConfidenceRule struct {
         Confidence              Confidence    `json:"confidence"`
         RequiresAnyOf           []SourceClass `json:"requires_any_of,omitempty"`
         RequiresAllOf           []SourceClass `json:"requires_all_of,omitempty"`
         RequiresDistinctClasses int           `json:"requires_distinct_classes,omitempty"`
     }

     type SDDDetector struct {
         ID                 string           `json:"id"`
         DisplayName        string           `json:"display_name"`
         Aliases            []string         `json:"aliases,omitempty"`
         Category           string           `json:"category"`
         CompetitorPriority int              `json:"competitor_priority"`
         Status             Status           `json:"status"`
         SourceReferences   []SourceRef      `json:"source_references"`
         Markers            []Marker         `json:"markers"`
         ConfidenceRules    []ConfidenceRule `json:"confidence_rules"`
     }
     ```
- **Files**: `internal/analyzer/sdd/detector.go` (new, ~110 lines).

### Subtask T002 — `sdd/registry.go` with embed loader + validation

- **Purpose**: Embed `signatures/sdd_detectors.json`, parse it, validate every detector at startup. Bad data ⇒ `panic` with a precise message.
- **Steps**:
  1. Create `internal/analyzer/sdd/registry.go`.
  2. Use the existing `signatureFS` (`//go:embed signatures/*.json` in `internal/analyzer/registry.go`) and add an exported helper in that file that reads every entry matching `signatures/sdd_detectors*.json` and returns the concatenated `[]byte` chunks. The sdd loader walks those bytes and parses each chunk as `[]SDDDetector`. The WP creates `signatures/sdd_detectors_base.json` containing `[]` so the glob always has at least one match (otherwise the build embed pattern fails).
  3. Validate:
     - Each `id` matches `^[a-z][a-z0-9_]{2,}$`.
     - `Status` is one of the three enum values.
     - For every `Marker`: `SourceClass` is one of the ten enum values; `pattern` is set unless `SourceClass ∈ {cli_binary, cli_version_probe}`, in which case `binary` is set.
     - For every `Marker` whose `SourceClass` is `cli_version_probe`, `VersionArgs` defaults to `["--version"]` if empty. WP01 implements the full deny-list helper in `registry.go`: reject any arg in `{--config, --registry, --token, --server, --login}` and any arg containing `/`. (Earlier drafts deferred this to WP02; the deny-list is now WP01's responsibility.)
     - Every `Marker.Pattern` compiles via `regexp.Compile`.
     - If `Status == "verified"`, `len(SourceReferences) >= 1`.
  4. Memoize the parsed registry with `sync.Once` (mirror the existing pattern in `internal/analyzer/registry.go`).
- **Files**: `internal/analyzer/sdd/registry.go` (new, ~150 lines), `internal/analyzer/signatures/sdd_detectors.json` (new, `[]`).
- **Notes**: The existing `signatureFS` lives in `internal/analyzer/registry.go`. Either (a) add a sibling exported function in that file to read `signatures/sdd_detectors.json` and return `[]byte`, then have `sdd/registry.go` parse it; or (b) introduce a second `//go:embed` inside the `sdd` package pointing at a copy of the JSON. Option (a) keeps a single source of truth — recommended.

### Subtask T003 — `EcosystemFingerprint` + `WorkflowFingerprints` field

- **Purpose**: Surface fingerprints on the report without creating an import cycle.
- **Steps**:
  1. In `internal/analyzer/types.go`, add:
     ```go
     type EcosystemFingerprint struct {
         ID            string   `json:"id"`
         Confidence    string   `json:"confidence"`
         Sources       []string `json:"sources"`
         EvidenceCount int      `json:"evidence_count"`
         Active        bool     `json:"active,omitempty"`
         Installed     bool     `json:"installed,omitempty"`
         VersionBucket string   `json:"version_bucket,omitempty"`
     }
     ```
  2. Add a new field to `Ecosystem`:
     ```go
     WorkflowFingerprints []EcosystemFingerprint `json:"workflow_fingerprints,omitempty"`
     ```
  3. Do NOT modify the existing fields on `Ecosystem`.
- **Files**: `internal/analyzer/types.go` (edit).

### Subtask T004 — `detector_test.go`

- **Purpose**: Guard the validator. Tests run on synthetic in-memory JSON (do not depend on the embedded empty file).
- **Steps**:
  1. Create `internal/analyzer/sdd/detector_test.go`.
  2. Export a package-internal helper `parseDetectors([]byte) ([]SDDDetector, error)` for tests; the main loader wraps it and panics.
  3. Tests:
     - Valid minimal entry round-trips.
     - Invalid `id` (uppercase, dashes, too short) → error.
     - Invalid `status` value → error.
     - Unknown `source_class` → error.
     - `Marker` with `cli_binary` source class but no `binary` field → error.
     - `Marker` with non-CLI source class but no `pattern` field → error.
     - Bad regex `pattern` → error.
     - `Status == "verified"` with empty `source_references` → error.
     - `version_args` containing `/` or any deny-listed flag → error.
- **Files**: `internal/analyzer/sdd/detector_test.go` (new, ~180 lines).

### Subtask T005 — gofmt + green tests

- **Purpose**: Confirm the foundation compiles and the rest of the analyzer behaves unchanged.
- **Steps**:
  1. `gofmt -w $(find . -name '*.go' -not -path './.git/*')`.
  2. `go vet ./...`.
  3. `go test ./...`.
  4. Confirm the existing golden test in `internal/analyzer/golden_test.go` still passes (it shouldn't see `WorkflowFingerprints` because the field is `omitempty` and we haven't wired the evaluator yet).
- **Files**: none modified; report results in the activity log.

## Test Strategy

Tests are required: see T004 above. No table-driven testing convention is enforced beyond Go idiom. Use `t.Run` for sub-cases. Reuse existing assertion helpers in `internal/analyzer/test_helpers_test.go` only if needed.

## Risks & Mitigations

- **Import cycle** between `analyzer` and `sdd` — mitigated by putting `EcosystemFingerprint` in the `analyzer` package.
- **Embed path collisions** — mitigated by reusing `signatureFS` via a single-source-of-truth helper.
- **Loader panic at startup with empty registry** — mitigated by accepting `[]` as valid (no detectors). The validation only fires per entry.

## Review Guidance

- The reviewer should verify the deliberately-bad-input test cases all produce errors (T004).
- Confirm no field on `EcosystemFingerprint` carries free text or maps; only the seven fields specified.
- Confirm `WorkflowFrameworks []string` is unchanged.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T07:26:09Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=1018 – Assigned agent via action command
- 2026-05-19T07:29:56Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=1018 – Foundation types, loader, EcosystemFingerprint, glob across sdd_detectors*.json
- 2026-05-19T07:30:22Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=4493 – Started review via action command
- 2026-05-19T07:31:52Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=4493 – Review passed: types match data-model.md; loader globs sdd_detectors*.json; validators cover all required failure modes; EcosystemFingerprint is bounded; tests green.
