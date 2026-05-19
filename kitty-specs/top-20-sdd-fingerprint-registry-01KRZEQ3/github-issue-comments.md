# GitHub Issue Comments — top-20-sdd-fingerprint-registry-01KRZEQ3

Ready-to-paste comment text for each GitHub issue named in the brief.
WP10 (the human or implementer running the issue-hygiene step) fills in the
`<PR-URL>` and `<COMMIT-SHA>` placeholders before posting. **Do not post
these from this WP** — WP09 produces the templates, WP10 posts them.

All references to Spec Kitty, GitHub Spec Kit, and OpenSpec follow the
canonical naming from
[`spec.md` § Domain Language](spec.md#domain-language). Never abbreviate
"Spec Kitty" as "Spec Kit" — that is a different product.

---

## #38 — Epic: starting work

Starting work on the top-20 SDD fingerprint registry mission.

Tracking under Spec Kitty mission slug
`top-20-sdd-fingerprint-registry-01KRZEQ3`. Scope, privacy stance, and
acceptance criteria are pinned in
[`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md`](../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md).

- Mission branch: `kitty/mission-top-20-sdd-fingerprint-registry-01KRZEQ3`
- Implementation lane PRs will land into `main` via the mission merge step.
- Privacy constraints (NFR-001, 16 forbidden raw-string categories) are
  enforced by a build-time serialization-leak test.
- Cross-negative tests (NFR-004) cover Spec Kitty vs GitHub Spec Kit vs
  OpenSpec in both directions.

PR(s): <PR-URL>
Commit: <COMMIT-SHA>

---

## #38 — Epic: implementation complete

Implementation of the top-20 SDD fingerprint registry is complete and merged
into `main`.

Highlights:

- `Ecosystem.WorkflowFingerprints` now carries one `EcosystemFingerprint`
  record per detected SDD tool. Seven bounded fields only; no raw paths, no
  raw version strings, no private names.
- The legacy `WorkflowFrameworks []string` field is preserved unchanged per
  [C-004](../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md#constraints).
- Cross-negative tests cover the Spec Kitty / GitHub Spec Kit / OpenSpec
  matrix (NFR-004).
- Build-time leak test enforces NFR-001 against all 16 forbidden raw-string
  categories.
- Maintainer-facing registry doc lives at
  [`docs/sdd-fingerprint-registry.md`](../../docs/sdd-fingerprint-registry.md).

- PR: <PR-URL>
- Merge commit: <COMMIT-SHA>
- Mission slug: `top-20-sdd-fingerprint-registry-01KRZEQ3`

---

## #42 — Schema implemented

The typed detector schema (FR-004) is implemented.

`SDDDetector` carries `id`, `display_name`, `aliases`, `category`,
`competitor_priority`, `status`, `source_references`, `markers`, and
`confidence_rules`. Markers tag evidence with one of the ten enumerated
source classes from FR-006. Loader validates every enum value at startup;
bad data panics rather than reaching runtime. See
[`data-model.md`](../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md)
and
[`contracts/sdd-detector.schema.json`](../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/sdd-detector.schema.json).

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #43 — Top-20 seeded

The top-20 SDD tool registry is seeded in
`internal/analyzer/signatures/sdd_detectors.json` (FR-005).

- 9 of 20 tools shipped at `verified` (Spec Kitty, GitHub Spec Kit,
  OpenSpec, Kiro, BMAD-METHOD, Spec Workflow MCP, ChatDev,
  Cognition/Devin, Microsoft Agent Framework).
- 11 tools required an A-04 scope conversation — see #66 for the research
  summary and the mission-review scope decision.
- Per-tool research files (one per tool, with public-source citations) live
  in
  [`docs/research/sdd-fingerprints/`](../../docs/research/sdd-fingerprints/README.md).

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #44 — Spec Kitty detector

`spec_kitty` detector implemented (FR-001).

Markers cover the `.kittify/` config dir, `kitty-specs/` mission directory,
and Spec Kitty-specific slash command names. Cross-negative tests (part of
the NFR-004 3×3 matrix) confirm a Spec Kitty fixture does not trigger
`github_spec_kit` or `openspec`.

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #45 — GitHub Spec Kit detector

`github_spec_kit` detector implemented (FR-002).

Markers derive from the upstream `spec-kit` project's public surface
(config layout, command names). Cross-negative tests confirm a GitHub Spec
Kit fixture does not trigger `spec_kitty` or `openspec`. Generic shared
artifacts (`specs/`, `tasks.md`, `design.md`) do not match this detector
alone (FR-012).

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #46 — OpenSpec detector

`openspec` detector implemented (FR-003).

Markers cover the `.openspec/` directory, `openspec.yaml`, and OpenSpec
command names. Cross-negative tests confirm an OpenSpec fixture does not
trigger `spec_kitty` or `github_spec_kit`. As with the other two, generic
shared artifacts do not match alone (FR-012).

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #47 — Kiro/BMAD/GSD detectors

`kiro`, `bmad`, and `gsd` detectors are implemented and `verified`. GSD was
promoted from `research_needed` to `verified` in the post-mission re-research
pass after the canonical upstream was identified at github.com/gsd-build/get-shit-done
(`.planning/` workspace, `get-shit-done-cc` npm package, `/gsd-*` slash command
vocabulary).

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #48 — Long-tail SDD detectors

Long-tail detectors are now all `verified` after the post-mission re-research
pass. The tier ships 9 verified detectors: Spec Workflow MCP, SDD Pilot,
spec2ship, ChatDev, PAUL, fspec, Cognition/Devin, Microsoft Agent Framework,
Tessl. Five tools from the original brief were removed entirely
(Spec-Driven Develop, whenwords, Intent, Agentic Code, CodeSpeak) because
no canonical upstream or public fingerprintable artifact surface could be
identified — what cannot be verified from public sources is scratched from
the list rather than left in a permanent `research_needed` state.

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #49 — Confidence + evidence model

Three-tier discrete confidence model (FR-007) is implemented with
deterministic rules per research §R-05:

- `high` — tool-specific marker + corroborating marker from a different
  source class.
- `medium` — exactly one of `cli_binary` installed, `slash_command`,
  `mcp_server_name`, or `package_manifest`.
- `low` — only `command_name` free-text mention.

`Active: true` requires `Confidence == "high"` AND ≥1 runtime-touch source
(`slash_command`, `mcp_server_name`, or `cli_binary`). Evidence count is
the integer number of marker hits and is always ≥ `len(Sources)`.

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #50 — Privacy leak test

The serialization-leak test asserts NFR-001: a fully populated `Report`
and its `AggregateEvent` payload, serialized via `encoding/json.Marshal`,
contain none of the canary values for the 16 forbidden raw-string
categories. Canary fixtures live in
[`contracts/forbidden-strings.md`](../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/forbidden-strings.md).

The test fails the build the moment any new field is added to
`EcosystemFingerprint` without a corresponding canary in the test — this
is the structural backstop against future schema drift.

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #66 — Research summary (post re-research)

Per-tool research complete for the final 15-tool SDD registry. Per-tool
files with public-source citations live at
[`docs/research/sdd-fingerprints/`](../../docs/research/sdd-fingerprints/README.md).

**Final status: 15 / 15 verified. Zero `research_needed` entries.**

Verified (15): Spec Kitty, GitHub Spec Kit, OpenSpec, Kiro, BMAD-METHOD,
GSD, Spec Workflow MCP, SDD Pilot, spec2ship, ChatDev, PAUL, fspec,
Cognition/Devin, Microsoft Agent Framework, Tessl.

Removed during re-research (no canonical upstream / no public artifact
surface):

- Spec-Driven Develop — the name is the category descriptor itself.
- whenwords — no canonical upstream identified.
- Intent (Augment Code) — hosted product with no documented local
  workspace-local artifact surface; generic name carries extreme false-
  positive risk.
- Agentic Code — category descriptor, no canonical upstream product.
- CodeSpeak — no canonical upstream identified.

Per the project rule, what cannot be verified from public sources is
scratched from the list rather than left in a permanent `research_needed`
state. Per FR-013 the production evaluator only emits fingerprints for
`status: verified` detectors.

- PR: <PR-URL>
- Commit: <COMMIT-SHA>

---

## #67 — CLI presence/version probing

CLI presence and version probing implemented (FR-008, FR-009, FR-016) per
the contract at
[`contracts/cli-probe.md`](../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/cli-probe.md).

- `sdd.CLIProbe` is an injectable interface with a `RealProbe` production
  implementation and a `FakeProbe` test fake. All unit tests use the fake;
  the real probe is exercised only by a dedicated integration test.
- `LookPath` returns only a boolean. The resolved path never leaves the
  function.
- `Version` runs with no shell, no stdin, a sanitized empty `Env`, no
  network, and a 2-second wall-clock deadline (NFR-002). Raw output is
  consumed in-package and normalized to a bounded `version_bucket`; the
  raw string is discarded.
- `version_args` are constrained by a loader-side deny-list (`--config`,
  `--registry`, `--token`, `--server`, `--login`, anything containing
  `/`). Loader rejection panics at startup.
- The evaluator refuses to call `LookPath` or `Version` for any name
  absent from the registry's `cli_binary` allowlist.

- PR: <PR-URL>
- Commit: <COMMIT-SHA>
