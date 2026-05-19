# Phase 0 Research — Token-Saving Recommendation Engine (Phase A)

This document resolves the open design questions raised during planning. Each
section follows the **Decision / Rationale / Alternatives** form.

## 1. Registry storage format

**Decision.** Encode the registry as a package-level Go literal,
`var registry = []TokenSavingTool{ ... }`, in
`internal/analyzer/token_saving_tools.go`.

**Rationale.**

- Every field is an enum or a small set of strings. The Go compiler validates
  enum-typed fields at compile time — no parser needed, no runtime
  validation overhead.
- Phase A is owned by engineers, not by non-engineer maintainers; the
  ergonomic gain of JSON/YAML edits is not worth the parser surface.
- Existing `internal/analyzer/signatures/*.json` is for regex/ID lists tuned
  outside the type system; the recommendation registry is a typed object
  graph that benefits from `go vet` and IDE jump-to-definition.

**Alternatives considered.**

- **Embedded JSON via `go:embed`.** Matches the codebase's signatures
  convention. Rejected because it introduces a parser, init-time validation,
  and the ability to ship a malformed registry without compile failure.
- **Embedded YAML.** Adds an external dependency, and YAML's whitespace rules
  invite drift. Rejected.

## 2. Recommendation ID scheme

**Decision.** `recommendation_id` is a composed enum string of the form
`rec.<recommendation_class>.<primary_tool_id>.<sorted_signal_ids_joined_with_underscore>`.
For empty-signal cases (no-op recommendation) the trailing segment is the
literal `none`.

**Rationale.**

- Every component is already an allowlisted enum string; the resulting ID is
  itself privacy-safe by construction.
- Human-readable IDs are trivially diff-able when a maintainer reviews
  golden-test fixtures.
- No hashing dependency, no integer-counter drift, no collision risk for the
  allowlisted enum space.

**Alternatives considered.**

- **Truncated SHA-256 hash** of the same canonical tuple. Compact and uniform
  width but opaque; harder to debug failing tests. Rejected.
- **Hand-assigned integer counter per `(class, primary_tool)`.** Smallest IDs
  but creates manual coordination on every registry edit. Rejected.

## 3. Rule firing precedence

**Decision.** When multiple signals fire, the engine evaluates rules in this
fixed order and picks the first that emits a recommendation as `Primary`. It
then continues scanning the same list and picks the next firing rule whose
candidate tool belongs to a different `recommendation_class` as `Secondary`.
At most one of each is emitted (C-006).

Fixed ordering, highest priority first:

1. `no_usage_visibility`            → class `usage_visibility`
2. `mcp_skill_bloat`                → class `mcp_skill_hygiene` (prune-first; never adds an MCP)
3. `mcp_tool_output_bloat`          → class `mcp_output_reducer`
4. `shell_output_bloat`             → class `shell_output_reducer`
5. `repeated_file_reads` / `broad_repo_exploration` → class `retrieval`
6. `unchanged_file_rereads`         → class `reread_guard`
7. `retry_loop` / `context_growth_spikes` → class `context_hygiene`
8. `output_verbosity`               → class `output_verbosity`

**Rationale.**

- Input/context-token reductions sit ahead of output-style tweaks, matching
  the spec's FR-016 directive.
- `mcp_skill_bloat` is placed above other reducers so the engine *removes*
  noise before recommending another tool.
- A fully static list makes determinism trivial to test (the engine emits the
  same primary for any equivalent input regardless of map iteration order).
- The "different-class secondary" rule prevents stacking two tools that solve
  the same problem (e.g. `rtk` + `leanctx` both as primary candidates).

**Alternatives considered.**

- **Severity-weighted scoring.** Each signal carries a numeric weight; engine
  picks the highest score. Rejected — introduces a tunable not specified in
  the brief and makes determinism brittle (weight tweak → mass rewrite of
  expected outputs).
- **Caller-supplied priority list.** Pushes policy out of the engine.
  Rejected — Phase B should rely on the engine being authoritative.

## 4. Tool-state conflict resolution

**Decision.** When the input `ToolStateMap` carries multiple evidence sources
for the same tool that imply different `ToolState` values, the engine resolves
to the highest-trust state using this precedence (highest first):

```
rejected_medium > active_high > configured_medium > installed_medium > mentioned_low > unknown
```

Rationale: an explicit rejection is the strongest signal (a user actively
opted out); an active observation outranks a mere config presence; mere
mentions are the weakest. This ordering is documented in `spec.md`'s **Edge
Cases** section and is enforced by a table-driven test.

**Alternatives considered.**

- **Active beats rejected** (active observation outranks explicit
  rejection). Rejected — re-recommending a tool the user already rejected is
  the exact behavior the engine exists to avoid.
- **Latest-evidence-wins.** Rejected — Phase A has no timestamp model in the
  `ToolStateMap`; adding one would inflate the privacy surface (timestamps
  fingerprint sessions).

## 5. Allowlist verification policy

**Decision.** For every tool ID promoted into Phase A's registry, the
`source_url` field must point at a public repository or canonical project
page that already appears in either (a) `start-here.md`'s allowlist or
(b) the current `docs/remediation/token-saving-tooling-matrix.md`. Any tool
whose public URL cannot be verified during implementation ships with
`install_policy = research_only`, `source_url = ""`, and a `notes` field
explaining the gap. The Phase A test suite asserts that every registry entry
either has a non-empty `source_url` or is `research_only`.

**Rationale.** The brief explicitly forbids inventing source URLs or safety
claims. Marking unverified entries as `research_only` keeps them visible
without giving the engine permission to recommend them by default.

## 6. Determinism implementation pattern

**Decision.** Every place inside the engine that iterates over a map (signals
input, evidence counts output, etc.) routes through a helper that returns a
sorted slice of keys before iteration. `TokenSavingRecommendation` is
serialised with `json.Marshal` and the order of `signal_ids[]` /
`skipped_tool_ids[]` slices is enforced as ascending lexicographic. The
`evidence_counts` field is built into a `map[EvidenceSource]int` whose
iteration order is irrelevant because `encoding/json` already sorts string-
keyed maps for output.

A dedicated test `TestRecommendDeterminism` calls the engine twice with the
same input and asserts `bytes.Equal(jsonA, jsonB)`.

**Rationale.** Go map iteration is intentionally randomised; relying on it
breaks NFR-001 invisibly. Routing through sorted-key helpers is a small,
auditable surface.

## 7. Privacy test design

**Decision.** `TestRecommendPrivacyBudget` builds a representative input set
that includes deliberately "private-looking" decoy strings as values inside
the `ToolStateMap` evidence-count side-channel (e.g. a tool ID variant whose
underlying registry entry is missing). It then marshals the recommendation
output and runs the resulting JSON through a tokenizer that asserts every
substring matches one of:

- a registered enum string (signal, evidence source, class, tool state, risk,
  install policy, ToolID present in the registry),
- a structural JSON character (`{`, `[`, `,`, `:`, `"`, etc.),
- an ASCII digit, period, or underscore (for IDs and counts).

Any byte that does not match the allowlist is a test failure.

**Rationale.** A positive-list scan is the only reliable way to prove that no
private data leaks — denylists rot. The test pins the privacy contract to a
single function so future regressions are easy to spot.

## 8. Phase B handoff surface

**Decision.** Phase A exposes three pure functions and one struct as the only
public surface Phase B will need to consume:

- `Recommend(signals []Signal, state ToolStateMap) RecommendationSet`
- `GetTool(id ToolID) (TokenSavingTool, bool)`
- `AllTools() []TokenSavingTool`
- `RegistryVersion() string`

`RecommendationSet` carries `Primary *TokenSavingRecommendation`,
`Secondary *TokenSavingRecommendation`, and `Skipped []SkipNote`. Phase B's
job is to populate `signals` and `state` from #38 fingerprint + #39
utilization data and to embed the returned recommendations into the report
JSON shape. No Phase A code needs to change at that point.

**Rationale.** Pinning the public surface now gives the parallel epics a
stable target and protects Phase A from being reopened when they land.

## Per-tool research notes

A short note for every allowlist entry slated for the Phase A registry. URLs
are taken verbatim from `start-here.md` or the existing matrix doc; no new
URLs have been invented for Phase A. Entries marked `research_only` ship
without a `source_url` until Phase B verifies them.

### Usage visibility

- **`ccusage`** — Source: github.com/ryoppippi/ccusage. Class
  `usage_visibility`, rank 1. `install_policy = recommend`, install/data risk
  `low`. Always-eligible primary when `no_usage_visibility` fires.
- **`tokenusage`** — Brief allowlist; URL **unverified**. Ships
  `research_only` with empty source URL until verified.
- **`claude_meter`** — Brief allowlist; URL **unverified**. Ships
  `research_only` with empty source URL.
- **`ccstatusline`** — Source: github.com/sirmalloc/ccstatusline. Class
  `usage_visibility`, rank 2. `install_policy = recommend`, low risk.
- **`claude_code_usage_monitor`** — Source: github.com/Maciek-roboblog/Claude-Code-Usage-Monitor.
  Class `usage_visibility`, rank 3. `install_policy = reference_only` (does
  not live inside the plugin runtime).
- **`claude_code_usage_tracker`** — Source: github.com/LyndonWangWork/Claude-Code-Usage-Tracker.
  Class `usage_visibility`, rank 4. `install_policy = reference_only`.

### Shell-output reducers

- **`rtk`** — Source: github.com/rtk-ai/rtk. Class `shell_output_reducer`,
  rank 1. Rewrites shell command execution; `install_policy =
  recommend_with_waiver`, risk `high`. State-machine driven (FR-010).
- **`leanctx`** — Brief allowlist; URL **unverified**. Class
  `shell_output_reducer`, rank 2. Ships `research_only` until verified.
- **`headroom`** — Brief allowlist; URL **unverified**. Class
  `shell_output_reducer`, rank 3. Ships `research_only` until verified.

### MCP/tool-output reducers

- **`context_mode`** — Source: github.com/mksglu/context-mode. Class
  `mcp_output_reducer`, rank 1. `install_policy = recommend`, low risk.
- **`distill`** — Brief allowlist; URL **unverified**. Class
  `mcp_output_reducer`, rank 2. Ships `research_only` until verified.
- **`token_optimizer_mcp`** — Brief flags as research/low confidence.
  Ships `research_only`.

### Retrieval / code-navigation

- **`serena`** — Brief allowlist; URL **unverified** (likely github project).
  Ships `research_only` until verified; Phase B promotion is expected.
- **`claude_context`** — Source: github.com/zilliztech/claude-context. Class
  `retrieval`, rank 2. Note: external vector DB / API key required.
- **`grepai`** — Source: github.com/yoanbernabeu/grepai. Class `retrieval`,
  rank 3. Local-first; requires embedding provider setup.
- **`codegraph`**, **`codebase_memory_mcp`**, **`code_review_graph`**,
  **`semble`**, **`jcodemunch_mcp`**, **`token_savior`**,
  **`cocoindex_code`** — Brief allowlist; URLs **unverified**. All ship
  `research_only` until Phase B verifies. Class `retrieval`.

### Reread guards / session memory

- **`read_once`** — Brief allowlist; URL **unverified**. Class
  `reread_guard`, rank 1. Ships `research_only`.
- **`openwolf`** — Brief allowlist; URL **unverified**. Class `reread_guard`,
  rank 2. Ships `research_only`.
- **`memsearch`** — Source: github.com/zilliztech/memsearch. Class
  `reread_guard`, rank 3. `install_policy = research_only` (matrix doc
  already flags it as too stateful for the initial pack).

### Output verbosity

- **`claude_token_efficient`** — Source: github.com/drona23/claude-token-efficient.
  Class `output_verbosity`, rank 1. `install_policy = recommend`, low risk.
- **`caveman`** — Source: github.com/JuliusBrussee/caveman. Class
  `output_verbosity`, rank 2. `install_policy = research_only` (opt-in only).

### Reference-only

- **`claude_code_hooks_mastery`** — Source: github.com/disler/claude-code-hooks-mastery.
  Reference architecture, not a runtime tool. `install_policy = reference_only`.
- **`awesome_claude_code`** — Source: github.com/hesreallyhim/awesome-claude-code.
  Discovery index. `install_policy = reference_only`.

The exhaustiveness test asserts every brief-listed ID is present in the
registry exactly once and every registry entry either has a verified URL or
ships as `research_only`.
