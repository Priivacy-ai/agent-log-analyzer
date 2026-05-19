---
work_package_id: WP04
title: 'Wiring: DetectEcosystem, Bloat Findings, Aggregate'
dependencies:
- WP01
- WP02
- WP03
requirement_refs:
- FR-005
- FR-006
- FR-008
- FR-011
planning_base_branch: main
merge_target_branch: main
branch_strategy: Lane worktree branches from the merged head of WP01+WP02+WP03; lanes.json will resolve the correct base.
subtasks:
- T014
- T015
- T016
- T017
- T018
agent: claude
history:
- event: generated
  at: '2026-05-19T08:00:33Z'
  by: /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/ecosystem.go
execution_mode: code_change
mission_id: 01KRZJGVG3MCCCY9MKB1YRDBQR
mission_slug: mcp-skill-bloat-analytics-01KRZJGV
owned_files:
- internal/analyzer/ecosystem.go
- internal/analyzer/analyzer.go
- internal/analyzer/analyzer_test.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load the agent profile by invoking `/ad-hoc-profile-load` with `profile_id: "implementer-ivan"` and `role: "implementer"`. Then return here.

## Objective

Plumb the WP02 detectors and the WP03 classifier into the existing `Analyze` pipeline. After this WP, every report produced by `analyzer.Analyze` has a fully populated `Ecosystem.ToolingUtilization` with correct buckets, ratios, bands, and new bloat findings flow into `Report.ImmediateFixes`. The existing `AggregateSafeEvent` auto-receives the new fields because it embeds `Ecosystem` by value.

This is the integration WP — it touches the most files but adds the least new logic. The hard work was done in WP01–WP03; this WP wires it.

## Context

Read first:
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/spec.md` — FR-005, FR-006 (remediation gated on band), FR-008 (emit in Report + Aggregate), FR-011 (preserve existing immediate fixes).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md` — §D-1 (placement), §D-5 (remediation wiring via findings).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/research.md` — R-4 fixed remediation string set.
- `internal/analyzer/analyzer.go` — `Analyze()` (line 28), `buildFindings()` (line 303), `immediateFixes()` (line 387), `normalizeReportCollections()` (line 455), `aggregateEvent()` (line 437).
- `internal/analyzer/ecosystem.go` — `DetectEcosystem()` (line 15).
- `internal/analyzer/analyzer_test.go` — existing test expectations; you will need to update where Ecosystem literals are compared.

Branch contract:
- Planning base: `main`. Merge target: `main`. Lane base resolved from `lanes.json` (depends on WP01+WP02+WP03).

## Detailed Guidance

### Subtask T014 — Pipeline reorder so metrics precede ecosystem

**Purpose**: The band classifier needs `Metrics.Rereads`, `RetryDepthMax`, `ContextGrowthEvents` to gate `severe`. Currently `DetectEcosystem` runs before `computeMetrics`. Reorder so metrics are available.

**Steps**:
1. Open `internal/analyzer/analyzer.go`. Locate the `Analyze` function (line 28).
2. Current order (lines 39-41):
   ```go
   metrics, timeline := computeMetrics(lines)
   ecosystem := DetectEcosystem(scrubbed, lines)
   findings := buildFindings(metrics, lines)
   ```
3. The order is already `metrics → ecosystem → findings`. **Good news: no reorder needed**. Just confirm by reading the code. If the order is different by the time you reach this subtask, fix to match.
4. However, `DetectEcosystem` needs metrics. **Option A**: change its signature to `DetectEcosystem(scrubbed, lines, metrics)`. **Option B**: split detection: `DetectEcosystem` returns the basic ecosystem; a new `populateToolingUtilization(eco, metrics)` runs after metrics are computed.

   Use **Option B** to minimize merge friction with Epic #38 (which is also editing `DetectEcosystem`):
   - Keep `DetectEcosystem(scrubbed, lines)` signature unchanged.
   - Add new internal call: `ecosystem.ToolingUtilization = computeToolingUtilization(scrubbed, lines, metrics)`.
   - Put `computeToolingUtilization` in `ecosystem.go` (next to its only caller).
5. New order in `Analyze`:
   ```go
   metrics, timeline := computeMetrics(lines)
   ecosystem := DetectEcosystem(scrubbed, lines)
   ecosystem.ToolingUtilization = computeToolingUtilization(scrubbed, lines, metrics)
   findings := buildFindings(metrics, lines, ecosystem) // signature change — see T016
   ```

**Files**:
- `internal/analyzer/analyzer.go` (modify).

**Validation**:
- [ ] `Analyze` calls `computeMetrics` before `computeToolingUtilization`.
- [ ] `DetectEcosystem` signature is unchanged (Option B preserved).

### Subtask T015 — `computeToolingUtilization` orchestrator

**Purpose**: Build the orchestrator function that calls all WP02 detectors and the WP03 classifier and assembles a `ToolingUtilization` value.

**Steps**:
1. In `internal/analyzer/ecosystem.go`, add:
   ```go
   func computeToolingUtilization(input []byte, lines []parsedLine, metrics Metrics) ToolingUtilization {
       registry := ecosystemRegistry()

       // --- MCP ---
       mcpExp := detectMCPExposureFromHeaders(input, registry)
       mcpCalls := detectMCPCallsFromToolUse(input, lines, registry)
       mcpExposureKnown := mcpExp.InferenceSource == InferenceSourceHeader
       mcpInferenceSource := InferenceSourceNone
       serverCount := -1
       toolCount := -1
       if mcpExposureKnown {
           mcpInferenceSource = InferenceSourceHeader
           serverCount = len(mcpExp.KnownIDs) + mcpExp.UnknownCount
           if mcpExp.ExposedToolKnown {
               toolCount = mcpExp.ExposedToolCount
           }
       } else if mcpCalls.UniqueServerCount > 0 {
           mcpExposureKnown = true
           mcpInferenceSource = InferenceSourceCalls
           serverCount = mcpCalls.UniqueServerCount
           toolCount = mcpCalls.UniqueToolCount
       }
       mcpTokens, mcpTokensKnown := estimateMCPFootprintTokens(mcpExp.SchemaTextBytes, serverCount, toolCount)
       mcpRatio := 0
       if mcpExposureKnown && (serverCount > 0 || toolCount > 0) {
           denom := toolCount
           if denom <= 0 {
               denom = serverCount
           }
           numer := mcpCalls.UniqueServerCount  // unique callers as proxy for utilized
           if numer > denom {
               numer = denom
           }
           mcpRatio = numer * 100 / denom
       }
       mcpBand := classifyMCPBand(mcpBandInput{
           ServerCountBucket:      countBucket(maxInt(serverCount, 0), mcpExposureKnown),
           ExposedToolCountBucket: countBucket(maxInt(toolCount, 0), mcpExposureKnown && toolCount >= 0),
           ContextTokenBucket:     tokenBucket(mcpTokens, mcpExposureKnown && mcpTokensKnown),
           UtilizationRatioPct:    mcpRatio,
           ExposureKnown:          mcpExposureKnown,
           Rereads:                metrics.Rereads,
           RetryDepthMax:          metrics.RetryDepthMax,
           ContextGrowthEvents:    metrics.ContextGrowthEvents,
       })
       mcp := MCPUtilization{
           KnownServerIDs:           dedupeSorted(mcpExp.KnownIDs),
           UnknownServerCount:       mcpExp.UnknownCount,
           ServerCountBucket:        countBucket(maxInt(serverCount, 0), mcpExposureKnown),
           ExposedToolCountBucket:   countBucket(maxInt(toolCount, 0), mcpExposureKnown && toolCount >= 0),
           ContextTokenBucket:       tokenBucket(mcpTokens, mcpExposureKnown && mcpTokensKnown),
           ExposureKnown:            mcpExposureKnown,
           InferenceSource:          mcpInferenceSource,
           CallCount:                mcpCalls.TotalCalls,
           KnownCallCount:           mcpCalls.KnownCallCount,
           UnknownCallCount:         mcpCalls.UnknownCallCount,
           UniqueKnownCalledIDs:     mcpCalls.UniqueKnownIDs,
           UniqueUnknownCalledCount: mcpCalls.UniqueUnknownCount,
           UtilizationRatioPct:      mcpRatio,
           ContextEfficiencyBucket:  efficiencyBucket(mcpRatio, tokenBucket(mcpTokens, mcpExposureKnown && mcpTokensKnown), mcpExposureKnown),
           WarningBand:              mcpBand,
       }

       // --- Skill --- analogous, with detectSkillExposureFromHeaders, detectSkillExecutionsFromLines,
       // estimateSkillFootprintTokens, classifySkillBand. Same structure.
       // ... (assembled identically)

       return ToolingUtilization{MCP: mcp, Skill: skill}
   }

   func maxInt(a, b int) int { if a > b { return a }; return b }
   func dedupeSorted(xs []string) []string {
       if len(xs) == 0 { return []string{} }
       seen := map[string]bool{}
       out := make([]string, 0, len(xs))
       for _, x := range xs { if !seen[x] { seen[x] = true; out = append(out, x) } }
       sort.Strings(out)
       return out
   }
   ```
2. Make sure every slice ends up sorted and deduplicated. Empty slices (not nil) — handled by `dedupeSorted`'s `return []string{}` path.

**Files**:
- `internal/analyzer/ecosystem.go` (extend).

**Validation**:
- [ ] All struct fields are populated (no zero-valued strings where an enum is required).
- [ ] When no exposure signal exists, `mcp.ExposureKnown == false`, all `*Bucket` fields are `"unknown"`, ratio is 0, band is `"unknown"`.
- [ ] Slices are always non-nil (use `[]string{}`).

### Subtask T016 — Bloat findings in `buildFindings`

**Purpose**: Add four new `Finding` entries (`mcp_bloat_high`, `mcp_bloat_severe`, `skill_bloat_high`, `skill_bloat_severe`) that flow through the existing `immediateFixes` pipeline so their recommendation strings appear in `Report.ImmediateFixes` when the band fires.

**Steps**:
1. In `analyzer.go`, change `buildFindings` signature to accept `Ecosystem`:
   ```go
   func buildFindings(m Metrics, lines []parsedLine, eco Ecosystem) []Finding { ... }
   ```
2. Update the caller in `Analyze` (line 41 vicinity).
3. At the end of `buildFindings`, append bloat findings:
   ```go
   tu := eco.ToolingUtilization
   appendBand := func(id, title, sev string, rec string) {
       findings = append(findings, Finding{
           ID: id, Title: title, Severity: sev, CostImpact: "medium-high",
           Evidence: FindingEvidence{Description: "Bloat band: " + sev},
           Recommendation: rec, Deterministic: true,
       })
   }
   switch tu.MCP.WarningBand {
   case "severe":
       appendBand("mcp_bloat_severe", "MCP tool surface severely underutilized", "high",
           "Disable unused MCP servers by default and lazy-load heavy MCP servers only when needed.")
   case "high":
       appendBand("mcp_bloat_high", "MCP tool surface underutilized", "medium",
           "Scope project-specific MCPs to project config instead of global config; prefer narrower MCP servers over all-tools-enabled setups.")
   }
   switch tu.Skill.WarningBand {
   case "severe":
       appendBand("skill_bloat_severe", "Skill surface severely underutilized", "high",
           "Move rarely used instructions out of always-loaded skill context; keep only high-signal skills in the default agent context.")
   case "high":
       appendBand("skill_bloat_high", "Skill surface underutilized", "medium",
           "Split general skills from project-specific skills.")
   }
   ```
4. Do NOT change existing finding IDs or recommendation strings — that would regress NFR-007.
5. Recommendation strings must come from the fixed FR-006/§D-5 set. No private content.

**Files**:
- `internal/analyzer/analyzer.go` (extend).

**Validation**:
- [ ] When `MCP.WarningBand == "high"`, `Report.Findings` contains `{ID: "mcp_bloat_high"}` and `Report.ImmediateFixes` contains its `Recommendation`.
- [ ] When `WarningBand == "normal"` or `"unknown"`, no bloat finding is emitted.
- [ ] Severity field is `"high"` for severe band, `"medium"` for high band.

### Subtask T017 — `normalizeEcosystemCollections` updates

**Purpose**: Ensure all new slice fields are non-nil (empty slice rather than nil) so JSON serialization is consistent. Matches the existing pattern at `analyzer.go:471`.

**Steps**:
1. In `normalizeEcosystemCollections` (analyzer.go:471), extend to normalize the new nested slices:
   ```go
   func normalizeEcosystemCollections(ecosystem *Ecosystem) {
       // ... existing nil-checks for KnownSkills, KnownPlugins, etc. ...

       // New (this mission):
       if ecosystem.ToolingUtilization.MCP.KnownServerIDs == nil {
           ecosystem.ToolingUtilization.MCP.KnownServerIDs = []string{}
       }
       if ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs == nil {
           ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs = []string{}
       }
       if ecosystem.ToolingUtilization.Skill.KnownExposedIDs == nil {
           ecosystem.ToolingUtilization.Skill.KnownExposedIDs = []string{}
       }
       if ecosystem.ToolingUtilization.Skill.KnownExecutedIDs == nil {
           ecosystem.ToolingUtilization.Skill.KnownExecutedIDs = []string{}
       }
   }
   ```

**Files**:
- `internal/analyzer/analyzer.go` (extend `normalizeEcosystemCollections`).

**Validation**:
- [ ] Marshaling an `Ecosystem` with default `ToolingUtilization` produces `"known_server_ids": []` and `"unique_known_called_ids": []` (not `null`).

### Subtask T018 — Update existing `analyzer_test.go` assertions

**Purpose**: Any existing test that compares `Ecosystem` literal-equal or asserts on its full JSON shape needs to be updated to include the new `ToolingUtilization` field (or to not depend on full-shape equality).

**Steps**:
1. Open `internal/analyzer/analyzer_test.go`. Grep for `Ecosystem{` and `ecosystem.` to find direct shape assertions.
2. For each occurrence:
   - If the assertion is field-by-field (e.g., `if got.MCPServersKnown != …`), no change needed.
   - If it compares whole structs (e.g., `reflect.DeepEqual(got.Ecosystem, want)`), either:
     - Update `want` to include a zero-value `ToolingUtilization` populated with normalized empty slices and `unknown` enums, OR
     - Switch to field-by-field assertion on the keys that test actually cares about.
3. Existing slash-command path-avoidance tests must still pass — they should, because WP02 reused the exact regex.
4. Existing privacy tests (unknown name leakage) must still pass — they should, because we only added fields containing counts and allowlist IDs.
5. Run `go test ./internal/analyzer/...` and fix any remaining failures.

**Files**:
- `internal/analyzer/analyzer_test.go` (modify only failing assertions).

**Validation**:
- [ ] `go test ./internal/analyzer/...` passes (full package).
- [ ] No existing test was removed (only updated).
- [ ] Existing privacy assertions still pass.

## Test Strategy

Tests are required (NFR-002, NFR-004, NFR-007). WP04 doesn't add new unit-test files; it updates existing tests to accommodate the new shape and relies on WP02/WP03 unit tests + WP05 golden tests for coverage of new behavior.

## Definition of Done

- [ ] `Analyze` calls `computeToolingUtilization` after `computeMetrics`.
- [ ] `Ecosystem.ToolingUtilization` is fully populated for every code path.
- [ ] Bloat findings appear in `Report.Findings` and their recommendations in `Report.ImmediateFixes` when band is `high` or `severe`.
- [ ] `normalizeEcosystemCollections` ensures all new slices are non-nil.
- [ ] `go test ./internal/analyzer/...` passes (full package — including WP01, WP02, WP03 tests).
- [ ] `gofmt -l internal/analyzer/ecosystem.go internal/analyzer/analyzer.go internal/analyzer/analyzer_test.go` returns empty.
- [ ] Existing privacy assertions (private name leakage in unknown count) still pass.

## Risks

- **Risk**: Changing `buildFindings` signature breaks callers in other test files or downstream code. **Mitigation**: grep for `buildFindings(` to find all callers; there should be exactly one (in `Analyze`). The function is package-internal.
- **Risk**: Subtle interaction with Epic #38 if it also edits `DetectEcosystem`. **Mitigation**: keep `DetectEcosystem` signature unchanged (Option B in T014). All new logic lives in `computeToolingUtilization`.
- **Risk**: `mcpRatio` formula edge case when `denom = 0`. **Mitigation**: explicit guard before computing the ratio; default to 0 with `ExposureKnown=false` driving band to `unknown`.
- **Risk**: Bloat findings double-fire when both `high` and `severe` paths are taken. **Mitigation**: `switch` is mutually exclusive — only one case runs per band.

## Reviewer Guidance

When reviewing:
- Verify `Analyze` calls in order: `computeMetrics` → `DetectEcosystem` → `computeToolingUtilization` → `buildFindings`.
- Verify all new slice fields in `ToolingUtilization` are normalized to `[]string{}` not nil.
- Verify ratio computation: when `denom == 0`, no division happens (no panic).
- Verify bloat-finding `Recommendation` strings are drawn ONLY from the FR-006/§D-5 fixed set.
- Run the full package tests and confirm none of the pre-existing privacy tests regress.
- Spot-check: marshal a Report with default Ecosystem, look at the JSON — every required key from `contracts/tooling-utilization.json` should appear with a sensible value (or `unknown`/empty array).

## Implementation Command

```bash
spec-kitty agent action implement WP04 --agent claude
```
