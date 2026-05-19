---
work_package_id: WP02
title: Engine types, enums, and conflict resolution
dependencies:
- WP01
requirement_refs:
- FR-004
- FR-005
- FR-006
- FR-007
- FR-018
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T006
- T007
- T008
- T009
- T010
- T011
agent: claude
history:
- '2026-05-19': created from mission token-saving-recommendation-engine-phase-a-01KRZKCJ
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/token_saving_types
execution_mode: code_change
owned_files:
- internal/analyzer/token_saving_types.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading the rest of this prompt, load the assigned agent profile:

```text
/ad-hoc-profile-load implementer-ivan
```

Then continue with **Objective** below.

## Objective

Declare every enum, struct, and small helper that the engine and tests will
consume. **No business logic in this WP** — just types, constants, and two
deterministic helpers (`ToolStateMap.Resolve`, sort helpers). The output of
this WP is what makes WP01's registry typed and what WP03's engine logic and
WP04's tests can compile against.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. The implementing
agent rebases onto WP01's merged head (or the lane's resolved base) per
`lanes.json`. Read it via `spec-kitty agent context resolve --mission
token-saving-recommendation-engine-phase-a-01KRZKCJ --wp WP02 --json`.

## Context

Primary references:

- `data-model.md` §"Enums" and §"Entities" (definitive list of enum values
  and struct field shapes)
- `research.md` §4 "Tool-state conflict resolution" (precedence order for
  `ToolStateMap.Resolve`)
- `contracts/token_saving_engine_go_api.md` (exported names and JSON tags)

WP01's `token_saving_tools.go` already declared `type ToolID string`. WP02
does **not** redeclare it — it imports the package-private type from the
same package.

## Owned files

This WP owns and is the only writer of:

- `internal/analyzer/token_saving_types.go` (new)

**Do not edit** `token_saving_tools.go` (WP01's), `token_saving_recommendations.go`
(WP03's), any test file, or any pre-existing analyzer file.

## Implementation command

```bash
spec-kitty agent action implement WP02 --agent claude
```

---

### Subtask T006 — File scaffold

**Purpose.** Stand up the new file with a clear header.

**Steps.**

1. Create `internal/analyzer/token_saving_types.go` with `package analyzer`
   and a short comment naming the WP and the mission slug. State that this
   file holds Phase A token-saving enums and structs and is consumed by
   `token_saving_tools.go` and `token_saving_recommendations.go`.
2. Add the import block: `import ("sort")` (only — no third-party imports
   permitted in Phase A).

**Validation.** `go build ./...` after T011 lands; T006 alone produces an
empty-but-valid file.

---

### Subtask T007 — Declare every named enum type and its constants

**Purpose.** All eight enum types from data-model.md plus their constants.

**Steps.** In `token_saving_types.go`, declare:

```go
type ToolState string
const (
    ToolStateUnknown          ToolState = "unknown"
    ToolStateMentionedLow     ToolState = "mentioned_low"
    ToolStateInstalledMedium  ToolState = "installed_medium"
    ToolStateConfiguredMedium ToolState = "configured_medium"
    ToolStateActiveHigh       ToolState = "active_high"
    ToolStateRejectedMedium   ToolState = "rejected_medium"
)

type EvidenceSource string
const (
    EvidenceCLIPresence          EvidenceSource = "cli_presence"
    EvidenceCLIVersion           EvidenceSource = "cli_version"
    EvidenceLogActiveCommand     EvidenceSource = "log_active_command"
    EvidenceMCPConfigured        EvidenceSource = "mcp_configured"
    EvidenceMCPActive            EvidenceSource = "mcp_active"
    EvidencePluginConfigured     EvidenceSource = "plugin_configured"
    EvidenceSkillConfigured      EvidenceSource = "skill_configured"
    EvidenceHookConfigured       EvidenceSource = "hook_configured"
    EvidenceStatuslineConfigured EvidenceSource = "statusline_configured"
    EvidenceReportMention        EvidenceSource = "report_mention"
    EvidenceFailureOrRejection   EvidenceSource = "failure_or_rejection"
)
```

Repeat the same pattern (named type + `const ( … )` block) for `Signal`,
`RecommendationClass`, `Confidence`, `RiskLevel`, `InstallPolicy`, and
`Reason`. Use the exact string values from data-model.md — do not invent
new values.

**Files.** `token_saving_types.go` grows to ~200 lines after this step.

**Validation.** `go vet` clean; `goimports -l` outputs nothing.

---

### Subtask T008 — Declare `ToolStateEntry` and `ToolStateMap`

**Purpose.** The engine's input shape.

**Steps.**

1. Declare:

   ```go
   type ToolStateEntry struct {
       Tool    ToolID                 `json:"tool"`
       State   ToolState              `json:"state"`
       Sources map[EvidenceSource]int `json:"sources"`
   }

   type ToolStateMap map[ToolID]ToolStateEntry
   ```

2. Document in a comment that callers must populate `Sources` with bounded
   integer counts only — never with raw user data.

**Validation.** `go vet` clean.

---

### Subtask T009 — Implement `ToolStateMap.Resolve`

**Purpose.** Conflict-state precedence helper used by the engine.

**Steps.**

1. Add this method (note: receiver is on `ToolStateMap`, but the function
   itself is a pure two-input combiner — keeping it as a method is purely
   for discoverability):

   ```go
   func (ToolStateMap) Resolve(a, b ToolState) ToolState {
       // Precedence: rejected_medium > active_high > configured_medium >
       //             installed_medium > mentioned_low > unknown
       order := map[ToolState]int{
           ToolStateRejectedMedium:   5,
           ToolStateActiveHigh:       4,
           ToolStateConfiguredMedium: 3,
           ToolStateInstalledMedium:  2,
           ToolStateMentionedLow:     1,
           ToolStateUnknown:          0,
       }
       if order[a] >= order[b] {
           return a
       }
       return b
   }
   ```

2. Document the precedence in a comment block above the method, citing
   `research.md` §4 and `spec.md` "Edge Cases".

**Validation.** Covered by WP04's `TestRecommendStateConflictResolves` —
not in WP02's own test file (we have none in this WP by design).

---

### Subtask T010 — Deterministic helpers

**Purpose.** Pinning iteration order is the entire foundation of NFR-001.

**Steps.**

1. Add `func (m ToolStateMap) SortedTools() []ToolID` that returns a
   lexicographically sorted slice of the map's keys. Use `sort.Slice` with
   `string(keys[i]) < string(keys[j])`.
2. Add `func sortedSignalIDs(in []Signal) []Signal` that returns a copy
   sorted ascending by string value, with duplicates removed.

**Validation.** Covered indirectly by WP04's `TestRecommendDeterminism`;
add no separate test in WP02.

---

### Subtask T011 — Output structs + `EngineVersion`

**Purpose.** The engine's output contract, matching contracts/.

**Steps.**

1. Declare with JSON tags exactly as specified in
   `contracts/token_saving_engine_go_api.md`:

   ```go
   type TokenSavingRecommendation struct {
       RecommendationID string                 `json:"recommendation_id"`
       PrimaryToolID    ToolID                 `json:"primary_tool_id"`
       SkippedToolIDs   []ToolID               `json:"skipped_tool_ids,omitempty"`
       Reason           Reason                 `json:"reason"`
       SignalIDs        []Signal               `json:"signal_ids"`
       Confidence       Confidence             `json:"confidence"`
       RiskLevel        RiskLevel              `json:"risk_level"`
       InstallPolicy    InstallPolicy          `json:"install_policy"`
       EvidenceCounts   map[EvidenceSource]int `json:"evidence_counts"`
   }

   type SkipNote struct {
       ToolID    ToolID `json:"tool_id"`
       Reason    Reason `json:"reason"`
       ForSignal Signal `json:"for_signal"`
   }

   type RecommendationSet struct {
       Primary         *TokenSavingRecommendation `json:"primary,omitempty"`
       Secondary       *TokenSavingRecommendation `json:"secondary,omitempty"`
       Skipped         []SkipNote                 `json:"skipped,omitempty"`
       RegistryVersion string                     `json:"registry_version"`
       EngineVersion   string                     `json:"engine_version"`
       Signals         []Signal                   `json:"signals"`
       UnknownIDCount  int                        `json:"unknown_id_count"`
   }
   ```

2. Declare the package-private constant `const engineVersionString = "v0.1-phase-a"`
   and the exported function `func EngineVersion() string { return engineVersionString }`.

**Validation.** `go build ./...` succeeds; `go vet` clean. The structs are
exercised end-to-end by WP04.

---

## Definition of Done

- [ ] `internal/analyzer/token_saving_types.go` exists and contains every
      enum, struct, and helper listed above.
- [ ] `go vet ./internal/analyzer/` and `go build ./...` succeed against
      WP01+WP02 together.
- [ ] No edits to any other file in the repo.
- [ ] `gofmt -w internal/analyzer/token_saving_types.go` is clean.
- [ ] The file is < 400 lines.

## Risks & reviewer guidance

- The enum constant string values are part of the contract — a single typo
  silently breaks NFR-001 (determinism) and is hard to spot. The reviewer
  should diff every constant against `data-model.md` character by character.
- `Resolve` is the lynchpin of FR-018 — re-derive the precedence ordering
  during review against research.md §4.

## Out of scope for WP02

- Any code that actually emits a `TokenSavingRecommendation` (lives in WP03).
- Any test file (WP04 owns the engine tests).
- Any registry / `TokenSavingTool` work (WP01 owns the registry).
