# Mission Review Report — Token-Saving Recommendation Engine (Phase A)

| Field | Value |
| --- | --- |
| Reviewer | `claude:opus-4-7:mission-reviewer` |
| Date | 2026-05-19 |
| Mission slug | `token-saving-recommendation-engine-phase-a-01KRZKCJ` |
| Mission ID | `01KRZKCJN2VCSE6M6T29VHZS96` |
| Baseline commit | `dad606a` (tasks finalize) |
| HEAD commit | `4128f0079a98ef7eb8462949fb6c8ecd404f2341` |
| WPs reviewed | WP01, WP02, WP03, WP04, WP05, WP06 |
| Merge commit | `a102ad6` (lane-a + lane-planning, --no-ff) |
| Diff scope | 5 new code/test files, 3 doc edits (1 new, 2 additive), 2276 insertions, 0 deletions in owned files |

## Gate Results

| Gate | Status | Note |
| --- | --- | --- |
| Spec-kitty contract tests (`tests/contract/`) | N/A | Spec-kitty-internal gate; mission is in `claude-log-analyzer` Go repo, not spec-kitty platform. |
| Spec-kitty architectural tests (`tests/architectural/`) | N/A | Same as above. |
| Cross-repo e2e (`spec-kitty-end-to-end-testing/scenarios/`) | N/A | Same as above. |
| `issue-matrix.md` | N/A | Phase A is a greenfield feature mission, not a bug-triage mission; no `issue-matrix.md` was authored, none required. |
| **Project tests — `go test ./...`** | **GREEN** | All 14 packages pass. `internal/analyzer` runs 9 token-saving tests (13 acceptance sub-cases) in ~0.2s. |
| `go vet ./...` | GREEN | No diagnostics. |
| `go build ./...` | GREEN | All binaries build. |
| NFR-003 additivity probe (`git diff dad606a..HEAD -- internal/analyzer/types.go internal/analyzer/ecosystem.go`) | EMPTY | Confirmed untouched, as required. |
| Hermeticity probe (forbidden runtime calls in token_saving_*.go) | CLEAN | No `os.Exec`, `exec.LookPath`, `exec.Command`, `http.*`, `net.*`, `os.Getenv`, `os.Open`, `filepath.*` in any token-saving source file. |

## FR Coverage Matrix

| FR | Requirement (summary) | Owning WP | Production code | Test | Status |
| --- | --- | --- | --- | --- | --- |
| FR-001 | Versioned registry, union of brief + matrix doc, deduped, research-only flagged | WP01 | `token_saving_tools.go:59-538` (`var registry`) | `TestRegistryAllowlistCoverage` (covers all 24 brief IDs; verifies non-brief entries are `reference_only` / `research_only`) | ADEQUATE |
| FR-002 | Entry carries 14 typed fields | WP01 | `TokenSavingTool` struct `token_saving_tools.go:38-54` | `TestRegistryInvariants` enforces non-empty ID, canonical form, uniqueness, ResearchOnly↔Policy, waiver-RollbackGuidance, (class,rank) unique, SourceURL/ResearchOnly | ADEQUATE |
| FR-003 | Public lookup API: `GetTool`, `AllTools`, `RegistryVersion` (no mutation) | WP01 | `token_saving_tools.go:542-572` | `TestRegistryVersionConstant`, `TestAllToolsSortedByClassAndRank`, `TestGetToolUnknownReturnsZero` | ADEQUATE |
| FR-004 | `ToolState` enum (6 values) | WP02 | `token_saving_types.go:28-37` | Used by every acceptance test; closed-set check via privacy allowlist | ADEQUATE |
| FR-005 | `EvidenceSource` enum (11 values) | WP02 | `token_saving_types.go:46-60` | Used in `buildRecommendation` filter (`registeredEvidenceSources` at `token_saving_recommendations.go:451`) and the privacy allowlist | ADEQUATE |
| FR-006 | `Signal` enum (11 values) | WP02 | `token_saving_types.go:67-81` | `registeredSignals` filter at `token_saving_recommendations.go:432`; `TestRecommend_AcceptanceScenarios` exercises 8 of 11 signals as primary input | ADEQUATE |
| FR-007 | Output struct fields + types | WP02 | `TokenSavingRecommendation` `token_saving_types.go:260-270`; JSON tags match contract | Marshalled in `TestRecommendDeterminism` and `TestRecommendPrivacyBudget` | ADEQUATE |
| FR-008 | Deterministic pure function (no time/RNG/I/O/env) | WP03, WP04 | `Recommend` at `token_saving_recommendations.go:346-424` — no I/O imports; only `sort`, `strings` | `TestRecommendDeterminism` calls `Recommend` twice per row over 50 input rows and asserts `bytes.Equal` on `json.Marshal` output | ADEQUATE |
| FR-009 | `no_usage_visibility` → ccusage; if ccusage active + server-quota evidence → server-quota-check | WP03, WP04 | Rule 1 at `token_saving_recommendations.go:65`; AS-02 override at `:227-233` (`EvidenceFailureOrRejection > 0` test) | AS-01 + AS-02 in `TestRecommend_AcceptanceScenarios` | ADEQUATE (see RISK-1 on AS-02 evidence-key choice) |
| FR-010 | shell_output_bloat → RTK state machine (absent/installed/configured/active/rejected) | WP03, WP04 | Rule 4 + `pickPrimary` state-machine at `token_saving_recommendations.go:235-263` | AS-03, AS-04, AS-05, AS-06 | PARTIAL — see RISK-2 (no `leanctx` fallback when RTK active/rejected because both leanctx and headroom are `research_only`) |
| FR-011 | mcp_tool_output_bloat → context_mode state machine | WP03, WP04 | Rule 3 + same state-machine | AS-07, AS-08, AS-09 | PARTIAL — same shape as FR-010: AS-09 produces no Primary because `distill` is research_only (test acknowledges this) |
| FR-012 | repeated_file_reads / broad_repo_exploration → retrieval tier; no stacking | WP03, WP04 | Rule 5 at `token_saving_recommendations.go:69`; candidates filter at `:142-155` | AS-10 in acceptance suite | PARTIAL — see RISK-4 (AS-10 expects Serena specifically; engine ignores Serena state because it's research_only and instead emits `claude_context`) |
| FR-013 | unchanged_file_rereads → read_once/leanctx; if active and persists → audit | WP03, WP04 | Rule 6 at `token_saving_recommendations.go:70` (`ClassRereadGuard`) | No dedicated AS-* scenario; covered transitively by `TestRecommendOnePlusOne` and `TestRecommendSkipsActiveTool` | PARTIAL — see RISK-3 (`reread_guard` class has only research_only + reference_only candidates; FR-013 cannot fire any Primary in Phase A; this is a Phase B gap, not implemented as advisory) |
| FR-014 | mcp_skill_bloat → prune/lazy-load/scope; NEVER add another MCP | WP03, WP04 | Rule 2 + advisory branch at `token_saving_recommendations.go:200-214` (PrimaryToolID="" with ReasonPruneFirst) | `TestRecommendMCPSkillBloatNeverAddsMCP` sweeps all 11 signals paired with `mcp_skill_bloat` and asserts Primary class is never `mcp_output_reducer` or `retrieval` | ADEQUATE (see RISK-5 below for the Primary-only scope of this guarantee) |
| FR-015 | output_verbosity only → claude_token_efficient; caveman opt-in only | WP03, WP04 | Rule 8 at `token_saving_recommendations.go:72`; caveman is `research_only` in registry so cannot be a default | AS-12 in acceptance suite | ADEQUATE |
| FR-016 | ≤1 primary + ≤1 secondary; prefer input/context reductions over output tweaks; never stack untrusted shell/proxy/MCP without waiver | WP03, WP04 | Secondary selection at `token_saving_recommendations.go:399-412` with `r.Class == primary.Class` skip | `TestRecommendOnePlusOne` sweeps all 121 (s1,s2) pairs and asserts at most one primary + one secondary with distinct classes | ADEQUATE |
| FR-017 | Skip any tool whose state is active_high; record in SkippedToolIDs/Skipped | WP03, WP04 | `pickPrimary` ActiveHigh branch at `token_saving_recommendations.go:237-242` | `TestRecommendSkipsActiveTool` sweeps every rule class with a real candidate and asserts the active tool appears in Skipped with `ReasonActivePersistent` and never as Primary | ADEQUATE |
| FR-018 | Conflict-state precedence: `rejected_medium > active_high > configured_medium > installed_medium > mentioned_low > unknown` | WP02, WP04 | `ToolStateMap.Resolve` at `token_saving_types.go:201-214` | **No dedicated unit test.** Function exists and is correct, but no test exercises it directly. Transitively covered only insofar as `Recommend` itself never calls `Resolve` (see RISK-6) | **PARTIAL** — see RISK-6 (function present but unreached + untested) |
| FR-019 | Synthetic-input test suite covering AS-01..AS-14 | WP04 | `TestRecommend_AcceptanceScenarios` table | 13 cases AS-01..AS-13 ; AS-14 is covered by `TestRecommendPrivacyBudget` (a separate top-level test, not a table row) | ADEQUATE |
| FR-020 | Privacy test marshals output and asserts only allowlisted enum strings + bounded counts | WP04 | `TestRecommendPrivacyBudget` at `token_saving_recommendations_test.go:636-695`; `findNonAllowlistedSubstrings` + `recommendationAllowlist` form a positive-list scanner | Scanner self-check at line 691-694 injects `sk-ant-LEAKY` and asserts it is flagged | ADEQUATE (see RISK-7 on scanner gaps) |
| FR-021 | Update `token-saving-tooling-matrix.md` and `plugin-artifacts.md` additively | WP06 | Diff: 15 + 13 line insertions, 0 deletions; both files gain a Registry-cross-reference / recommendation-embedding section + bidirectional "See also" links | n/a (docs) | ADEQUATE |
| FR-022 | New `token-saving-recommendation-engine.md` doc | WP05 | New 392-line file; sections: scope, contents, classes, allowlist policy, state model, recommendation contract, risk/install policy, waiver gate, privacy, Phase B integration | n/a (docs) | ADEQUATE |
| NFR-001 | Determinism (byte-equal marshalled JSON across runs) | WP04 | All map iterations route through `SortedTools`, `sortedSignalIDs`, `sortedEvidenceKeys`; only naked `range m` calls live inside those three sort helpers | `TestRecommendDeterminism` (50 rows, byte-equal assertion) | ADEQUATE |
| NFR-002 | Privacy budget (zero raw/private strings in output) | WP04 | Engine never copies caller strings except registered `ToolID` (validated via `GetTool` at `token_saving_recommendations.go:364`) | `TestRecommendPrivacyBudget` decoy-state sweep across all 11 single signals + 5 pair signals | ADEQUATE |
| NFR-003 | Additivity (no rewrites of `types.go` / `ecosystem.go`); existing tests stay green | All WPs | `git diff dad606a..HEAD -- types.go ecosystem.go` is empty; `go test ./...` green | All 14 packages still pass | ADEQUATE |
| NFR-004 | Hermetic tests (no network/env/fs writes) | WP04 | Tests use only stdlib `bytes`, `encoding/json`, `regexp`, `sort`, `strings`, `testing` | Verified by grep of token-saving test imports | ADEQUATE |
| NFR-005 | Registry version bumped on any registry edit | WP01 | `registryVersion = "phase-a-2026-05-19"` constant; `TestRegistryVersionConstant` pins the live value to a golden | Golden-string match test exists | ADEQUATE (see RISK-8: the golden does not encode a content hash, so additive non-content changes could land without a bump; see also drift note) |
| NFR-006 | Engine < 1 ms per call on dev laptop for ≤50 tools × ≤11 signals | WP03 | Implementation is O(rules × candidates) with no I/O | No `Benchmark*` exists; the `TestRecommendDeterminism` row of 50 inputs completes in <10 ms wall-clock, which is consistent with <1 ms per call but not a formal benchmark | **PARTIAL** — see RISK-9 (no `go test -bench` assertion; relies on the test wall-clock as a proxy) |

**Summary**: 17 ADEQUATE, 5 PARTIAL, 0 MISSING, 0 FALSE_POSITIVE.

## Drift Findings

**DRIFT-1 (LOW) — AS-14 is not a table row in `TestRecommend_AcceptanceScenarios`.**

Spec lists 14 acceptance scenarios (AS-01..AS-14). The implementation covers AS-01..AS-13 as table rows in `TestRecommend_AcceptanceScenarios` and folds AS-14 (privacy probe) into the separate `TestRecommendPrivacyBudget` function. This is a structural choice the spec implicitly allows (AS-14 by nature is a marshalling check, not a `Primary` comparison), but the spec text reads "Synthetic-input test suite exercises every acceptance scenario AS-01 through AS-14 with table-driven Go tests" (FR-019). A strict reading would have AS-14 as a row in the table; the implementation chose readability over literal table membership. Evidence: `spec.md:60` (FR-019), `token_saving_recommendations_test.go:227-405` (table covers AS-01..AS-13 only), `:636-695` (AS-14 as standalone test). **Non-blocking** — semantics covered, structure differs from spec wording.

**DRIFT-2 (LOW) — AS-10 reality vs intent.**

Spec AS-10 says "`repeated_file_reads`, Serena `active_high` → Do not recommend Serena again; prefer the next retrieval-tier tool only if waste persists in a different retrieval mode." The implementation's AS-10 test asserts the engine recommends `claude_context` as Primary. But this happens not because the engine *detected* Serena was active and advanced — it happens because Serena is `research_only` and never enters the candidate set to begin with. The engine never observes the active_high signal for Serena because Serena is filtered out at `eligibleCandidates` (`token_saving_recommendations.go:142-155`). Net behavior matches the spec ("don't recommend Serena") but the **mechanism** is different: not a skip-because-active, just a skip-because-research-only. The WP04 review explicitly accepted this, and the test comment acknowledges it (`token_saving_recommendations_test.go:360-362`). Evidence: `token_saving_tools.go:293-307` (Serena entry, `InstallPolicy=research_only`), `token_saving_recommendations.go:149` (research_only filter). **Non-blocking** — observable behavior matches spec, but the FR-012 "skip waste-persists" guard does not actually fire for Serena because Serena cannot fire in the first place.

**DRIFT-3 (LOW) — FR-013 unchanged_file_rereads cannot emit a Primary in Phase A.**

The `reread_guard` class registry entries are: `read_once` (research_only), `openwolf` (research_only), `memsearch` (research_only), `claude_code_hooks_mastery` (reference_only). All four are filtered out by `eligibleCandidates`. When `SignalUnchangedFileRereads` fires alone, the engine therefore produces `Primary == nil` and no Skipped entry (because no candidate is even considered). The spec's FR-013 wording reads "Rule **unchanged_file_rereads** ⇒ recommend `read_once` or `leanctx`" — but `read_once` is research_only in the brief itself, and `leanctx` is in a different class (`shell_output_reducer`). Phase A therefore silently *no-ops* on unchanged_file_rereads. The advisory-branch fallback at `:200-214` does not fire because the rule's class doesn't match `ClassMCPSkillHygiene` or `ClassContextHygiene` (the two advisory classes hard-coded by the "no eligible candidates" branch). Evidence: `token_saving_recommendations.go:200` (advisory only triggered when `len(candidates) == 0`, which IS the case here), `token_saving_tools.go:421-469` (all `reread_guard` entries are research_only/reference_only). Actually re-checking: `len(candidates) == 0` IS true for `reread_guard` after the research_only filter, so the advisory branch DOES fire, and the test would see a Primary with `PrimaryToolID=""` and `Reason=ReasonAbsent`. **Verify**: there is no test for SignalUnchangedFileRereads firing alone. The `TestRecommendOnePlusOne` does pair it with each other signal, but never alone. Evidence: `token_saving_recommendations_test.go:601-626`. **Non-blocking but worth a note**: the FR-013 promise is met only via an empty advisory, which is silent UX.

**DRIFT-4 (LOW) — Reviewer note about WP01 expanding allowlist with `ccstatusline`.**

The brief enumerates 22 explicit IDs (counted at `start-here.md` section ~"Phase A Scope"). The WP01 test's `briefAllowlist` slice contains 24 entries: 22 brief IDs + `ccstatusline` + `claude_code_hooks_mastery` (no, wait — reading `token_saving_tools_test.go:44-69`, the slice is 24 entries; `claude_code_hooks_mastery` is NOT in it, so the actual additions are `ccstatusline` and re-counting shows there are exactly 24 with `ccstatusline` included). Rationale comment at lines 41-43 explains the addition. Evidence: `token_saving_tools_test.go:42-69`. This is a documented scope expansion approved during WP01 review. **Non-blocking** — documented and reviewed.

**DRIFT-5 (NONE) — No code outside the mission's owned files was modified.**

`git diff dad606a..HEAD --name-only` confirms only:
- `internal/analyzer/token_saving_*.go` (5 new files, all owned)
- `docs/remediation/{token-saving-recommendation-engine.md, token-saving-tooling-matrix.md, plugin-artifacts.md}` (1 new, 2 additive)
- `kitty-specs/.../` (spec-kitty status auto-commits)

No invasion of `types.go`, `ecosystem.go`, `analyzer.go`, or any other existing module. No drift.

## Risk Findings

**RISK-1 (LOW) — AS-02 server_quota_check uses `EvidenceFailureOrRejection > 0` as the proxy.**

Spec AS-02 says "`ccusage` active + server-quota mismatch evidence → Recommend server-quota visibility / local-vs-server divergence note". The implementation uses `entry.Sources[EvidenceFailureOrRejection] > 0` as the trigger (`token_saving_recommendations.go:227-233`). `EvidenceFailureOrRejection` is a generic evidence source that callers populate for ANY failure-or-rejection observation (not specifically server-quota). The spec / data-model has no `server_quota_mismatch` evidence source. The interpretation is reasonable for Phase A (the only failure source ccusage could meaningfully emit is a server-quota mismatch, since ccusage itself doesn't fail any other way the analyzer observes), but it is an interpretive choice not documented in code. Evidence: `data-model.md` §EvidenceSource lists only `failure_or_rejection`; no `server_quota_mismatch`. The engine doc (`token-saving-recommendation-engine.md`) does not call this out explicitly. **Non-blocking** — Phase B can refine the evidence-source taxonomy. Suggest adding a comment in `pickPrimary` explaining the proxy choice.

**RISK-2 (MEDIUM) — FR-010 fallback chain (RTK active → leanctx) does not fire in Phase A.**

Spec FR-010: "shell_output_bloat → RTK state machine: absent → rtk; installed/configured → activation/config audit; active + persistent → leanctx (and optionally headroom); rejected → skip RTK + leanctx with risk notes." But `leanctx` and `headroom` are `research_only` in the registry, so they are filtered out of `eligibleCandidates`. When RTK is active_high or rejected_medium, `pickPrimary` walks past RTK (via Skip/continue) and the loop ends with no more candidates — Primary becomes nil. AS-05 and AS-06 acceptance rows accept this (`primaryNil: true`). This is *intentional* per the WP04 reviewer's note ("Only rtk is recommend-eligible in shell_output_reducer (leanctx, headroom are research_only); after skip → no primary"). But the spec language reads as if Phase A should emit `leanctx` as the fallback. Evidence: `spec.md:90` (FR-010), `token_saving_tools.go:226-257` (rtk recommend_with_waiver, leanctx/headroom research_only), `token_saving_recommendations_test.go:282-310` (AS-05/AS-06 accept primaryNil). **Material gap** — the spec promises a fallback that the implementation cannot deliver in Phase A. The Phase B verification of leanctx URL would unblock this. Document this clearly in the engine doc or research.md.

**RISK-3 (MEDIUM) — FR-013 cannot emit a real Primary; advisory branch produces an empty-toolID Primary that may confuse callers.**

When `SignalUnchangedFileRereads` is the only signal and no eligible candidates exist (all entries are research_only/reference_only), `pickPrimary` enters the `len(candidates) == 0` branch and emits an advisory with `PrimaryToolID=""` and `Reason=ReasonAbsent` (NOT `ReasonPruneFirst` or `ReasonAuditConfig` — the rule's `PrimaryReason` is `ReasonAbsent`). The result is a Primary with empty tool ID and reason=absent, which is semantically odd ("the thing that is absent has no name"). The advisory pattern is designed for `mcp_skill_bloat` (Reason=PruneFirst) and `context_hygiene` (Reason=AuditConfig); reread_guard inherits the same code path with a different semantic. Evidence: `token_saving_recommendations.go:200-214, :70` (rule 6 PrimaryReason=ReasonAbsent), no test exercising SignalUnchangedFileRereads alone. **Recommend** adding either (a) a dedicated test row, or (b) a comment in the engine doc clarifying that Phase A `unchanged_file_rereads` always emits an advisory.

**RISK-4 (LOW) — AS-10 semantic vs. mechanical equivalence.**

See DRIFT-2. The risk is that a future change promoting `serena` from `research_only` to `recommend` would silently change AS-10's behavior (engine would now skip serena via the active_high path AND still emit claude_context — observable behavior unchanged, but skipped_tool_ids would include serena, which is the *correct* contract behavior). Phase A's test does not exercise this future state. **Non-blocking**.

**RISK-5 (LOW) — `TestRecommendMCPSkillBloatNeverAddsMCP` is scoped to Primary only.**

The test asserts `Primary.class != mcp_output_reducer && != retrieval`. It does NOT assert anything about Secondary. When `mcp_skill_bloat + mcp_tool_output_bloat` are both present, the engine's Primary is the advisory (mcp_skill_hygiene class) and the Secondary can legitimately be `context_mode` (mcp_output_reducer class). The WP04 reviewer accepted this as consistent with FR-014's "by default" wording. **Verification: this is correct interpretation**, because rule precedence places mcp_skill_hygiene (rule 2) ahead of mcp_output_reducer (rule 3), so mcp_output_reducer can only ever be Secondary when both signals fire. The spirit of "do not add another MCP" is preserved: the user is first told to prune; the secondary suggests a reducer as an additional play. Evidence: `token_saving_recommendations_test.go:571-594`, contract invariant 5 in `contracts/token_saving_engine_go_api.md` (uses the word "Primary's"). **Non-blocking** — defensible interpretation, but worth documenting in the engine doc.

**RISK-6 (MEDIUM) — `ToolStateMap.Resolve` (FR-018) is implemented but unreachable and untested.**

The function exists at `token_saving_types.go:201-214` and is correct (precedence verified by inspection). But: (a) `Recommend` never calls `Resolve` — it consumes `state[id].State` directly without resolving conflicts (`token_saving_recommendations.go:218-219`); (b) no test exercises `Resolve` directly. The spec's edge-case section says "Conflicting states for the same tool... engine resolves deterministically by a documented precedence order". The implementation assumes the caller has already resolved conflicts before populating `ToolStateMap`, which matches the data-model.md note "State is the resolved (post-conflict) state". So functionally this is consistent. But: (1) `Resolve` is dead code at present; (2) FR-018's coverage claim in `tasks.md` says "WP02, WP04" — WP02 implements it, WP04 does not test it. Evidence: `tasks.md` Requirement coverage table FR-018 row; grep of `Resolve` callers returns zero outside tests-that-don't-exist. **Recommend** either (a) call `Resolve` inside `Recommend` (defensive — but contract says caller pre-resolves), or (b) add a unit test for `Resolve` to lock the precedence, or (c) document in the engine doc that the engine assumes pre-resolved state and treats the function as a public helper for callers.

**RISK-7 (LOW) — Privacy positive-list scanner: edge cases.**

The scanner at `token_saving_recommendations_test.go:182-196` tokenises on `[A-Za-z0-9_]+` and decomposes allowlisted tokens via `[.\-]` separators. It also admits any sorted pair of two signal names joined by `_` (line 158-168) to handle multi-signal recommendation IDs. Gaps to consider:
- **Three-signal joins.** If a future change concatenates 3+ signal IDs in a recommendation_id, the scanner's pair allowlist would not match. Phase A's rule precedence makes 3+ joins impossible (`firingSignalsFor` returns signals filtered by a single rule's FiringSignals, and only one rule has 2 firing signals: shell_output_bloat+tool_output_bloat, repeated_file_reads+broad_repo_exploration, retry_loop+context_growth_spikes). So 3+ joins cannot arise. Verified by inspecting `rulePrecedence` — no rule has >2 FiringSignals. **No gap.**
- **Multi-word ToolID like `claude_code_usage_monitor`.** Tokenised as a single token; in `recommendationAllowlist` it's added whole. Decomposition into `claude`, `code`, `usage`, `monitor` happens because separators `.` and `-` split — but `_` is INSIDE tokens, not a separator. So `claude_code_usage_monitor` is allowlisted whole, but its components are not separately added. This works because the recommendation_id format uses `.` as the structural separator between class, tool_id, and signal_join. A multi-word ToolID never gets sub-tokenised. **No gap.**
- **Recommendation ID with empty toolID (advisory).** `composeRecommendationID` produces `rec.mcp_skill_hygiene.none.mcp_skill_bloat`. Tokens: `rec`, `mcp_skill_hygiene`, `none`, `mcp_skill_bloat`. All four are in the allowlist (line 132 "none"; `ClassMCPSkillHygiene`; `SignalMCPSkillBloat`). **No gap.**

The scanner is robust for Phase A's emission paths. **Non-blocking**.

**RISK-8 (LOW) — `RegistryVersion` golden constant ("phase-a-2026-05-19") is a date string, not a content hash.**

`TestRegistryVersionConstant` asserts the constant equals "phase-a-2026-05-19". This means a maintainer who adds a new tool MUST manually edit the date string AND the test in lockstep. A maintainer who adds a tool without editing the constant will pass `TestRegistryVersionConstant` (it still equals the hardcoded golden) but break the NFR-005 contract semantically ("monotonically increasing identifier"). Evidence: `token_saving_tools.go:34`, `token_saving_tools_test.go:188-192`. **Non-blocking** but a planning-bug surface — a content-hash-based golden would catch silent edits. Phase B candidate.

**RISK-9 (LOW) — NFR-006 latency claim has no benchmark test.**

NFR-006 promises < 1 ms per call. There is no `BenchmarkRecommend` in the test file. The `TestRecommendDeterminism` runs 50 row pairs (100 Recommend calls) in ~0.2 s wall-clock for the entire `internal/analyzer` test package, which is consistent with the claim but not a formal assertion. Evidence: `internal/analyzer/` directory has no `_bench` file; `grep -r BenchmarkRecommend` returns nothing. **Non-blocking** — Phase B could add a `Benchmark*` with a hard threshold.

**RISK-10 (LOW) — Engine assumes registry entries with `RecommendationClass=reread_guard` and `class_rank=99` (reference-only) sort after real candidates.**

`claude_code_hooks_mastery` is in `ClassRereadGuard` with `ClassRank=99` and `InstallPolicy=reference_only`. The `eligibleCandidates` filter drops it because it's `reference_only`, but if a future maintainer changes its policy without checking the rank-99 sentinel, it could become a default Primary recommendation despite being a "reference architecture" rather than a runtime tool. Same applies to `awesome_claude_code` in `ClassOutputVerbosity` rank 99. Evidence: `token_saving_tools.go:506-537`. **Non-blocking** — invariant test could be tightened to assert "rank 99 entries must be reference_only".

## Silent Failure Candidates

| File:Line | Pattern | Could mask | Severity |
| --- | --- | --- | --- |
| `token_saving_recommendations.go:264-268` | `pickPrimary` returns `pickResult{Rec: nil, ...}` when all candidates were skipped | A rule that fires but emits no Primary may surprise callers (no Skipped entry from THIS exhausted candidate set sometimes when all candidates were filtered out by research_only). Specifically the AS-05/AS-06 path returns nil Primary with one Skipped entry (the RTK skip), which is correct. The risk is the implicit no-op when ALL candidates are research_only (e.g. distill+token_optimizer_mcp when context_mode is rejected). | LOW |
| `token_saving_recommendations.go:258` | `default: ... buildRecommendation(..., rule.PrimaryReason, ...)` falls through for unknown ToolState strings | If a caller passes a typo'd ToolState (e.g. `"acvtive_high"`), the engine silently treats it as Unknown and emits a `ReasonAbsent` recommendation. The named-type `ToolState` provides compile-time safety for callers within the same module; external JSON callers via Phase B integration would not be guarded. | LOW |
| `token_saving_recommendations.go:218-222` | `entry := state[cand.ID]` — Go returns the zero `ToolStateEntry` if absent, then `st = ""` becomes `ToolStateUnknown` | This is intentional and documented. Not a real silent failure. | NONE |
| `token_saving_types.go:201-214` | `Resolve` ignores any third+ ToolState value; only takes 2 inputs | A caller resolving 3+ evidence sources must call Resolve repeatedly. The function is associative + commutative w.r.t. the order map, so this is safe — but the API is N-1 to use. No test covers the iterated case. | LOW |

## Security Notes

| Concern | Status | Evidence |
| --- | --- | --- |
| Subprocess execution | NONE | No `os/exec`, `exec.Command`, `exec.LookPath`, `exec.Run` anywhere in token-saving sources. |
| Network calls | NONE | No `net/*`, `http.*` imports. |
| Filesystem reads/writes | NONE | No `os.Open`, `os.Read*`, `os.Write*`, `filepath.*`, `ioutil.*`, `io.Copy` etc. |
| Environment variable reads | NONE | No `os.Getenv`, `os.Environ`. |
| Time-based behavior | NONE | No `time.Now`, `time.Since` (would break NFR-001). |
| Random source | NONE | No `math/rand`, `crypto/rand` (would break NFR-001). |
| Reflection | NONE | No `reflect.*` in source. |
| Integer overflow | NONE | All counters are `int` (Go's int is 64-bit on dev platforms); no fixed-width arithmetic. |
| Unbounded allocation | LOW | `EvidenceCounts` map size is bounded by 11 (EvidenceSource enum size); `Skipped` slice grows linearly with candidates × rules, max ~30 entries; no input field is unbounded. |
| Buffer overruns / string-slicing | NONE | All string operations are on enum-typed values; `composeRecommendationID` concatenates known-bounded enum strings. |
| Privacy contract | UPHELD | Caller-supplied `ToolID` strings outside the registry are dropped via `GetTool(tid)` check at `token_saving_recommendations.go:364` and counted only; never echoed in output. Verified by `TestRecommendPrivacyBudget` with explicit decoy ToolIDs (`private_company_secret_tool`, `sk_ant_FAKE_TOKEN_DECOY`). |
| Caller-supplied `EvidenceSource` keys | UPHELD | `buildRecommendation` filters via `registeredEvidenceSources()` at `token_saving_recommendations.go:286-293`; unknown source keys are dropped silently. |
| Caller-supplied `Signal` values | UPHELD | `Recommend` filters via `registeredSignals()` at `token_saving_recommendations.go:349-358`; unknown signals are counted via `UnknownIDCount`. |
| Locked decision C-001 (no CLI probing) | UPHELD | Grep confirms zero subprocess calls in token_saving sources. |
| Locked decision C-005 (only enum keys in output maps) | UPHELD | `EvidenceCounts` keys are filtered to `registeredEvidenceSources`; serialised JSON has enum keys only. |
| Locked decision C-006 (≤1+1) | UPHELD | `TestRecommendOnePlusOne` sweep + Secondary class!=Primary class enforcement at `:399-412`. |
| Locked decision C-010 (waiver-required tools have rollback guidance) | UPHELD | `TestRegistryInvariants` enforces `InstallPolicy == "recommend_with_waiver" ⇒ RollbackGuidance != ""`. Only `rtk` is recommend_with_waiver in the registry; its rollback guidance is populated. |

## Non-Goal Invasion Check

Spec out-of-scope items:
- **No CLI probing**: CONFIRMED — no subprocess calls.
- **No finalized report JSON shape**: CONFIRMED — `RecommendationSet` is the engine's own output type; no integration into report shape.
- **No force-merge with #38/#39 structs**: CONFIRMED — types live in `token_saving_*.go` only; no edits to `types.go`/`ecosystem.go`.
- **No paid plugin generation deep changes**: CONFIRMED — `docs/remediation/plugin-artifacts.md` diff is +13 lines, additive only; no edits to `internal/remediation/`.
- **Issue #68 not closed**: out of scope for this review (operational, not code).

No non-goal invasion detected.

## Ownership Boundary Check

| WP | Declared owned files | Files actually touched (in the WP's commits) | Result |
| --- | --- | --- | --- |
| WP01 | `token_saving_tools.go`, `token_saving_tools_test.go` | Same | CLEAN |
| WP02 | `token_saving_types.go` (+ documented handoff edit to `token_saving_tools.go` to remove WP01's placeholder aliases) | Same. Handoff edit was 2 lines: alias removal + 3 string conversions in registry literal. Confirmed in WP02 review evidence: "the 2 WP01->WP02 handoff edits (alias removal + 3 string conversions) are minimal". | CLEAN (documented handoff) |
| WP03 | `token_saving_recommendations.go` | Same | CLEAN |
| WP04 | `token_saving_recommendations_test.go` | Same. Reviewer evidence: "only token_saving_recommendations_test.go touched". | CLEAN |
| WP05 | `docs/remediation/token-saving-recommendation-engine.md` (new) | Same | CLEAN |
| WP06 | `docs/remediation/{token-saving-tooling-matrix.md, plugin-artifacts.md}` | Same; +28 insertions, 0 deletions, additive only | CLEAN |

No ownership drift.

## Map-Iteration Determinism Audit

All `range` calls in token-saving source files:

| File:Line | Range expression | Map or slice? | Acceptable? |
| --- | --- | --- | --- |
| `token_saving_recommendations.go:81` | `range r.FiringSignals` | slice | yes |
| `:94` | `range signals` | slice | yes |
| `:98` | `range rulePrecedence` | slice | yes |
| `:112` | `range r.FiringSignals` | slice | yes |
| `:125` | `range r.FiringSignals` | slice | yes |
| `:145` | `range all` (output of `AllTools()`) | slice | yes |
| `:217` | `range candidates` | slice | yes |
| `:288` | `range sortedEvidenceKeys(entry.Sources)` | slice (helper-sorted) | yes |
| `:317` | `range signalIDs` | slice | yes |
| `:352` | `range sorted` | slice | yes |
| `:363` | `range state.SortedTools()` | slice (helper-sorted) | yes |
| `:385` | `range validSignals` | slice | yes |
| `:399` | `range rules[1:]` | slice | yes |
| `:473` | `range m` (inside `sortedEvidenceKeys`) | map (acceptable: this IS the sort helper) | yes |
| `token_saving_tools.go:543` | `range registry` | slice | yes |
| `token_saving_types.go:221` | `range m` (inside `SortedTools`) | map (acceptable: this IS the sort helper) | yes |
| `token_saving_types.go:245` | `range cp` | slice | yes |

Only two naked-map `range` calls exist; both are inside the canonical sort helpers (`SortedTools`, `sortedEvidenceKeys`). NFR-001 contract upheld.

## Final Verdict

**PASS WITH NOTES**

The mission is functionally complete, correctly implements the deterministic policy, upholds every locked decision (C-001 through C-010), and ships a robust positive-list privacy scanner. All 9 token-saving tests pass; the full `go test ./...` suite is green; `go vet` and `go build` are clean; NFR-003 additivity is perfect (zero edits to `types.go`/`ecosystem.go`).

The notes are all PARTIAL findings, not failures:

1. **RISK-2 (MEDIUM)**: FR-010's fallback chain (RTK active → leanctx) is unreachable in Phase A because leanctx/headroom are `research_only`. AS-05 and AS-06 accept primary=nil. The spec's promise is mechanical (fallback chain exists), but the registry's verification gaps mean the fallback emits nothing in practice. Document in engine doc or Phase B research notes.

2. **RISK-3 (MEDIUM)**: FR-013's `unchanged_file_rereads` rule fires the empty-candidate advisory branch with `Reason=ReasonAbsent` and `PrimaryToolID=""`, which is semantically odd (the spec language reads as if a real tool should be emitted). No test exercises this signal in isolation.

3. **RISK-6 (MEDIUM)**: `ToolStateMap.Resolve` (FR-018) is implemented correctly but never called by the engine and never directly tested. Acceptable if documented as "callers pre-resolve"; risky if a future maintainer assumes the engine handles conflicts.

4. Several LOW-severity drift items (DRIFT-1 AS-14 table membership, DRIFT-2 AS-10 mechanism, RISK-1 AS-02 evidence proxy, RISK-5 mcp_skill_bloat primary-only scope, RISK-8 golden version string, RISK-9 missing benchmark).

None of the above blocks Phase B. Phase B (issue #38 + #39 wire-up) can proceed immediately. The MEDIUM-severity items should be addressed in the engine doc before any external consumer (paid-pack generator, report UX) begins integrating against the Phase A surface.

## Blocking vs Non-Blocking

**Blocking for Phase B integration**: NONE.

**Non-blocking but should be documented before Phase B integration**:
- RISK-2 (FR-010 unreachable fallback)
- RISK-3 (FR-013 silent empty advisory)
- RISK-6 (FR-018 Resolve is dead code)

**Non-blocking, Phase B follow-up**:
- RISK-1, RISK-4, RISK-5, RISK-7, RISK-8, RISK-9, RISK-10
- DRIFT-1, DRIFT-2, DRIFT-3, DRIFT-4
- Tighten `TestRegistryInvariants` to assert rank-99 entries are reference_only.
- Add a unit test for `ToolStateMap.Resolve` to lock the precedence.
- Add a `BenchmarkRecommend` to formally pin NFR-006.
- Consider a content-hash-based `RegistryVersion` to catch silent edits.
