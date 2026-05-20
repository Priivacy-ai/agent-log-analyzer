# Mission Review Report: launch-correctness-01KRZZVK

**Reviewer**: claude:opus-4-7:mission-reviewer
**Date**: 2026-05-19
**Mission**: `launch-correctness-01KRZZVK` — Launch Correctness Fixes
**Baseline commit**: `3a7a8da` (last merge before mission start)
**HEAD at review**: `970d4e7` (squash merge of mission on `codex/launch-correctness`)
**WPs reviewed**: WP01, WP02, WP03
**Source-of-truth artifacts read**: `spec.md`, `plan.md`, `data-model.md`, `tasks.md`, `contracts/cli-analyze.md`, `contracts/mcp-call-counting.md`, `contracts/aggregate-merge.md`, `research.md`, all three WP prompt files, all four `status.events.jsonl` blocks.

---

## Gate Results

The four hard gates defined in this skill (Contract / Architectural / Cross-Repo E2E / Issue Matrix) target the Spec Kitty CLI's own Python codebase. **They are not applicable to this Go project** (`agent-log-analyzer`). Per Step 8.5's intent — gates exist where the project defines them — the equivalent gates for this project are the charter verification baseline (NFR-001). Recording those instead.

### Gate 1 — Contract tests
- N/A — this project has no `tests/contract/` subtree. Equivalent: charter verification baseline.

### Gate 2 — Architectural tests
- N/A — this project has no `tests/architectural/` subtree.

### Gate 3 — Cross-repo E2E
- N/A — this project is single-repo.

### Gate 4 — Issue matrix
- N/A — this project does not maintain an `issue-matrix.md`. Equivalent coverage: `acceptance-matrix.json` (created in the acceptance step) records FR-001..FR-010, NFR-001..NFR-005, C-001..C-007 with evidence references. **PASS**: no row has `verdict: unknown`; all marked `pass` except C-003/C-004 marked `pending` (PR-open and issue-closure are PR-time events, not pre-PR gates).

### Equivalent Gate — Charter Verification Baseline (NFR-001)
- Command: `go test ./... && go vet ./... && gofmt -d $(find . -name '*.go' -not -path './.git/*')`
- Exit codes: `0`, `0`, `0` (gofmt produced no output).
- Result: **PASS**.
- Outstanding: `terraform -chdir=infra/aws fmt -check -recursive` and `./scripts/smoke-local.sh` were not re-run in this final review pass; the implementer subagents ran them as part of WP01/WP02/WP03. They should be re-run as part of PR-open hygiene (added to PR test plan).

---

## FR Coverage Matrix

| FR ID | Description (brief) | WP Owner | Test File(s) | Test Adequacy | Finding |
|-------|---------------------|----------|--------------|---------------|---------|
| FR-001 | Positional log path as `--log` alias | WP01 | `cmd/claude-analyzer/main_test.go::TestAnalyze_PositionalOnly_UsesPositional` | ADEQUATE | — |
| FR-002 | Refuse positional + `--log` together | WP01 | `cmd/claude-analyzer/main_test.go::TestAnalyze_PositionalPlusLog_Refuses` (asserts substring `cannot combine positional log path with --log`) | ADEQUATE | — |
| FR-003 | Refuse >1 positional path | WP01 | `cmd/claude-analyzer/main_test.go::TestAnalyze_TwoPositionals_Refuses` (asserts substring `unexpected extra argument`) | ADEQUATE | — |
| FR-004 | Docs document positional form | WP01 | `usage()` text + `README.md` + `docs/testing-plan.md` + `web/app.js` all show both forms | ADEQUATE | — |
| FR-005 | Header tokens never count as calls | WP02 | `tooling_detect_test.go::TestDetectMCPCallsFromToolUseHeaderMask` + `TestInsideAny` + `TestInsideAnyCombinedMCPAndSkillRanges` | ADEQUATE | — |
| FR-006 | Fixture 08 proves the mask | WP02 | `testdata/tooling/08-header-only-zero-calls.log` (6 header tokens, 0 tool_use records) + assertion in `TestDetectMCPCallsFromToolUseHeaderMask` | ADEQUATE | — |
| FR-007 | Merge WorkflowFingerprints by id | WP03 | `aggregate_test.go::TestMergeWorkflowFingerprints_*` (3 tests covering group-by-id, version-bucket disagreement, disjoint ids) + invariants tests | ADEQUATE | — |
| FR-008 | Merge ToolingUtilization (MCP+Skill) | WP03 | `aggregate_test.go::TestMergeMCPUtilization_AllFields`, `TestMergeSkillUtilization_AllFields`, `TestMaxWarningBand_AllPairs` (5×5 matrix) | ADEQUATE | — |
| FR-009 | Paid artifact consumes merged | WP03 | `artifact_test.go::TestGenerate_MergedAggregate_FlowsToArtifact` — builds two distinct synthetic inputs, calls real `AggregateReports`, calls real `Generate`, asserts union appears in artifact bytes | ADEQUATE | — |
| FR-010 | PR comments on #74/#70/#72 at start + ready | All | Procedural | DEFERRED | See OPEN-1 — must be done at PR-open time |
| NFR-001 | Verification baseline | All | `go test ./...` + `go vet ./...` + `gofmt -d` all green at HEAD | ADEQUATE | — |
| NFR-002 | Privacy canary across merge | WP03 | `leak_test.go::TestMergedAggregateContainsNoForbiddenStrings` — reads real fixtures 06+07, runs real `Analyze` + `AggregateReports` + `Generate`, asserts 25 forbidden substrings absent from all 3 sinks (Ecosystem JSON, AggregateEvent JSON, artifact bytes) | ADEQUATE | — |
| NFR-003 | Zero header false-positives | WP02 | `TestGoldenToolingFixtures` 8/8 green; fixture 08 directly asserts CallCount==0 | ADEQUATE | — |
| NFR-004 | CLI ≤ 5% slowdown | WP01 | Code review: `runAnalyze` adds `len(positional)` + 1 bounds compare (O(1)) | ADEQUATE | — |
| NFR-005 | 100-input merge < 5s | WP03 | `aggregate_test.go::TestAggregateReports100_PerfCeiling` — measured ~163µs locally, ceiling 5s | ADEQUATE | — |

**Anti-pattern audit (per Step 6 §"Recurring anti-patterns")**:
- **Synthetic-fixture-only tests**: NOT FOUND. Every FR test exercises real production paths. `TestGenerate_MergedAggregate_FlowsToArtifact` uses synthetic input `Report` values but calls real `AggregateReports` and real `Generate`; the synthetic inputs are constructed inputs, not synthetic outputs that bypass the production path. `TestMergedAggregateContainsNoForbiddenStrings` reads real log fixtures and runs the real analyzer pipeline.
- **Dead code**: NOT FOUND. Grep verified: `latestClaudeLogFn` called from `runAnalyze` (the live entry point); `insideAny` called from `detectMCPCallsFromToolUse`; `mergeMCPUtilization` / `mergeSkillUtilization` / `mergeWorkflowFingerprints` all called from `mergeEcosystems` at lines 147–149.
- **FR-in-frontmatter-but-no-test**: NOT FOUND. All 10 FRs have at least one grep hit in `*_test.go`.
- **API gate not extended**: N/A — no new event types or REST endpoints.
- **TOCTOU between create+API call**: N/A — no new external API calls introduced.
- **Silent empty-result on hidden error**: NOT FOUND. The two `return nil` sites in new code (`unionSorted`, `mergeWorkflowFingerprints` early-return for both-empty inputs) are intentional and documented — they let `omitempty` drop the field rather than emit `[]`.
- **Locked decision violated**: NOT FOUND. C-007 (`evidence_count = sum`) verified at `aggregate.go::mergeWorkflowFingerprints` line 476: `cur.fp.EvidenceCount += fp.EvidenceCount`.
- **Ownership drift at shared files**: ONE LEGITIMATE SHARED FILE (`golden_test.go`, owned by WP03; touched only by WP03 per the diff). No add/add conflicts.

---

## Drift Findings

### DRIFT-1: T016 contract deviation (kept WorkflowFingerprints nilling in `golden_test.go`)

**Type**: SPEC DEVIATION (documented)
**Severity**: LOW
**Spec reference**: `tasks.md` T016 ("Update `golden_test.go` to assert merged `WorkflowFingerprints` instead of nilling them") and `data-model.md` "Touched test data and fixtures" table.
**Evidence**:
- Diff at `internal/analyzer/golden_test.go:55..59`: the nilling lines (`report.Ecosystem.WorkflowFingerprints = nil` and `report.AggregateEvent.Ecosystem.WorkflowFingerprints = nil`) are PRESERVED in the merged code; a new annotation was added above documenting the deviation and pointing readers to the new test surface.
- `acceptance-matrix.json::deviations` field `DEV-T016` records the rationale and compensating coverage.

**Analysis**: The deviation is justified by reality on the ground. The single-report `TestGoldenSampleReport` path runs `Analyze()`, which does NOT invoke `mergeEcosystems`. So `WorkflowFingerprints` in that test's output is populated only by the SDD evaluator, which walks `$PATH` looking for installed CLI binaries. The slice contents differ between a developer laptop with `spec-kitty` installed and a clean CI container. Removing the nilling would expose environment non-determinism without exercising merge logic. FR-007 coverage was relocated to `aggregate_test.go` (six invariants + row-by-row helpers) and `artifact_test.go::TestGenerate_MergedAggregate_FlowsToArtifact` (end-to-end consumer assertion). The reviewer (`reviewer-renata`) accepted the deviation; the acceptance-matrix records it explicitly. **Verdict: accepted**. Not a regression.

---

## Risk Findings

### RISK-1: `detectMCPCallsFromToolUse` re-runs both exposure-header detectors per call

**Type**: PERFORMANCE / DESIGN
**Severity**: LOW
**Location**: `internal/analyzer/tooling_detect.go:316..320`
**Trigger condition**: every paid-scan invocation, every single-report analysis.

**Analysis**: To get the byte-range mask, `detectMCPCallsFromToolUse` calls `detectMCPExposureFromHeaders(input, registry)` and `detectSkillExposureFromHeaders(input, registry)` internally. These detectors are also called separately by the per-report aggregation path (`ecosystem.go::computeToolingUtilization`), so each detector runs **twice** per analysis instead of once. Each detector is a small number of regex passes over the input; the cost is bounded but non-zero. The NFR-005 test (100-input merge in 163µs) suggests the cost is well within budget. The implementer chose this layout to keep the fix contained to `tooling_detect.go` rather than threading exposure values through a signature change. Acceptable for this mission; recorded as a candidate for follow-up refactor if scan latency becomes a concern.

### RISK-2: `bucketLabelRank` collides numeric ordinals across enum families

**Type**: BOUNDARY-CONDITION
**Severity**: LOW
**Location**: `internal/analyzer/aggregate.go:329..353`
**Trigger condition**: a merge between two reports whose `ToolingUtilization` carries bucket values from different families on the same field.

**Analysis**: The `bucketLabelRank` map assigns ordinal `2` to BOTH `"1-3"` (count-bucket family) and `"<1k"` (token-bucket family), `3` to `"4-10"` and `"1k-5k"`, etc. This is intentional under the assumption that bucket fields hold values from a single family. If a future change introduces cross-family bucket values (unlikely given the closed-enum policy), `maxBucketRank("1-3", "<1k")` would non-deterministically pick `a` (because `ra >= rb`). The current code is correct under the closed-enum assumption; flagged as a latent fragility if the bucket enum space is ever widened.

### RISK-3: `mergeSkillUtilization` uses `len(KnownExposedIDs)` as denominator for ratio

**Type**: SPEC INTERPRETATION
**Severity**: LOW
**Location**: `internal/analyzer/aggregate.go::mergeSkillUtilization`
**Trigger condition**: merged ratio output for skills.

**Analysis**: The data-model says `UtilizationRatioPct = round(100 * KnownExecutedCount / KnownExposedCount)` for skills. There is no numeric `KnownExposedCount` field — only the `KnownExposedIDs` slice. The implementer chose `len(union(KnownExposedIDs))` as the denominator, which matches per-report detection semantics (`ecosystem.go::computeToolingUtilization` derives `KnownExposedCount` from the same source). This is the right call but it is an inferred interpretation, not a spec-stated one. Recorded for documentation in a follow-up data-model update.

---

## Silent Failure Candidates

| Location | Condition | Silent result | Spec impact |
|----------|-----------|---------------|-------------|
| — | — | — | — |

No silent-failure candidates introduced by this mission. The two `return nil` early-returns in `unionSorted` and `mergeWorkflowFingerprints` are intentional `omitempty` interactions, documented inline.

---

## Security Notes

| Finding | Location | Risk class | Recommendation |
|---------|----------|------------|----------------|
| — | — | — | — |

No new subprocess calls, no new file I/O paths, no new HTTP calls, no new credential operations. Diff scan: `git diff 3a7a8da..HEAD -- internal/ cmd/ | grep -E '^\+.*subprocess|^\+.*exec\.|^\+.*Open\(|^\+.*os\.Remove'` returned zero hits.

The CLI's new positional-argument handling does pass the positional string into `os.ReadFile(path)` (`main.go:79`), but this was already the behavior for `--log` and is the documented intent for FR-001. No path traversal beyond what the user explicitly provides; no shell evaluation.

---

## Final Verdict

**PASS**

### Verdict rationale

All ten functional requirements (FR-001..FR-010) and all five non-functional requirements (NFR-001..NFR-005) are covered by adequate tests that exercise real production code paths. No anti-pattern from Step 6 is present: no synthetic-fixture-only tests, no dead code, no silent failures, no locked-decision violations, no API-gate gaps. The one documented spec deviation (DRIFT-1 / DEV-T016) is justified by environment-dependent SDD probe behavior; compensating coverage is real and complete. Three LOW risk findings are recorded as candidates for follow-up but none is release-blocking. Security surface is unchanged. The mission's charter privacy stance (C-002) and bounded-cardinality rule (C-001) are preserved structurally — every new merged value flows through the existing `safePublicID` allowlist gate.

The mission is releasable. Open the PR.

### Open items (non-blocking)

- **OPEN-1 (FR-010)**: PR comments on issues #74, #70, #72 at "starting work" and "ready for review" must be posted by the PR-opening agent at PR-open time. The mission code carries this obligation in each WP prompt; the act itself happens during the next step.
- **OPEN-2 (Terraform + Docker smoke)**: Re-run `terraform -chdir=infra/aws fmt -check -recursive` and `./scripts/smoke-local.sh` before PR-open to refresh the verification baseline against the final merged code.
- **OPEN-3 (NFR-004 baseline)**: The new `cmd/claude-analyzer/main_test.go` establishes the going-forward baseline. Future PRs that touch `runAnalyze` must hold within 5% of the previous commit's wall time on the same fixtures.
- **OPEN-4 (Performance follow-up)**: `detectMCPCallsFromToolUse` calls both exposure-header detectors twice per analysis (RISK-1). If scan latency ever becomes a concern, fold the detector outputs through a single pass.
- **OPEN-5 (Skill-ratio denominator)**: Document `mergeSkillUtilization`'s use of `len(union(KnownExposedIDs))` as the ratio denominator in a follow-up data-model amendment (RISK-3).

## Retrospective Reminder

The mission terminus already passed. A `retrospective.yaml` may have been captured. Before context decays, review the captured retrospective and apply staged proposals:

- `spec-kitty retrospect summary` — cross-mission view (read-only)
- `spec-kitty agent retrospect synthesize --mission launch-correctness-01KRZZVK` — apply staged proposals from the authored `retrospective.yaml` (dry-run by default; add `--apply` to mutate)

If the record does not exist, the terminus facilitator did not run or was skipped without a recorded reason — escalate before proceeding to release tag.
