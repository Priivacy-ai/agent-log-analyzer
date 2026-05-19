# Quickstart: Report Intelligence UX

**Mission**: `report-intelligence-ux-01KS070G`
**Audience**: any agent or developer verifying the mission end-to-end.

## Prerequisites

- Repo checked out at `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer`.
- Go toolchain available (project's existing version).
- Docker available for local smoke + browser QA.
- `main` at or beyond commit `5df679c` (Phase 1 squash).

## Backend / Test Verification

Run the standard verification baseline from the repo root:

```bash
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
```

All five must pass.

## Browser QA (final user-visible gate)

Phase 2 ships UI sections. Type/test checks are necessary but not sufficient — exercise the actual page:

```bash
docker compose up --build
```

Then in a browser at `http://localhost:8080`:

1. Click **Generate Local Commands** and copy the analyze command.
2. Run the analyze command against a known fixture (e.g., `testdata/fixtures/sample-claude.jsonl` or a fixture from the analyzer testdata tree that triggers `mcp_bloat_*` or `skill_bloat_*`).
3. Upload the sanitized report JSON via the printed curl command.
4. Open the resulting report page.

**Verification checklist** (per FR-001…009):

- [ ] **Workflow Fingerprints section** is visible if and only if the report's `ecosystem.workflow_fingerprints[]` is non-empty.
- [ ] For each fingerprint row: ID, confidence label, sources badges, evidence count, active/installed indicators, version_bucket (if present) are visible.
- [ ] **MCP & Skill Utilization section** is visible if and only if `ecosystem.tooling_utilization` is present.
- [ ] MCP row shows the bucket labels, call counts, warning-band chip.
- [ ] Skill row shows the bucket labels, execution counts, warning-band chip.
- [ ] When MCP `warning_band ∈ {high, severe}` and a matching `mcp_bloat_*` finding exists in `report.findings[]`, the advice block renders below the MCP row.
- [ ] When MCP `warning_band ∈ {watch, normal, unknown}`, no advice block appears below the MCP row.
- [ ] Same for Skill.
- [ ] When `exposure_known === false` on either surface, the row shows `inferred from: <enum>` and does NOT show a utilization ratio percentage.

**Privacy verification** (per NFR-001 / C-002):

```bash
# In the browser, View Source on the report page (or use DevTools > Elements > "..." > Copy outer HTML).
# Save the rendered HTML to /tmp/report.html (this exact path is referenced by the WP01 Definition of Done).
grep -Eo 'mcp__[A-Za-z0-9_-]+|skill__[A-Za-z0-9_-]+|plugin__[A-Za-z0-9_-]+' /tmp/report.html | sort -u
```

The expected output is empty OR contains only IDs from the public allowlist (Spec Kitty, OpenSpec, GitHub Spec Kit, etc.). Any other name is a regression.

> **Note on grep coverage**: the regex above only catches `mcp__` / `skill__` / `plugin__` prefixes — these are the highest-risk surfaces for the new sections. The other 13 hostile-canary categories (paths, branches, hostnames, etc.) are caught by the Go-side `TestRenderInputs_NoCanaryInRendererJSON` test in WP01 T006 at the JSON-serialization level. DOM-grep is the narrow visual confirmation; Go test is the structural backstop.

**FR-009 non-regression QA** (per `plan.md` requirement coverage matrix):

After verifying the two new sections, scroll to the existing `Ecosystem` block (the `<pre id="ecosystem">` summary) and confirm all of today's fields still render for the same fixture:

- `Client: …`
- `OS: …`
- `Frameworks: …`
- `MCPs: …`

If any of these regressed, FR-009 fails and the WP cannot be accepted.

**NFR-005 measurement**:

Once during browser QA, wrap the `renderReport(report)` call in `performance.now()` deltas:

```js
// Paste at the top of renderReport in DevTools Console (don't commit):
const _t0 = performance.now();
// ... let the report render normally ...
const _t1 = performance.now();
console.log("renderReport delta:", _t1 - _t0, "ms");
```

Capture two measurements: one against `main` (before this WP) and one against the WP branch (after). Paste both numbers into the PR description. Budget: the **delta of deltas** (after minus before) for the two new renderers combined should stay under 5 ms on a developer laptop.

## Test-Only Local Smoke

If Docker is unavailable, manual JSON-level verification suffices for the privacy invariant:

```bash
go test ./internal/analyzer/ -run TestReportSerializationContainsNoForbiddenStrings -v
go test ./internal/analyzer/ -run TestMergedAggregateContainsNoForbiddenStrings -v
# Plus any new tests added by WP01/WP02
```

## Failure Modes & Triage

| Symptom | Likely cause | Triage |
|---|---|---|
| Advice block renders for `watch` band | WP02 lookup table accepted a band it shouldn't | Re-check the lookup keys in `app.js` against contracts/render-tooling-utilization.md C5–C7. |
| Advice block missing for `severe` band | Matching finding absent from input report | Verify `internal/analyzer/analyzer.go` still emits the `mcp_bloat_*`/`skill_bloat_*` finding for that band; this would be a regression in the analyzer, not the UI. |
| Fingerprint section visible but blank | Empty array hidden-state bug | Check FR-001 / C1 in render-workflow-fingerprints.md. |
| Unknown name appears in DOM | Schema regression introduced upstream | Open as a blocker; this is an NFR-001 violation. Bisect the change that added an unbounded string field. |
| New JS framework / `package.json` | NFR-004 violation | Reject the change. |

## Mission Acceptance Posture

When all WPs are `approved`/`done`:

```bash
spec-kitty next --agent claude --mission report-intelligence-ux-01KS070G --json
# Then per /spec-kitty.accept: write acceptance-matrix.json mapping each FR/NFR/C
# to a verification artifact (test name, manual QA note, or commit reference).
```

## Out-of-Scope Reminders

When verifying, do NOT spot-check Phase 3+ features:

- No "next-best-recommendation" card expected (issue #73 / Phase 3).
- No paid-pack personalization expected (issue #64 / Phase 4).
- No new Stripe / hostname / cloud changes expected.
