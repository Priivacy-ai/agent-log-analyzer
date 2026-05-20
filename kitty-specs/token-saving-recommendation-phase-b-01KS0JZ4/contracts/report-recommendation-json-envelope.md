# Contract: `Report.Recommendation` JSON Envelope

The bounded JSON shape rendered into `Report.Recommendation` and embedded
into `PluginArtifact.Recommendation`.

## Shape

```jsonc
{
  "recommendation": {                                 // omitempty
    "primary": {                                      // omitempty
      "recommendation_id": "rtk_active",              // bounded id, registry-defined
      "primary_tool_id": "rtk",                       // bounded enum (registry)
      "skipped_tool_ids": ["leanctx", "headroom"],    // bounded enums, may be []
      "reason": "absent",                             // Reason enum
      "signal_ids": ["shell_output_bloat"],           // []Signal, sorted+deduped
      "confidence": "high",                           // Confidence enum
      "risk_level": "low",                            // RiskLevel enum
      "install_policy": "recommend",                  // InstallPolicy enum
      "evidence_counts": {                            // map[EvidenceSource]int
        "cli_presence": 1,
        "report_mention": 2
      }
    },
    "secondary": { ... same shape ... },              // omitempty
    "skipped": [                                      // omitempty
      { "tool_id": "ccusage", "reason": "active_persistent", "for_signal": "no_usage_visibility" }
    ],
    "registry_version": "v1-phase-a",                 // bounded
    "engine_version": "v0.1-phase-a",                 // bounded
    "signals": ["no_usage_visibility", "tool_output_bloat"], // sorted+deduped
    "unknown_id_count": 3
  }
}
```

## Field-level invariants

| Field | Type | Invariant |
| --- | --- | --- |
| `recommendation_id` | string | Member of the registry's `RecommendationID` enum vocabulary. |
| `primary_tool_id` | string | Member of the registry's `ToolID` allowlist. |
| `skipped_tool_ids` | array | Each element is a member of the allowlist. May be empty (omitempty). |
| `reason` | string | Member of the `Reason` enum. |
| `signal_ids` | array | Each element is a member of the `Signal` enum. Sorted ascending. Deduplicated. |
| `confidence` | string | One of `low` / `medium` / `high`. |
| `risk_level` | string | One of `low` / `medium` / `high`. |
| `install_policy` | string | One of `bundle` / `recommend` / `recommend_with_waiver` / `research_only` / `reference_only`. |
| `evidence_counts` | object | Keys are `EvidenceSource` enum members; values are bounded non-negative integers ≤ 100. Map serialized in sorted key order. |
| `signals` | array | Sorted ascending, deduplicated. |
| `unknown_id_count` | integer | Non-negative; counts the unknown IDs the engine saw via registry lookup. |
| `registry_version` | string | Frozen Phase A value. |
| `engine_version` | string | Frozen Phase A value. |

## What this JSON does NOT contain

- No raw command strings.
- No raw file paths.
- No private/unknown MCP, skill, or plugin names.
- No prompt text.
- No transcript fragments.
- No raw version strings.
- No free-form string fields at all — every string is a bounded enum or
  registry-defined allowlist value.
- No timestamps.
- No user IDs, session IDs, or anything correlatable to a user.

The privacy budget assertion in `internal/analyzer/leak_test.go` checks
for the forbidden patterns above by grepping the marshaled JSON.

## Backwards compatibility

- `omitempty` on `Report.Recommendation` ensures legacy report JSON (no
  field) deserializes cleanly with `Recommendation == nil`.
- A `Report` with `Recommendation == nil` is valid; the UI renders no
  panel and produces no console error (FR-012).
- A `Report` whose `Recommendation` carries `Primary == nil` and
  `Secondary == nil` is also valid; the UI renders the "no action needed"
  note (FR-009).

## Determinism

The same `Report` always produces the same `Recommendation` JSON bytes:

- Field order follows Go field-declaration order in `RecommendationSet`,
  which is fixed.
- All array fields are sorted (`signal_ids`, `signals`).
- `evidence_counts` map iteration goes through sorted keys (Phase A's
  contract).
- The signal slice the engine evaluated is itself the output of
  `sortedSignalIDs`.

Tested by a 100-iteration loop that marshals the resulting
`Recommendation` twice per iteration and asserts byte-identical output.

## Paid artifact passthrough

`PluginArtifact.Recommendation` is the same JSON shape, embedded under
the top-level `recommendation` key of the paid artifact JSON. The
contract is identical because the field is `*analyzer.RecommendationSet`,
carried verbatim from `Report.Recommendation`.
