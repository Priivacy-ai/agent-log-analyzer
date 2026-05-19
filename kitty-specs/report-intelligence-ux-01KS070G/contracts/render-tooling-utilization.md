# Renderer Contract: `renderToolingUtilization(report)`

**Mission**: `report-intelligence-ux-01KS070G`
**Owner**: WP02
**Scope**: client-side rendering — no network, no model invocation, no new persisted state.

## Input

```ts
type WarningBand = "severe" | "high" | "watch" | "normal" | "unknown";

interface MCPUtilization {
  known_server_ids: string[];
  unknown_server_count: number;
  server_count_bucket: string;
  exposed_tool_count_bucket: string;
  context_token_bucket: string;
  exposure_known: boolean;
  inference_source: string;
  call_count: number;
  known_call_count: number;
  unknown_call_count: number;
  unique_known_called_ids: string[];
  unique_unknown_called_count: number;
  utilization_ratio_pct: number;       // 0..100, clamped
  context_efficiency_bucket: string;
  warning_band: WarningBand;
}

interface SkillUtilization { /* analogous skill-scoped fields */ }

interface Finding {
  id: string;
  title: string;
  severity: string;
  cost_impact: string;
  evidence?: object;
  recommendation: string;
  deterministic: boolean;
}

interface ReportLike {
  ecosystem?: {
    tooling_utilization?: {
      mcp?: MCPUtilization;
      skill?: SkillUtilization;
    };
  };
  findings?: Finding[];
}
```

## Behavior

1. **Section visibility**:
   - If `report.ecosystem.tooling_utilization` is missing → section `#tooling-utilization` is hidden, function returns.
   - Otherwise → section is shown with two rows in fixed order: **MCP** first, then **Skill**.

2. **Row content** (applies to both MCP and Skill):
   - Show the bucket labels: `server_count_bucket` / `exposed_count_bucket`, `exposed_tool_count_bucket` (MCP only), `context_token_bucket`, `context_efficiency_bucket`.
   - Show the call/execution counts: `call_count` and split `known_call_count` / `unknown_call_count` (MCP); `executed_count` / `unknown_executed_count` (Skill). Counts are numeric only — never names.
   - Show the unknown surface count (`unknown_server_count` for MCP; `unknown_exposed_count` for Skill).
   - Show the warning-band chip using its enum value (verbatim). Apply a CSS class per band so styling can distinguish `severe`/`high`/`watch`/`normal`/`unknown` (style is implementation detail — band labels are the contract).
   - If `exposure_known === true`: render the utilization ratio as `<utilization_ratio_pct>%`.
   - If `exposure_known === false`: render `inferred from: <inference_source>` and suppress the ratio.

3. **Advice block (FR-005 / FR-006)**:
   - Lookup table (by surface):
     - MCP, `warning_band == "severe"` → search `report.findings[]` for `id == "mcp_bloat_severe"`; render its `recommendation` string in the advice block.
     - MCP, `warning_band == "high"` → search for `id == "mcp_bloat_high"`.
     - Skill, `warning_band == "severe"` → search for `id == "skill_bloat_severe"`.
     - Skill, `warning_band == "high"` → search for `id == "skill_bloat_high"`.
   - If no matching finding is found, the advice block is not rendered.
   - For any band ∈ `{watch, normal, unknown}`, the advice block is not rendered. (Enforced by the lookup table not having a key for those bands.)

4. **Idempotent re-render**: the function must fully replace the prior contents of `#tooling-utilization` on every call.

## Prohibitions

| ID | Prohibition |
|---|---|
| P1 | The function must not render any private MCP/skill name — it never reads `*_ids` arrays as text; it reads their `length` only. |
| P2 | The function must not call `fetch()`, `XMLHttpRequest`, or any network primitive. |
| P3 | The function must not source advice copy from anywhere other than the four allowlisted finding IDs in `report.findings[]`. |
| P4 | The function must not interpolate any field as raw HTML; all field values are rendered via `textContent` or equivalent escape-safe DOM API. |
| P5 | The function must not throw on missing/null/empty inputs; missing data → hidden section or hidden cell. |

## Verification

| Check | How |
|---|---|
| C1: hidden when block missing | assertion against a report with `ecosystem.tooling_utilization === undefined` |
| C2: renders both MCP and Skill rows | assertion against a fixture with both utilization blocks |
| C3: severe band → advice block present | fixture with `warning_band="severe"` AND matching finding present |
| C4: high band → advice block present | fixture with `warning_band="high"` AND matching finding present |
| C5: watch band → no advice block | fixture with `warning_band="watch"` |
| C6: normal band → no advice block | fixture with `warning_band="normal"` |
| C7: unknown band → no advice block | fixture with `warning_band="unknown"` |
| C8: `exposure_known=false` → no ratio displayed, inference label shown | fixture |
| C9: no unknown names in DOM | hostile-fixture leak test asserts no canary in renderer JSON input |
| C10: no XSS path | renderer uses `textContent` only — verified by source inspection |
