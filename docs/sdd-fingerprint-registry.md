# SDD Fingerprint Registry

The SDD fingerprint registry is the analyzer's typed, public, allowlist-driven
catalogue of spec-driven-development (SDD) tools it knows how to recognize in a
Claude Code workspace. It exists so that ecosystem analytics rest on
deterministic, public evidence rather than guessed names — and so that no raw
private content from the workspace can leak into a fingerprint record.

This document is the maintainer-facing entry point. The authoritative scope,
privacy stance, and acceptance criteria live in the mission spec at
[`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md).

## Overview

The registry seeds entries for the top-20 SDD tools (Spec Kitty, GitHub Spec
Kit, OpenSpec, Kiro, BMAD-METHOD, and the long tail). Each entry is a typed
`SDDDetector` record that declares the public markers (config directories,
config files, slash commands, MCP server names, skill names, plugin manifests,
package manifest dependencies, CLI binary names, and CLI version probes) used
to identify that tool, plus deterministic rules that map matched markers to a
discrete confidence level. The analyzer emits at most one
`EcosystemFingerprint` record per detected tool. The record carries seven
bounded fields — `id`, `confidence`, `sources`, `evidence_count`, `active`,
`installed`, `version_bucket` — and nothing else. No raw paths, no raw
`--version` output, no private names, no free-text "evidence" fields. The
structural shape of the record is what makes the privacy guarantees
enforceable; the leak test (NFR-001) is the build-time backstop. See
[mission spec §Purpose](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md#purpose).

## Final top-N table (post re-research)

The per-tool research files in [`docs/research/sdd-fingerprints/`](research/sdd-fingerprints/README.md)
are the authoritative source for every marker and citation. Each detector
entry in `internal/analyzer/signatures/sdd_detectors.json` MUST point back to
one of these files via its `source_references`.

| # | Tool                       | ID                          | Status   | Research file |
| - | -------------------------- | --------------------------- | -------- | ------------- |
| 1 | Spec Kitty                 | `spec_kitty`                | verified | [`spec-kitty.md`](research/sdd-fingerprints/spec-kitty.md) |
| 2 | GitHub Spec Kit            | `github_spec_kit`           | verified | [`github-spec-kit.md`](research/sdd-fingerprints/github-spec-kit.md) |
| 3 | OpenSpec                   | `openspec`                  | verified | [`openspec.md`](research/sdd-fingerprints/openspec.md) |
| 4 | Kiro                       | `kiro`                      | verified | [`kiro.md`](research/sdd-fingerprints/kiro.md) |
| 5 | BMAD-METHOD                | `bmad`                      | verified | [`bmad.md`](research/sdd-fingerprints/bmad.md) |
| 6 | GSD                        | `gsd`                       | verified | [`gsd.md`](research/sdd-fingerprints/gsd.md) |
| 7 | Spec Workflow MCP          | `spec_workflow_mcp`         | verified | [`spec-workflow-mcp.md`](research/sdd-fingerprints/spec-workflow-mcp.md) |
| 8 | SDD Pilot                  | `sdd_pilot`                 | verified | [`sdd-pilot.md`](research/sdd-fingerprints/sdd-pilot.md) |
| 9 | spec2ship                  | `spec2ship`                 | verified | [`spec2ship.md`](research/sdd-fingerprints/spec2ship.md) |
| 10| ChatDev                    | `chatdev`                   | verified | [`chatdev.md`](research/sdd-fingerprints/chatdev.md) |
| 11| PAUL                       | `paul`                      | verified | [`paul.md`](research/sdd-fingerprints/paul.md) |
| 12| fspec                      | `fspec`                     | verified | [`fspec.md`](research/sdd-fingerprints/fspec.md) |
| 13| Cognition / Devin          | `cognition_devin`           | verified | [`cognition-devin.md`](research/sdd-fingerprints/cognition-devin.md) |
| 14| Microsoft Agent Framework  | `microsoft_agent_framework` | verified | [`microsoft-agent-framework.md`](research/sdd-fingerprints/microsoft-agent-framework.md) |
| 15| Tessl                      | `tessl`                     | verified | [`tessl.md`](research/sdd-fingerprints/tessl.md) |

**Current verified count: 15 / 15.**

A post-mission re-research pass investigated each of the 11 tools that had
previously shipped as `research_needed`. Six tools were verified against
public-source citations and promoted to production detectors (GSD, SDD Pilot,
spec2ship, PAUL, fspec, Tessl). The remaining five had no canonical upstream
or no fingerprintable public artifact surface and were removed from the
registry entirely (Spec-Driven Develop, whenwords, Intent, Agentic Code,
CodeSpeak). See
[`docs/research/sdd-fingerprints/README.md`](research/sdd-fingerprints/README.md)
for the per-tool reasoning and the "Removed tools" section.

## Source-class taxonomy

A detector's evidence is expressed as a set of markers, each tagged with one
of ten enumerated source classes (FR-006). One-sentence summaries below; full
type definitions and loader invariants live in
[data-model.md](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md#sddsourceclass).

| Source class       | One-line description |
| ------------------ | -------------------- |
| `config_dir`       | Tool-specific directory name (e.g., `.kittify/`, `.openspec/`). |
| `config_file`      | Tool-specific file name (e.g., `openspec.yaml`). |
| `package_manifest` | Dependency entry in `package.json` / `pyproject.toml` / `go.mod` / similar. |
| `command_name`     | Free-text mention of a tool's CLI name in scrubbed input. Low-confidence on its own. |
| `slash_command`    | Registered slash command identifier (`/<name>`). |
| `mcp_server_name`  | An `mcp__<name>__` server identifier. |
| `skill_name`       | A registered skill identifier. |
| `plugin_manifest`  | A plugin manifest filename or id. |
| `cli_binary`       | An installable CLI binary name probed via `LookPath`. |
| `cli_version_probe`| A safe `--version` / `version` probe of a `cli_binary`. |

## Confidence levels

Confidence is a discrete `high` / `medium` / `low` value. The evaluator
returns the highest tier that the matched markers satisfy. Rules are
deterministic and live in research §R-05 (see
[research.md](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md)).

- **`high`** — ≥1 tool-specific marker (e.g., tool-specific `config_dir`,
  `slash_command`, `mcp_server_name`, `skill_name`, or `plugin_manifest`)
  **plus** ≥1 corroborating marker from a different source class (e.g.,
  `config_file`, `package_manifest`, or `cli_binary`). Special case:
  `cli_binary` installed + tool-specific `config_dir` or `config_file`.
- **`medium`** — exactly one of `cli_binary` installed, `slash_command`,
  `mcp_server_name`, or `package_manifest` matches, with no corroborating
  tool-specific marker.
- **`low`** — only a `command_name` regex hit via free-text mention (e.g.,
  "I tried openspec yesterday"). Treated as evidence of awareness, not
  installation. Per [C-005](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md#constraints),
  low-confidence textual mentions MUST NOT count as installed tools in
  aggregate adoption metrics.

`Active: true` additionally requires `Confidence == "high"` AND at least one
runtime-touchable source (`slash_command`, `mcp_server_name`, or
`cli_binary`).

## Status semantics

The `status` field on each detector record (FR-015) takes one of three values:

- **`verified`** — Every marker has a public citation that a reviewer can
  follow. The detector is safe to emit in production.
- **`research_needed`** — One or more markers lack a citation, or the tool has
  no clear public fingerprintable surface. **Per FR-013, the evaluator MUST
  filter these detectors out before scoring; production fingerprints are
  never emitted for them.** Per
  [C-001](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md#constraints),
  the mission ships zero detectors in this state once Definition of Done is
  satisfied. Any tool stuck here triggers an A-04 scope conversation.
- **`blocked`** — The tool has been investigated and found to have no public
  artifact useful for fingerprinting. Escalate to the user; do not invent
  markers.

## CLI probe privacy rules

The full behavioural contract lives at
[`contracts/cli-probe.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/cli-probe.md).
The four rules a maintainer must internalise:

1. **`LookPath` returns only a boolean.** The resolved executable path is
   never returned to callers, never logged, never stored on the receiver,
   never embedded in an error. The `Installed: true` field on the fingerprint
   carries the entire user-visible answer.
2. **Raw `--version` output never escapes the `sdd` package.** Output is
   consumed in-package by `normalizeVersionBucket`, normalized to a bounded
   `version_bucket`, and the raw string is discarded. The fingerprint carries
   only the bucket.
3. **Allowlisted binary names only.** The evaluator refuses to call
   `LookPath` or `Version` for any name that does not appear as a
   `cli_binary` marker on some loaded `SDDDetector`. Unknown binary names
   discovered in user logs are never probed.
4. **2-second wall-clock timeout per probe ([NFR-002](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md#non-functional-requirements)).**
   Probes run with no shell, no stdin, a sanitized empty `Env`, and no
   network access. Timeout records `installed: true` (if `LookPath` succeeded)
   with no version bucket; the analyzer continues.

`version_args` are constrained by a loader-side deny-list. See the next
section's link for the exact list.

## What must never be uploaded

The full canary list (one representative value per category) lives at
[`contracts/forbidden-strings.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/forbidden-strings.md).
The leak test (NFR-001) builds a fully populated `Report` carrying every
canary and asserts none of them appear in the serialized output or in the
`AggregateEvent` payload. **Do not paste canary fixture values into this
doc, into code comments, or into commit messages — the leak test searches
verbatim.**

The 16 forbidden raw-string categories (names only):

1. user prompts
2. task descriptions
3. raw transcript excerpts
4. raw tool inputs
5. raw tool outputs
6. raw file paths
7. repo URLs
8. branch names
9. usernames
10. hostnames
11. emails
12. session IDs
13. transcript paths
14. private MCP / skill / plugin names
15. raw `LookPath` / `which` paths
16. raw `--version` output / stable hashes of private strings

If a maintainer adds a new field to `EcosystemFingerprint`, they MUST also
extend the leak test with a corresponding canary. PR review enforces this.

## How to add a detector

The end-to-end developer flow — research the tool, add the registry entry,
add fixtures and tests, run the test suite, verify privacy invariants — lives
at
[`quickstart.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/quickstart.md).
That document is the canonical "I have a new tool, what do I do?" reference;
this section deliberately does not duplicate it.

A new detector MUST:

- cite at least one public source reference,
- include at least one tool-specific marker,
- declare confidence rules (no detector evaluates without them),
- be accompanied by a fixture under
  `internal/analyzer/sdd/testdata/fixtures/<tool-id>.txt` and at least one
  positive and one cross-negative test, and
- not introduce any new field to `EcosystemFingerprint` without extending the
  leak test in lockstep.

## Spec Kitty vs GitHub Spec Kit vs OpenSpec

**These three products are distinct and must never be conflated.** They are
separate upstreams with separate maintainers, separate config layouts, and
separate slash-command vocabularies. A workspace can contain artifacts from
all three at once, from one of them, or from none of them, and the analyzer
must report exactly what is present — no inferences across product lines.

| Product | Maintainer surface | This project's relationship |
| ------- | ------------------ | --------------------------- |
| **Spec Kitty** | `.kittify/` config dir, Spec Kitty-specific slash commands, `kitty-specs/` mission directory | This is the SDD tool **this repository's own workflow** uses. It is detected by `spec_kitty` markers. |
| **GitHub Spec Kit** | The upstream GitHub-published `spec-kit` project. Has its own config and command names. | Detected by `github_spec_kit` markers. A Spec Kitty workspace MUST NOT trigger this detector. |
| **OpenSpec** | The upstream OpenSpec product. Has its own config (`openspec.yaml`, `.openspec/`) and command names. | Detected by `openspec` markers. Neither Spec Kitty nor GitHub Spec Kit may trigger this detector. |

Generic artifacts shared across the SDD ecosystem — `specs/`, `tasks.md`,
`design.md`, `STATE.md`, `hooks`, `requirements.md` — **must not** trigger
any of these three detectors on their own (FR-012). Tool-specific markers are
required for a match.

The cross-negative test matrix required by
[NFR-004](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md#non-functional-requirements)
asserts at least 9 dedicated cross-direction cases (3 fixtures × 3
directions): a Spec-Kitty-only fixture triggers exactly `spec_kitty`; a
GitHub-Spec-Kit-only fixture triggers exactly `github_spec_kit`; an
OpenSpec-only fixture triggers exactly `openspec`. Any regression in this
matrix blocks the build.

## Cross-references

- Mission spec — [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md)
- Architecture plan — [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/plan.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/plan.md)
- Data model — [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md)
- CLI probe contract — [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/cli-probe.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/cli-probe.md)
- Forbidden-strings contract — [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/forbidden-strings.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/forbidden-strings.md)
- Per-tool research files — [`docs/research/sdd-fingerprints/`](research/sdd-fingerprints/README.md)
- Adjacent ecosystem docs — [`ecosystem-signatures.md`](ecosystem-signatures.md), [`data-retention-and-analytics.md`](data-retention-and-analytics.md), [`logging-policy.md`](logging-policy.md)
