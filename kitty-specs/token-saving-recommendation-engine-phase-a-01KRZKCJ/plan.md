# Implementation Plan: Token-Saving Recommendation Engine (Phase A)

**Branch (planned)**: `codex/token-recommendations-phase-a` (timestamp suffix if collision) | **Date**: 2026-05-19 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/spec.md`

## Summary

Build an additive Go package surface inside `internal/analyzer/` that exposes
(a) a versioned, code-owned **token-saving tool registry**, (b) a small set of
strongly-typed enums for tool state, evidence sources, signals, classes, risk,
and install policy, and (c) a pure deterministic **recommendation engine** that
turns `(signals, ToolStateMap)` into at most one primary and one secondary
`TokenSavingRecommendation`. The engine never re-recommends a tool that is
`active_high`, never stacks untrusted shell/proxy/MCP tools without the
`recommend_with_waiver` install policy, and emits only allowlisted enum
strings and bounded integer counts — verified by a privacy test.

Phase A code lives in new files; existing `internal/analyzer/types.go` and
`internal/analyzer/ecosystem.go` are not rewritten. Phase B (after #38 and
#39 land) will populate `ToolStateMap` from real fingerprint/utilization
inputs without touching the engine.

## Technical Context

**Language/Version**: Go 1.25 (per repo `go.mod`)
**Primary Dependencies**: Standard library only (`encoding/json`, `sort`, `strings`, `testing`). No new third-party imports.
**Storage**: Registry is an in-memory Go literal compiled into the binary (`var registry = []TokenSavingTool{ ... }`). No on-disk state.
**Testing**: `go test ./...` with table-driven tests in `internal/analyzer/token_saving_recommendations_test.go`. Hermetic — no network, no env, no filesystem writes outside the test workdir.
**Target Platform**: Linux/macOS x86_64 and arm64 (matches host repo). Engine is OS-independent.
**Project Type**: Single Go module (existing repo).
**Performance Goals**: < 1 ms per `Recommend(signals, state)` call for ≤ 50 registry entries × ≤ 11 signals (NFR-006).
**Constraints**: Determinism (NFR-001 byte-identical JSON); privacy (NFR-002 only allowlisted enums leak); additivity (NFR-003 no broad rewrites of `types.go` / `ecosystem.go`); hermetic tests (NFR-004).
**Scale/Scope**: ≤ 50 tools in the registry; ≤ 2 recommendations per call (C-006); single-user analyzer pass.
**Recommendation ID scheme**: Composed enum string `rec.<recommendation_class>.<primary_tool_id>.<sorted_signal_ids_joined_with_underscore>`. Stable, human-readable, deterministic by construction.
**Rule precedence**: Fixed 8-step ordering documented in `research.md` (no_usage_visibility → mcp_skill_bloat → mcp_tool_output_bloat → shell_output_bloat → repeated_file_reads/broad_repo_exploration → unchanged_file_rereads → retry_loop/context_growth_spikes → output_verbosity).

No deferred-clarification markers remain.

## Charter Check

*Skipped — `.kittify/charter/charter.md` does not exist for this repo. No charter gates apply.*

## Project Structure

### Documentation (this feature)

```
kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/
├── spec.md                                    # Feature specification (committed)
├── plan.md                                    # This file (committed by setup-plan)
├── research.md                                # Phase 0 output
├── data-model.md                              # Phase 1 output
├── quickstart.md                              # Phase 1 output
├── contracts/
│   └── token_saving_engine_go_api.md          # Phase 1 output — public Go contract
├── checklists/
│   └── requirements.md                        # Spec-phase checklist (committed)
└── tasks/                                     # populated by /spec-kitty.tasks (Phase 2)
```

### Source Code (repository root)

This is a single Go module. Phase A is purely additive under `internal/analyzer/`:

```
internal/analyzer/
├── token_saving_tools.go                      # NEW — registry (Go literal) + lookup API
├── token_saving_tools_test.go                 # NEW — registry invariant tests
├── token_saving_recommendations.go            # NEW — engine + types + enums
├── token_saving_recommendations_test.go       # NEW — table-driven decision tests
├── types.go                                   # UNTOUCHED in Phase A
├── ecosystem.go                               # UNTOUCHED in Phase A
└── ... (existing files unchanged)

docs/remediation/
├── token-saving-tooling-matrix.md             # UPDATED — registry cross-reference
├── plugin-artifacts.md                        # UPDATED — additive recommendation object note
└── token-saving-recommendation-engine.md      # NEW — engine doc (classes, dedupe, privacy, Phase B plan)
```

**Structure Decision**: Single Go module, additive files under
`internal/analyzer/`. No new subdirectories, no new third-party dependencies,
no top-level packages. The registry is a Go literal (not a JSON file under
`signatures/`) because Phase A wants compile-time validation of the enum
fields and zero parser surface; the JSON-signatures pattern is used for
ecosystem regex inputs that are tuned outside of the type system, which is
not the situation here.

## Complexity Tracking

*No Charter Check violations to justify (no charter present).*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None — additive package surface within an existing module, standard library only, no new directories. | — | — |

## Outstanding Risks

| Risk | Mitigation |
| --- | --- |
| Unverifiable tool URLs (some allowlist entries from the brief may not have stable public sources) | Entries with unverified URLs ship with `install_policy = research_only` and an empty `source_url`; `research.md` flags them so Phase B can replace or remove them. |
| Map iteration order in Go (non-deterministic) breaks NFR-001 | All map accesses in engine output paths route through sorted-key helpers (`sortedSignalIDs`, `sortedEvidenceKeys`). A dedicated determinism test marshals twice and `bytes.Equal`s. |
| Future addition of new signals or tool states could silently bypass rule precedence | `RegistryVersion()` is bumped on any registry edit, and the engine has an exhaustiveness test that asserts every `Signal` enum value is named in the precedence list. |
