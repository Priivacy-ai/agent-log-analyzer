# Contract: MCP / Skill Call Counting Invariant

**Backs:** FR-005, FR-006, NFR-003
**Implementation site:** `internal/analyzer/tooling_detect.go`
**Test sites:** `internal/analyzer/tooling_detect_test.go`, `internal/analyzer/golden_test.go`
**Fixture site (NEW):** `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log`

## Invariant

> A token of the shape `mcp__<server>__<tool>` (or any analogous skill identifier in an enumerated exposure header) appearing inside an MCP exposure-header byte range or a skill exposure-header byte range contributes **zero** to `MCPUtilization.CallCount`, `MCPUtilization.KnownCallCount`, `SkillUtilization.ExecutedCount`, and `SkillUtilization.KnownExecutedIDs`.

Equivalently: header bytes are advertisements; only tokens outside any header byte range are calls.

## Operational definition

Let `H_mcp` and `H_skill` be the sets of byte ranges identified by the exposure-header detectors (one half-open `[Start, End)` interval per recorded header block). For any candidate match offset `o` produced by the raw-byte regex rescan in `detectMCPCallsFromToolUse`:

```
o counts as a call  ⟺  ∀ r ∈ H_mcp ∪ H_skill : o ∉ [r.Start, r.End)
```

Parsed-line tool-use scans (`ToolName` startswith `mcp__`) are already outside header bytes by construction; they are unaffected.

## Implementation contract

1. The exposure header detector populates `mcpExposure.HeaderRanges` and `skillExposure.HeaderRanges` during the same pass that produces existing exposure fields.
2. `detectMCPCallsFromToolUse` accepts (or reads from its enclosing scope) the combined header range set and applies the offset-membership check before incrementing call counters.
3. The header range set is built once per input and reused. No nested loops over `len(rawBytes)`.

### Complexity ceiling

- Header range set is bounded by the number of exposure headers in a log (small constant, typically 0–3).
- Membership check is `O(|H_mcp| + |H_skill|)` per raw match. With a per-log match cap of `M` and a header count of `H`, total mask work is `O(M·H)` — negligible against the existing scan cost.

## C-006 no-op stability

For any fixture where `H_mcp` and `H_skill` are both empty (no exposure headers detected), the masked counter behavior is identical to the existing unmasked counter behavior. The change is a strict no-op on fixtures `00..06` (and any future fixture without header tokens).

Implementation must satisfy:

- If `len(HeaderRanges) == 0`, the mask check short-circuits and the rescan returns the same result as the current code.
- Tests for fixtures `00..06` assert byte-identical `ToolingUtilization` before and after the change (current golden snapshots).

## Fixture (NEW): `08-header-only-zero-calls.log`

Composition:

- One Claude Code system message whose body contains an MCP exposure header advertising at least 5 `mcp__server__tool` identifiers. At least 2 of those identifiers MUST be on the public allowlist (so they would otherwise be counted as known calls). At least 1 SHOULD be unknown (so it would otherwise increment unknown counts).
- Zero `tool_use` records in the rest of the log.
- Privacy hygiene: any unknown server/tool names in the header MUST be obviously synthetic (e.g. `mcp__synthetic-fixture__placeholder`) — never resembling real private names.

Expected derived values after fix:

| Field | Expected |
|-------|----------|
| `MCPUtilization.CallCount` | `0` |
| `MCPUtilization.KnownCallCount` | `0` |
| `MCPUtilization.UniqueKnownCalledIDs` | `[]` |
| `MCPUtilization.UtilizationRatioPct` | `0` |
| `MCPUtilization.WarningBand` | per existing band rules (zero calls + non-zero exposures → likely `watch` or `high` per current thresholds) |
| `MCPUtilization.KnownServerIDs` / `UnknownServerCount` | non-zero (exposures still counted) |

NFR-003 is the load-bearing assertion: across the full fixture set, the masked counter's false-positive rate is **exactly 0**.

## Tests

In `internal/analyzer/tooling_detect_test.go`:

- Extend the existing table-driven test to include fixture `08-header-only-zero-calls.log`.
- Add a unit test for the masking primitive itself: given a list of header ranges and a candidate offset list, assert the in-mask offsets are filtered out.

In `internal/analyzer/golden_test.go`:

- Update golden expectations for fixture `07-mixed-known-unknown.log` if the existing snapshot was inflated by header-token double-counts. The expected counts go **down or stay equal** — never up. A delta in this direction is the fix, not a regression.

In `internal/analyzer/leak_test.go`:

- The privacy canary already iterates the report serialization. Confirm `HeaderRanges` is never present in the serialized JSON (it's unexported and not in any `json:` tag — assert at the canary level by structural checks).

## Out of scope

- Changing exposure header detection itself (it already records header block boundaries locally — the change is to *retain* those boundaries, not to detect them differently).
- Adding a new public `MCPUtilization` field for header counts (still represented by `ExposedToolCount`).
- Skill-specific bug fix (none exists today; defensive structural addition only).
