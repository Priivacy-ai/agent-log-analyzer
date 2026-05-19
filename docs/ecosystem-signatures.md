# Ecosystem Signatures

Ecosystem intelligence is driven by versioned JSON registries embedded into the analyzer:

```text
internal/analyzer/signatures/
  coding_agents.json
  frameworks.json
  mcp_servers.json
  package_managers.json
  plugins.json
  skills.json
```

Rules:

- Known public tools emit stable IDs.
- Unknown MCP/skill/plugin names are counted, not stored.
- Patterns must not capture or emit private raw strings.
- Every new signature needs a fixture or detector test when practical.

The current registry is intentionally public-tool-only. Private company tool names remain outside aggregate analytics unless an explicit opt-in path is added later.

Candidate discovery tooling lives in `cmd/signature-research` and `scripts/research-signatures.sh`. It crawls public registries/search APIs and writes review-only output to `.data/signature-candidates.json`; it does not update the production registry automatically.

## MCP and Skill Utilization (Epic #39)

### What this is

This block answers a single question: *Are the user's MCPs and skills actually being used enough to justify the context they add?* The analyzer measures, per transcript, how many MCP servers/skills appear to be exposed, how much context they likely consume, how often they are actually called or executed, and rolls those signals into a deterministic warning band. The point is not to discourage tooling — it is to surface bloat that taxes a session's context budget without paying for itself.

### The fields

The new block lives at `Ecosystem.tooling_utilization` and has two parallel subtrees: `mcp` (`MCPUtilization`) and `skill` (`SkillUtilization`). The canonical reference is [`kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md`](../kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md); the short tour:

- **Known/unknown ID arrays** (`known_server_ids`, `known_exposed_ids`, `unique_known_called_ids`, `known_executed_ids`) — sorted lists of allowlist hits only. Unknown counterparts (`unknown_server_count`, `unknown_exposed_count`, `unique_unknown_called_count`, `unknown_executed_count`) are counts only; names are never stored.
- **Buckets** (`server_count_bucket`, `exposed_tool_count_bucket`, `exposed_count_bucket`, `context_token_bucket`, `context_efficiency_bucket`) — each is drawn from a closed enumeration. See the buckets table below.
- **Exposure provenance** (`exposure_known`, `inference_source`) — `exposure_known` is true only when a real signal was observed; `inference_source` records how it was obtained (`header`, `calls`, or `none`).
- **Call / execution counts** (`call_count`, `known_call_count`, `unknown_call_count`, `executed_count`) — total observed activity, broken down by allowlist vs. unknown.
- **Ratio and efficiency** (`utilization_ratio_pct`, `context_efficiency_bucket`) — integer percentage of exposed tooling that was actually exercised, plus a coarse efficiency bucket derived from ratio crossed with footprint.
- **Warning band** (`warning_band`) — the rolled-up signal: `normal`, `watch`, `high`, `severe`, or `unknown`.

### Buckets

All string-valued fields above come from these closed sets. The analyzer will never emit a value outside these enumerations.

| Bucket family | Values |
|---------------|--------|
| Count         | `none`, `1-3`, `4-10`, `11-25`, `26-50`, `51-100`, `100+`, `unknown` |
| Token         | `none`, `<1k`, `1k-5k`, `5k-15k`, `15k-50k`, `50k+`, `unknown` |
| Efficiency    | `unused`, `underutilized`, `moderate`, `well-utilized`, `unknown` |
| Warning bands | `normal`, `watch`, `high`, `severe`, `unknown` |

### Footprint estimator

Context-token footprint is estimated with a hybrid:

1. **Schema-text path (preferred)** — when the transcript carries inline tool/skill descriptions (for example, `system-reminder` blocks listing available MCP tools or deferred skills), the analyzer measures the byte length of those local blocks and divides by 4 to approximate tokens. The measured text stays local; only the resulting bucket is emitted.
2. **Constant-per-item fallback** — when no schema text is available, the analyzer falls back to fixed per-item constants: **~150 tokens per exposed MCP tool**, **~250 tokens per MCP server overhead**, **~400 tokens per exposed skill**.
3. **Unknown path** — when neither exposure nor schema text is observable, the token bucket is `unknown` and `exposure_known` is `false`.

### Band thresholds

Warning bands are deterministic; the same inputs always produce the same band. The authoritative thresholds (including the exact bucket and ratio cutoffs for `watch`, `high`, and `severe`) live in [`plan.md` §D-4](../kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md). At a high level: `normal` covers small surfaces or healthy utilization; `watch` flags medium surfaces with low utilization; `high` flags large surfaces with very low utilization and meaningful footprint; `severe` requires `high` conditions *and* at least one observed degradation signal (re-reads, retry depth, or context growth).

**Bands never fire on count alone. A user with many MCPs but high utilization is `normal`.** That product rule is intentional: the goal is to surface unused context, not to punish people for installing tools they exercise.

### Privacy stance

Unknown — that is, non-allowlist — MCP server names, MCP tool names, and skill names are counted only. They are never stored in memory beyond the bucketing step, never logged, and never emitted in any aggregate output. Raw schemas, tool descriptions, tool arguments, skill instruction text, and skill examples are likewise excluded by construction. See [`data-retention-and-analytics.md`](./data-retention-and-analytics.md) for the full upload contract and the enumerated list of categories the analyzer never collects.

Examples in this doc use generic phrasing (for example, "a private MCP server" or "an internal company skill") rather than naming any real product, company, or vendor.

### Differences from Epic #38

Epic #38 introduced the SDD (Spec-Driven Development) tool **fingerprint registry** — the allowlists under `internal/analyzer/signatures/` that identify *which* public tools the user has installed (coding agents, MCP servers, plugins, skills, frameworks, package managers). Epic #39 — this block — is **utilization analytics**: it consumes those allowlists to decide what counts as "known," and then measures whether the installed MCPs and skills are actually being used. The two are complementary and coexist in the same `Ecosystem` block: #38 contributes identity (`MCPServersKnown`, `KnownSkills`, etc.); #39 contributes behavior (`tooling_utilization`).
