# Renderer Contract: `renderWorkflowFingerprints(report)`

**Mission**: `report-intelligence-ux-01KS070G`
**Owner**: WP01
**Scope**: client-side rendering — no network, no model invocation, no new persisted state.

## Input

```ts
type Confidence = "low" | "medium" | "high";

interface EcosystemFingerprint {
  id: string;            // public allowlisted ID
  confidence: Confidence;
  sources: string[];     // closed enum entries
  evidence_count: number;
  active?: boolean;
  installed?: boolean;
  version_bucket?: string;
}

interface ReportLike {
  ecosystem?: {
    workflow_fingerprints?: EcosystemFingerprint[];
  };
}
```

## Behavior

1. **Section visibility**:
   - If `report.ecosystem.workflow_fingerprints` is missing, empty, or non-array → DOM section `#workflow-fingerprints` is set to `hidden` and the function returns.
   - Otherwise → DOM section is shown.

2. **Rows**:
   - For each fingerprint in input order, render one row containing:
     - the `id` as the row title (literal text),
     - the `confidence` enum label,
     - each entry of `sources[]` as a discrete label/badge,
     - `evidence_count` as a numeric count,
     - an indicator for `active` (truthy = visible "active" marker; falsy = no marker),
     - an indicator for `installed` (truthy = visible "installed" marker; falsy = no marker),
     - `version_bucket` as a small label when truthy and non-empty; suppressed otherwise.

3. **Idempotent re-render**: the function must fully replace the prior contents of `#workflow-fingerprints` (no accumulation across calls).

## Prohibitions

| ID | Prohibition |
|---|---|
| P1 | The function must never render any string outside `EcosystemFingerprint`'s bounded fields (no fallback `evidence` text, no derived names). |
| P2 | The function must not call `fetch()`, `XMLHttpRequest`, or any network primitive. |
| P3 | The function must not write to global window state beyond the existing UI conventions in `app.js`. |
| P4 | The function must not throw on any of: `report===null`, `report.ecosystem===undefined`, `workflow_fingerprints===undefined`, empty array, non-array (defensive). |
| P5 | The function must not interpolate any field as raw HTML; all field values are rendered via `textContent` or equivalent escape-safe DOM API. |

## Verification

| Check | How |
|---|---|
| C1: hidden when empty | unit-style assertion against a synthetic report with no fingerprints |
| C2: renders all bounded fields | unit-style assertion against a synthetic report with one fingerprint per confidence band |
| C3: no XSS path | renderer uses `textContent` only — verified by source inspection |
| C4: idempotent re-render | call twice with different inputs; assert DOM matches second input only |
| C5: allowlist invariant on fingerprint IDs | hostile-fixture leak test (WP01 T006 — `TestRenderInputs_NoCanaryInRendererJSON`) asserts no canary string ever reaches the JSON consumed by this renderer **and** that every `EcosystemFingerprint.id` produced for the hostile input is from the public SDD allowlist (i.e. the detector cannot fabricate a private name into the fingerprint surface). |
