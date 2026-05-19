# Quickstart: Token-Saving Recommendation Phase B Wiring

Engineer-facing validation walkthrough. After all WPs land, follow these
steps to verify the mission against its acceptance criteria.

## 1. Build and run unit tests

```bash
cd /path/to/claude-log-analyzer
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go vet ./...
go test ./...
```

Expected: all tests pass. New tests under `internal/analyzer/` include:

- `recommendation_wiring_test.go` — derivation + AttachRecommendation tests
- `leak_test.go` (extended) — recommendation-JSON privacy budget
- `aggregate_test.go` (extended) — aggregate-recommendation re-run test
- `golden_test.go` (extended) — recommendation JSON golden(s)

Under `internal/remediation/`:

- `artifact_test.go` (extended) — `PluginArtifact.Recommendation` passthrough test

## 2. Smoke the single-report path locally

```bash
./scripts/smoke-local.sh
```

Expected:

- Docker-compose flow comes up cleanly.
- A local log analysis posts a `Report` to the local API.
- The `Report` JSON now carries a `recommendation` object.
- The web UI at the smoke URL shows the new recommendation panel above
  the Workflow Fingerprints section.

## 3. Smoke the paid aggregate path

```bash
go test ./internal/remediation/... -v -run TestPluginArtifact
```

Expected: the generated paid artifact JSON contains a top-level
`recommendation` field whose contents match the merged `Report.Recommendation`.

Optionally, run the paid-local Docker flow if available:

```bash
# (existing paid-local script if applicable)
```

## 4. Render performance check

```bash
./scripts/load-local.sh 25
```

Then open the rendered report page for the severe-MCP fixture in a
browser and use DevTools Performance to confirm:

- p95 render < 500ms (matches the PR #76 baseline).
- No console warnings.
- Recommendation panel renders in the same paint as the other intelligence
  sections.

## 5. DOM privacy probe

In DevTools console on the rendered report page:

```js
const fragments = [...document.querySelectorAll('*')].map(n => n.outerHTML);
const forbidden = /mcp__|skill__|plugin__/;
fragments.filter(f => forbidden.test(f));
```

Expected: empty array (zero forbidden substrings).

## 6. Recommendation JSON privacy probe

```bash
go test ./internal/analyzer/... -v -run TestLeak
```

Expected: leak test asserts zero forbidden patterns over the marshaled
`recommendation` field on a synthetic Report whose `Ecosystem` carries
private MCP/skill/plugin names.

## 7. Determinism probe

```bash
go test ./internal/analyzer/... -v -run TestRecommendationWiringDeterminism
```

Expected: 100 iterations produce byte-identical `recommendation` JSON
output for the canonical fixture.

## 8. Accept-fixture walkthrough

After all WPs are approved:

```bash
spec-kitty accept --mission token-saving-recommendation-phase-b-01KS0JZ4
```

Expected: acceptance reports green. Then:

```bash
spec-kitty merge --mission token-saving-recommendation-phase-b-01KS0JZ4
```

## Acceptance criteria mapping

| Spec AS | Validation |
| --- | --- |
| AS-01 (`tool_output_bloat` → reducer recommendation) | Step 1 unit test + step 2 UI check |
| AS-02 (active Serena skips retrieval recommendation) | Step 1 unit test |
| AS-03 (severe MCP → prune-first) | Step 1 unit test + step 4 UI check |
| AS-04 (no-op renders note) | Step 1 unit test + step 2 UI check |
| AS-05 (active ccusage skips usage-visibility) | Step 1 unit test |
| AS-06 (paid aggregate carries merged set) | Step 3 |
| AS-07 (DOM privacy) | Step 5 |
| AS-08 (single-report determinism) | Step 7 |
| AS-09 (render perf) | Step 4 |
| AS-10 (`configured_medium` → audit-config not install) | Step 1 unit test |

## Troubleshooting

- **`recommendation` missing from `Report` JSON in step 2**: confirm
  `AttachRecommendation` is called inside `Analyze`. Grep:
  `grep -n AttachRecommendation internal/analyzer/analyzer.go`. Exactly
  one call site is expected.
- **DOM privacy probe finds `mcp__*` tokens**: rendered panel is using
  `innerHTML` somewhere; switch to `textContent`-with-element-composition.
- **Determinism test flakes**: somewhere in derivation code an unordered
  `range` over a map is hiding. Grep:
  `grep -n 'range\s\+m\|range\s\+state' internal/analyzer/recommendation_wiring.go`.
