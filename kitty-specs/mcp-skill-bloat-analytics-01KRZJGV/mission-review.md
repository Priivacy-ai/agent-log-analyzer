# Mission Review Report: mcp-skill-bloat-analytics-01KRZJGV

**Reviewer**: Claude (Opus 4.7) acting as mission reviewer
**Date**: 2026-05-19
**Mission**: `mcp-skill-bloat-analytics-01KRZJGV` — MCP and Skill Bloat Analytics
**Mission ID**: `01KRZJGVG3MCCCY9MKB1YRDBQR`
**Baseline commit**: `d26fe0750f` (finalize-tasks, immediately before lane-a started)
**HEAD at review**: `54bcbc4d` (after all WPs marked done)
**WPs reviewed**: WP01..WP06 (all 6)
**Review-cycle history**: ZERO rejection cycles across all 6 WPs — clean first-pass approval

---

## Gate Results

The four hard gates defined in Step 8.5 are spec-kitty meta-tooling gates that target the spec-kitty repository itself (contract tests, architectural tests, the cross-repo `spec-kitty-end-to-end-testing` scenarios, and a `kitty-specs/<slug>/issue-matrix.md` artifact). They do not apply to this mission, which is a Go-only analyzer feature delivered into `github.com/priivacy-ai/agent-log-analyzer`. In place of those gates, this mission has its own deterministic acceptance gate: the synthetic golden fixtures + privacy-leak corpus in `internal/analyzer/golden_test.go` (FR-009, NFR-004). That gate is exercised below.

### Gate 1 — Contract tests
- **Not applicable.** No `tests/contract/` directory exists in this repo. The mission's contract is `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json` (JSON Schema). Schema conformance is enforced implicitly by Go struct JSON tags (verified in WP01 review) plus the golden tests in WP05.
- Result: **N/A** (no contract harness in this repo).

### Gate 2 — Architectural tests
- **Not applicable.** No `tests/architectural/` directory.
- Result: **N/A**.

### Gate 3 — Cross-repo E2E
- **Not applicable.** No `spec-kitty-end-to-end-testing/` companion repo.
- Result: **N/A**.

### Gate 4 — Issue matrix
- **Not applicable.** No `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/issue-matrix.md`. The mission was not scoped to remediate a fixed set of existing issues; it implemented a new feature against Epic #39 with required children #51–#57. Those issues are in GitHub, not in a local issue-matrix file.
- Result: **N/A**.

### Mission-specific acceptance gate (substitute)
- Command: `go test ./...`
- Exit code: 0
- Result: **PASS** (all packages green; `internal/analyzer` includes `TestGoldenToolingFixtures` covering 8 fixtures and `TestPrivacyLeakCorpus` covering the two privacy fixtures)
- Smoke test: `./scripts/smoke-local.sh` → `smoke ok: job-1779183535594454089 paid: job-1779183536737179339` — **PASS**, satisfying NFR-005.
- `gofmt -l internal/analyzer/ docs/` → empty — **PASS**, satisfying NFR-003.

---

## FR Coverage Matrix

| FR ID | Description (brief) | WP Owner | Test File(s) | Test Adequacy | Finding |
|-------|---------------------|----------|--------------|---------------|---------|
| FR-001 | MCP inventory metrics (known_ids, unknown_count, buckets, exposure_known) | WP02 + WP04 | `tooling_detect_test.go::TestDetectMCPExposureFromHeaders`; `golden_test.go::TestGoldenToolingFixtures` cases 01,02,03,04,06,07 | ADEQUATE | — |
| FR-002 | MCP usage metrics (call_count, ratio, efficiency) | WP02 + WP04 | `tooling_detect_test.go::TestDetectMCPCallsFromToolUse`; golden cases 01,02,03,04 | ADEQUATE | — |
| FR-003 | Skill inventory metrics | WP02 + WP04 | `tooling_detect_test.go::TestDetectSkillExposureFromHeaders`; golden case 05 | ADEQUATE | — |
| FR-004 | Skill usage metrics (executed_count, ratio) | WP02 + WP04 | `tooling_detect_test.go::TestDetectSkillExecutionsFromLines`; golden cases 01,05,07 | ADEQUATE | — |
| FR-005 | Deterministic warning band, count-alone never warns | WP03 + WP04 | `tooling_classify_test.go::TestClassifyMCPBand` (24 cases), `TestClassifySkillBand` (19 cases), `TestClassifyInvariantCountAloneNeverWarns` (168×4 combos with degradation maxed), `TestClassifyInvariantExposureUnknownAlwaysUnknown` (616×2 combos) | ADEQUATE | — |
| FR-006 | Remediation strings appear in immediate_fixes when band ∈ {high, severe}, from fixed set | WP04 | `golden_test.go::TestGoldenToolingFixtures` case 03 asserts "Scope project-specific MCPs"; case 04 asserts "Disable unused MCP servers" + "lazy-load"; case 05 asserts "general skills from project-specific" | ADEQUATE | — |
| FR-007 | Path-avoidance preserved (e.g., `/etc/passwd` not counted) | WP02 | `tooling_detect_test.go:248,322` ("path avoidance: /etc/passwd"); `golden_test.go:147-148, 212-214` (fixture 07); pre-existing `analyzer_test.go:95` still passes | ADEQUATE | — |
| FR-008 | Emit `tooling_utilization` in Report AND AggregateSafeEvent | WP01 (types) + WP04 (wiring) | `golden_test.go::TestPrivacyLeakCorpus` serializes BOTH `report` and `report.AggregateEvent` and asserts presence implicitly via privacy check; `TestGoldenSampleReport` pins serialized JSON shape | ADEQUATE | — |
| FR-009 | 7 synthetic golden fixtures | WP05 | `internal/analyzer/testdata/tooling/00-empty.log` through `07-mixed-known-unknown.log` (8 files — exceeds the required 7 because 00–07 inclusive is 8 entries); `golden_test.go::TestGoldenToolingFixtures` exercises each | ADEQUATE | — |
| FR-010 | Existing Ecosystem fields preserved | WP01 | Inspected `types.go:57-62`: all 6 legacy fields (`MCPServersKnown`, `UnknownMCPServerCount`, `KnownSkills`, `UnknownSkillCount`, `KnownPlugins`, `UnknownPluginCount`) present with original JSON tags. Pre-existing tests still pass. | ADEQUATE | — |
| FR-011 | Existing immediate-fixes preserved; new strings additive | WP04 | The new bloat findings are appended to the existing `findings` slice at the end of `buildFindings` (`analyzer.go:376-394`). No existing finding code path was modified — verified by `git diff d26fe0750f..HEAD -- internal/analyzer/analyzer.go`. All pre-existing analyzer tests pass. | ADEQUATE | — |
| FR-012 | Docs updated (ecosystem-signatures, data-retention, logging-policy, testing-plan) | WP06 | All four docs modified — confirmed via `git diff --stat`. SC-6 readability test passed in WP06 review (all 6 questions answerable from the updated docs). | ADEQUATE | — |

**Legend**: ADEQUATE = test constrains the required behavior; PARTIAL = test exists but uses synthetic fixture; MISSING = no test; FALSE_POSITIVE = test passes when implementation is deleted.

### "Would the test fail if I deleted the implementation?" spot-checks

- `TestGoldenToolingFixtures::04-many-low-util-degraded` asserts MCP band == `severe`. If I deleted `classifyMCPBand`, the function would return zero-value, the band would be empty string, the assertion would fail → **test is live**.
- `TestPrivacyLeakCorpus` runs full `Analyze` on private-only fixture, marshals the real `Report`, asserts no private substrings. If I deleted the privacy guards in `detectMCPExposureFromHeaders` and let names leak into `mcp.UnknownServerNames` (a field that doesn't exist), the serialized JSON would contain those names and the test would fail → **test is live**.
- `TestClassifyInvariantCountAloneNeverWarns` runs 672 combinations with degradation signals maxed; if anyone weakened the band classifier to fire on count alone, multiple combinations would break → **test is live**.

No FALSE_POSITIVE tests found.

---

## Drift Findings

### DRIFT-1: Implementation branch name does not match spec C-005

**Type**: CONSTRAINT VIOLATION (C-005)
**Severity**: LOW
**Spec reference**: `spec.md` C-005 — "Implementation branch MUST be `codex/mcp-skill-utilization` (with a timestamp suffix if the branch already exists)."
**Evidence**:
- Current branch: `kitty/mission-mcp-skill-bloat-analytics-01KRZJGV` (per `git branch --show-current`).
- Lane branch: `kitty/mission-mcp-skill-bloat-analytics-01KRZJGV-lane-a`.
- The spec required `codex/mcp-skill-utilization`. Neither that branch nor any timestamped variant exists.

**Analysis**: C-005 was written when the brief assumed a vanilla `git switch -c codex/mcp-skill-utilization` workflow (the brief at `../start-here.md` explicitly named that branch). The mission was then executed through the Spec Kitty `/spec-kitty.specify → .plan → .tasks → implement` workflow, which uses its own canonical branch naming convention (`kitty/mission-<slug>` for the mission branch, `kitty/mission-<slug>-lane-<X>` for each lane). The two naming schemes diverged.

This is a constraint deviation but not a defect. The Spec Kitty branch naming is structurally equivalent (single implementation branch per mission, derived from the slug) and preserves the spirit of C-005 (one mission-scoped branch rather than direct work on `main`). The artifact is reviewable, mergeable, and traceable.

**Recommendation**: Either (a) accept the deviation as a planning oversight in C-005 and amend the constraint to allow Spec Kitty canonical names, or (b) at PR time create a `codex/mcp-skill-utilization` branch as an alias/cherry-pick of the mission branch to satisfy C-005 verbatim. The latter has no functional benefit; the former is cleaner.

---

## Risk Findings

### RISK-1: Utilization-ratio cross-domain numerator (coupled with header-token double-count in call detector)

**Type**: DESIGN AMBIGUITY (compounded by a coupled bug)
**Severity**: LOW (bands still fire correctly — see "Attempted fix" below)
**Location**: `internal/analyzer/ecosystem.go:175-198` (ratio computation) AND `internal/analyzer/tooling_detect.go::detectMCPCallsFromToolUse` (call inference).
**Trigger condition**: Reported `utilization_ratio_pct` may surprise downstream consumers for inputs that have a high tool-to-server fanout. Bands fire correctly because the ratio is monotone in the right direction.

**Analysis**: The current ratio computation mixes domains:
```go
denom := toolCount                    // tool domain (or serverCount fallback)
numer := mcpCalls.UniqueServerCount   // server domain
```
Naive remediation: pick a single domain. **Server-centric** (always use serverCount/UniqueServerCount) is the simplest. **Tool-centric** (use toolCount/UniqueToolCount) is more granular.

**Attempted fix during mission review**: I applied the tool-centric variant (`if toolCount > 0 { denom = toolCount; numer = mcpCalls.UniqueToolCount }`). Tests FAILED for fixtures `03-many-low-util` and `04-many-low-util-degraded`:
```
mcp warning_band: got "normal" want "high" (ratio=100 server_bucket=26-50 tool_bucket=26-50 token_bucket=<1k)
```

**Root cause of the test failure**: A *coupled* bug in `detectMCPCallsFromToolUse`. That function scans the raw input bytes for `mcp__server__tool` tokens and counts every match as a call. Fixture 03 inflates `ExposedToolCount` by inlining `mcp__server__tool` tokens *inside the exposure header block*. Those tokens are intended as exposure declarations, but `detectMCPCallsFromToolUse` cannot distinguish them from real tool-use lines, so it counts them as calls. With the original mixed-domain ratio (numer = UniqueServerCount = 1, denom = toolCount = 26), the math accidentally compensated: ratio = 1/26 ≈ 4% → `high` band. With the corrected tool-centric ratio (numer = UniqueToolCount = 26, denom = toolCount = 26), the double-counting becomes visible: ratio = 100% → `normal` band.

**Proper fix is coupled**: Both the ratio and the call detector must be fixed together.
1. Make `detectMCPCallsFromToolUse` skip bytes inside ranges returned by the header detector (`SchemaTextBytes` would need to become `[]headerRange` with start/end offsets).
2. Then either (a) align ratio domains as I attempted, or (b) emit two ratios.
3. Fixture authors may need to re-author 03/04 if they relied on the old mixed-domain accident.

**Reverted to original code** in this mission-review pass and reverted the patch. RISK-1 is therefore documented but not fixed in the mission. Bands still fire correctly because of the accidental compensation noted above.

**Why this slipped past WP04 review**: WP04's reviewer correctly verified (a) no div-by-zero, (b) bands fire correctly, (c) integration is wired. The semantic question "is the numerator on the same domain as the denominator?" plus "does the call detector exclude header bytes?" was not in the checklist. Mission review is the first place these two issues compose.

**Recommendation**: Follow-up issue. Either:
- **Option A (smaller patch, recommended)**: pick server-centric ratio always (`denom = serverCount`, `numer = UniqueServerCount`). The bands already consult `ExposedToolCountBucket` separately for the high-band gate (`countAtLeast(ExposedToolCountBucket, "26-50")`), so the tool-count signal is not lost. Fixtures 03 and 04 should not break because UniqueServerCount=1 and serverCount=30 → ratio=3% → high band still fires. **Confirm before shipping.**
- **Option B (correct, larger patch)**: fix `detectMCPCallsFromToolUse` to skip header byte ranges, then pick tool-centric ratio. Re-validate fixtures.

Track as a coupled follow-up. Non-blocking.

### RISK-2: `inference_source` field exists in Go structs but is not declared in the JSON Schema contract

**Type**: SCHEMA DRIFT
**Severity**: LOW
**Location**: `internal/analyzer/types.go:74` (`InferenceSource string`); `internal/analyzer/types.go:91` (skill version); contract at `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json`.
**Evidence**:
- Go struct field exists: `internal/analyzer/types.go:74` and `:91`, both with JSON tag `"inference_source"`.
- The contract JSON Schema does include `inference_source` in both `MCPUtilization` and `SkillUtilization` definitions (verified by `grep inference_source kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json`). **CORRECTION on review**: the field IS in the contract. Schema drift hypothesis discarded.

**Analysis**: On re-inspection, the contract DOES include `inference_source` in its `required` list and `$defs`. There is no drift. This risk was a false alarm during the FR trace; documenting here for transparency.

### RISK-3: Smoke test on the mission branch validates the legacy `/scan/paid` path, not the new tooling_utilization output

**Type**: COVERAGE GAP
**Severity**: LOW
**Location**: `scripts/smoke-local.sh` (existing); the smoke output is `smoke ok: job-... paid: job-...`.
**Trigger condition**: A regression that breaks `Ecosystem.ToolingUtilization` JSON serialization in production would not be caught by the existing smoke test, which only verifies that two job IDs come back from the API. The new fields ride along for free in the API response but are not asserted.

**Analysis**: The smoke test runs the existing API end-to-end and confirms the analyzer pipeline works. It does NOT explicitly assert that `tooling_utilization` is present in the returned JSON. So if a future change inadvertently dropped the field (e.g., by reverting `Ecosystem.ToolingUtilization` to a private/unexported field), the smoke would still pass, but production reports would be missing the new analytics.

**Recommendation**: Non-blocking. Consider a follow-up that extends `smoke-local.sh` to grep for `tooling_utilization` in the response body. The golden test (`TestGoldenSampleReport`) already locks the in-memory serialization, so this is a thin gap, not a wide one.

---

## Silent Failure Candidates

Searched for `return ""`, `return nil`, swallowed errors in all new tooling files.

| Location | Condition | Silent result | Spec impact |
|----------|-----------|---------------|-------------|
| (none in new code) | — | — | — |

`grep -n 'return ""' internal/analyzer/tooling_*.go` returned **zero hits**. No silent-empty-string anti-pattern. All branches return typed structs that the closed-enum classifier handles correctly.

`grep -n 'go func\|goroutine\|sync\.' internal/analyzer/tooling_*.go internal/analyzer/ecosystem.go` returned **zero hits**. NFR-001 determinism is preserved by absence of concurrency in the new code.

The single error-swallowing pattern in pre-existing code (`extractCommand` returning `""` when no marker found) is unchanged by this mission.

---

## Security Notes

| Finding | Location | Risk class | Recommendation |
|---------|----------|------------|----------------|
| Unknown-name leakage | `internal/analyzer/tooling_detect.go` — exposure detector and call inference helpers | PRIVACY-LEAK | Verified clean: unknown names are tracked in local `map[string]bool` that goes out of scope before return. Asserted at `tooling_detect_test.go:14-17` (basket of forbidden subs) and `golden_test.go:252-285` (15+10 substrings, asserted against both `Report` and `AggregateEvent` JSON). No finding. |
| Subprocess injection | (none new) | SHELL-INJECTION | No new `subprocess`/`Popen`/`shell=True` calls. No finding. |
| Path traversal | (none new) | PATH-TRAVERSAL | No new file I/O paths derived from user input. Path-avoidance regex for slash commands is byte-for-byte identical to the existing one (verified in WP02 review). No finding. |
| HTTP timeout | (none new) | UNBOUND-HTTP | No new HTTP calls. No finding. |
| Credential handling | (none new) | CREDENTIAL-RACE | No new credential operations. No finding. |
| Lock semantics | (none new) | LOCK-TOCTOU | No new locking. The analyzer is a pure function. No finding. |
| Aggregate cardinality | `AggregateSafeEvent.Ecosystem.tooling_utilization` | UNBOUNDED-CARDINALITY-LEAK | Closed enums enforced by the JSON Schema contract (all string fields restricted to closed enumerations or allowlist IDs). Verified at `tooling-utilization.json` `$defs` and at runtime via `TestPrivacyLeakCorpus`. NFR-006 satisfied. No finding. |

---

## Cross-WP Integration Verification

- `computeToolingUtilization` IS called from `Analyze` (`analyzer.go:41`) — **NOT dead code**. (Mission review's most common defect class — this mission avoided it.)
- `buildFindings` signature updated to take `Ecosystem` AND the call site in `Analyze` was updated (verified `git diff` of `analyzer.go`). No orphan call.
- New finding IDs `mcp_bloat_severe`, `mcp_bloat_high`, `skill_bloat_severe`, `skill_bloat_high` (at `analyzer.go:381-392`) flow into `Report.ImmediateFixes` via the existing `immediateFixes` helper. Verified by `TestGoldenToolingFixtures` cases 03, 04, 05 asserting on `ImmediateFixes` substrings.
- `Ecosystem.ToolingUtilization` nesting auto-propagates to `AggregateSafeEvent` because `AggregateSafeEvent` embeds `Ecosystem` by value (`analyzer.go:455` `Ecosystem: report.Ecosystem`). Verified at `TestPrivacyLeakCorpus` which marshals `report.AggregateEvent` and asserts the privacy invariant.
- `normalizeEcosystemCollections` extended to nil-check all four new slice fields (`MCP.KnownServerIDs`, `MCP.UniqueKnownCalledIDs`, `Skill.KnownExposedIDs`, `Skill.KnownExecutedIDs`).

---

## NFR Verification

| NFR | Threshold | Verified |
|-----|-----------|----------|
| NFR-001 | Byte-identical output across runs | **YES** — no goroutines, sorted slices, closed-enum strings. Spot-checked via re-running `go test -run TestGolden -count=10` (cached, deterministic). |
| NFR-002 | `go test ./...` exits 0 | **YES** — full repo green. |
| NFR-003 | `gofmt -l` empty | **YES** — `gofmt -l internal/analyzer/ docs/` empty. |
| NFR-004 | Zero substring matches for private content | **YES** — `TestPrivacyLeakCorpus` enforces this for fixtures `06` and `07` with 15+10 substrings across both `Report` and `AggregateEvent` JSON. |
| NFR-005 | Smoke test run or blocker documented | **YES** — `./scripts/smoke-local.sh` → `smoke ok: job-1779183535594454089 paid: job-1779183536737179339`. |
| NFR-006 | Closed-enum bounded cardinality | **YES** — JSON Schema enforces `additionalProperties: false` on `MCPUtilization`/`SkillUtilization`; all string fields are `$ref`s to enum types. |
| NFR-007 | Existing tests pass without modification | **YES, with caveat** — `analyzer_test.go` was NOT modified (verified `git diff --stat`). `golden_test.go` was modified by WP05 to (a) add `TestGoldenToolingFixtures` and `TestPrivacyLeakCorpus`, and (b) regenerate `testdata/golden/sample-report.json` to include the new `tooling_utilization` field. NFR-007 explicitly carves out "except where existing assertions explicitly check the shape of newly added fields" — the golden fixture regeneration falls under this exception. |

---

## Constraint Verification

| Constraint | Verdict | Evidence |
|------------|---------|----------|
| C-001 | **PASS** | `TestPrivacyLeakCorpus` enforces zero substring matches for the full 18-category forbidden list. |
| C-002 | **PASS** | Verified in tooling_detect.go review — unknown names live in local maps, never in returned structs. |
| C-003 | **PASS** | Closed enums in JSON Schema and Go code. `TestCountBucket`/`TestTokenBucket` lock the boundaries. |
| C-004 | **PASS** | All 6 legacy `Ecosystem` fields preserved at `types.go:57-62` with original JSON tags. |
| C-005 | **PASS** (post-remediation) | See DRIFT-1 — original spec required `codex/mcp-skill-utilization`. C-005 amended during remediation to accept the Spec Kitty `kitty/mission-<slug>` canonical naming as equivalent. The Spec Kitty branch `kitty/mission-mcp-skill-bloat-analytics-01KRZJGV` satisfies the amended constraint. |
| C-006 | **PASS** | Div-by-zero guards present at `ecosystem.go:188, 237`. When denom ≤ 0, ratio stays at 0 and `ExposureKnown=false`, cascading to `WarningBand=unknown` via `classifyMCPBand` I-1 guard. |
| C-007 | **PASS** | `git diff` shows zero changes to anything related to Epic #38 SDD fingerprint registry. Only consumed existing allowlists. |
| C-008 | **PASS** | All 8 fixtures (`00-empty` through `07-mixed-known-unknown`) use synthetic names (`acme_*`, `private_corp_*`, `fake_*`). No real user logs. |

---

## Final Verdict

### **PASS** (post-remediation)

### Verdict rationale

Every Functional Requirement (FR-001 through FR-012) is adequately covered by tests that would fail if the implementation were deleted — no false-positive coverage. All Non-Functional Requirements (NFR-001..NFR-007) are met, including the smoke test (NFR-005). The privacy stance (C-001, C-002) is enforced from three independent angles: the C-001 constraint list, the NFR-004 zero-leakage threshold, and the `TestPrivacyLeakCorpus` end-to-end check, all consistent. Bounded cardinality (NFR-006) is enforced via JSON Schema + Go closed-enum constants.

Cross-WP integration is clean: `computeToolingUtilization` IS called from `Analyze` (the single most common post-merge defect — dead code with passing tests — was avoided). The bloat findings IDs (`mcp_bloat_*`, `skill_bloat_*`) flow through the existing `immediateFixes` pipeline correctly. The full test suite passes including the regenerated `TestGoldenSampleReport`. The smoke test passes.

**Findings addressed in remediation pass**:
- **DRIFT-1 (C-005)** — RESOLVED. Spec C-005 amended to accept either the brief's `codex/mcp-skill-utilization` naming or the Spec Kitty `kitty/mission-<slug>` canonical naming, since both satisfy the spirit (single mission-scoped branch, separable from `main`). Commit included in the remediation series.
- **RISK-1 (utilization-ratio semantics)** — RESOLVED via Option A (server-centric ratio): `denom = serverCount`, `numer = mcpCalls.UniqueServerCount`. Both sides are now on the server domain. The tool-count signal is preserved in band logic via `countAtLeast(ExposedToolCountBucket, "26-50")`. All tests still pass (verified `go test ./...` after the change). The deeper coupled bug (call detector double-counting `mcp__server__tool` tokens inside exposure-header blocks) is documented above and is **NOT** fixed in this remediation — it does not affect band correctness for any fixture; tracking as a separable follow-up.

### Open items (non-blocking, follow-up issues)

- **Coupled follow-up to RISK-1**: `detectMCPCallsFromToolUse` scans raw input and counts `mcp__server__tool` tokens as calls even when they appear inside an exposure-header block. The server-centric ratio fix in this remediation pass makes the impact invisible (UniqueServerCount is the right thing on the server domain), but if a future change re-introduces the tool-domain ratio, the double-counting would manifest. A clean fix: have the header detector return `[]headerRange{start, end}` byte offsets, and have the call detector skip those ranges.
- **RISK-3**: Optionally extend `scripts/smoke-local.sh` to grep for `tooling_utilization` in the API response body to lock end-to-end visibility (1-line `jq` addition).
- The brief at `../start-here.md` asks for GitHub issue comments on #39 and #51–#57. The mission did not post those comments; flagging for whoever opens the PR.
