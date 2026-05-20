# Quickstart — Token-Saving Recommendation Engine (Phase A)

This guide shows how to call the Phase A engine from a Go test or a
downstream package (e.g. a paid-pack generator). Phase A is consumed
**through synthetic inputs only**; #38 fingerprint and #39 utilization data
will be wired into the `ToolStateMap` later, in Phase B.

## 1. Import the engine

```go
import (
    "encoding/json"
    "fmt"

    "github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
)
```

## 2. Build a synthetic input

```go
signals := []analyzer.Signal{
    analyzer.SignalShellOutputBloat,
    analyzer.SignalRepeatedFileReads,
}

state := analyzer.ToolStateMap{
    "rtk": {
        Tool:  "rtk",
        State: analyzer.ToolStateConfiguredMedium,
        Sources: map[analyzer.EvidenceSource]int{
            analyzer.EvidenceCLIPresence:     1,
            analyzer.EvidencePluginConfigured: 1,
        },
    },
    "serena": {
        Tool:  "serena",
        State: analyzer.ToolStateActiveHigh,
        Sources: map[analyzer.EvidenceSource]int{
            analyzer.EvidenceLogActiveCommand: 4,
        },
    },
}
```

## 3. Call the engine

```go
set := analyzer.Recommend(signals, state)
out, _ := json.MarshalIndent(set, "", "  ")
fmt.Println(string(out))
```

Expected behaviour for the inputs above:

- `SignalShellOutputBloat` fires rule 4 in the precedence list. The candidate
  is `rtk`. Its state is `configured_medium` → engine emits the
  `audit_config` Primary recommendation pointing at `rtk`.
- `SignalRepeatedFileReads` fires rule 5 (`retrieval`). The candidate is
  `serena`, but `state[serena].State == active_high` → engine appends a
  `SkipNote{tool_id: "serena", reason: "active_persistent", for_signal:
  "repeated_file_reads"}` and advances to the next eligible retrieval tool
  whose state is not `active_high`. If the next eligible tool is
  `research_only`, the engine continues searching; otherwise that tool
  becomes the Secondary recommendation.

## 4. Read the output structure

```jsonc
{
  "primary": {
    "recommendation_id": "rec.shell_output_reducer.rtk.shell_output_bloat",
    "primary_tool_id": "rtk",
    "reason": "audit_config",
    "signal_ids": ["shell_output_bloat"],
    "confidence": "medium",
    "risk_level": "high",
    "install_policy": "recommend_with_waiver",
    "evidence_counts": {
      "cli_presence": 1,
      "plugin_configured": 1
    }
  },
  "secondary": { /* … or omitted if no eligible retrieval tool exists */ },
  "skipped": [
    {
      "tool_id": "serena",
      "reason": "active_persistent",
      "for_signal": "repeated_file_reads"
    }
  ],
  "registry_version": "phase-a-2026-05-19",
  "engine_version": "v0.1-phase-a",
  "signals": ["repeated_file_reads", "shell_output_bloat"],
  "unknown_id_count": 0
}
```

Note that `signal_ids` and `signals` are sorted ascending; the engine's
output JSON is byte-stable for any equivalent input.

## 5. Write a table-driven test

```go
func TestRecommend_ShellBloat_RTKConfigured(t *testing.T) {
    set := analyzer.Recommend(
        []analyzer.Signal{analyzer.SignalShellOutputBloat},
        analyzer.ToolStateMap{
            "rtk": {Tool: "rtk", State: analyzer.ToolStateConfiguredMedium},
        },
    )

    if set.Primary == nil {
        t.Fatal("expected a primary recommendation")
    }
    if set.Primary.PrimaryToolID != "rtk" {
        t.Fatalf("expected rtk primary, got %q", set.Primary.PrimaryToolID)
    }
    if set.Primary.Reason != analyzer.ReasonAuditConfig {
        t.Fatalf("expected audit_config, got %q", set.Primary.Reason)
    }
}
```

## 6. Privacy assertion (mandatory for every new test scenario)

```go
out, err := json.Marshal(set)
if err != nil {
    t.Fatal(err)
}
if leaked := findNonAllowlistedSubstrings(out); len(leaked) > 0 {
    t.Fatalf("recommendation JSON leaked private data: %v", leaked)
}
```

`findNonAllowlistedSubstrings` is implemented once in
`token_saving_recommendations_test.go` and reused by every scenario test.
This is the single point that enforces NFR-002 across the whole suite.

## 7. What you should *not* do

- Do not pass raw user strings (file paths, private tool names, host names,
  branch names) into `ToolStateMap` keys or `Sources` keys. The engine will
  count and discard unknown `ToolID` values via `UnknownIDCount`, but the
  `Sources` map keys are typed as `EvidenceSource` enum and a non-enum value
  would simply fail to compile.
- Do not mutate the slice returned by `AllTools()`. Treat it as immutable;
  it is a defensive copy that may share storage with the registry in future
  refactors.
- Do not call `Recommend` in a loop expecting different results — it is
  deterministic and pure.
