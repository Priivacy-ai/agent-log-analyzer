# SDD Fingerprint Research

This directory holds the per-tool research that feeds the production SDD detector registry (`internal/analyzer/signatures/sdd_detectors.json`). Each tool's file follows the template defined in [`research.md` §R-01](../../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md) and must cite public sources for every marker before it can graduate from `research_needed` to `verified`.

The customer-facing registry overview lives at `docs/sdd-fingerprint-registry.md` (owned by WP09 — not in this directory).

## Methodology

See [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md` §R-01](../../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md) for the canonical per-tool template and §R-02 for the disposable-clone procedure.

Summary:

1. Locate the tool's official repository and docs.
2. (Optional) Clone into `/Users/robert/code-analyzer-dev/claude-code-analyzer-20260519-082245-0QWuF7/research/<tool-id>/` — **never** commit the cloned source.
3. Record markers per source class (`config_dir`, `config_file`, `package_manifest`, `command_name`, `slash_command`, `mcp_server_name`, `skill_name`, `plugin_manifest`, `cli_binary`, `cli_version_probe`).
4. Cite a public URL for every marker.
5. List generic negative-test markers that MUST NOT trigger this detector alone.
6. Set `status: verified` only when all citations resolve and at least one tool-specific marker exists.

## Status semantics

- **`verified`** — Every marker has a public citation that a reviewer can follow to confirm the tool ships that artifact / command / server name. The detector is safe to emit in production.
- **`research_needed`** — One or more markers lack a citation, or the tool has no clear public fingerprintable surface. Per FR-013, production detectors MUST NOT be emitted for tools in this state.
- **`blocked`** — The tool has been investigated and found to have no public artifact useful for fingerprinting. Escalate to the user.

## Citation policy

- Citations MUST resolve to a public URL (GitHub repo, vendor docs, npm/PyPI page, blog post). No private artifacts, no inferred markers.
- Where uncertainty exists about a repo move or rename, pin a commit hash in the citation.
- Disposable install runs in the research-clones directory count as public-source equivalent (we are observing what a published tool generates), but each such observation MUST cite the upstream repo or docs that document the install command.

## Final top-N status index (post re-research)

After the post-mission re-research pass, the registry contains **15 verified SDD tools** in 3 tiers. All previously `research_needed` entries have been either promoted to `verified` (with citations and production detectors) or removed from the registry entirely. There are no `research_needed` entries remaining.

| # | Tool | Slug | Tier | Status |
| --- | --- | --- | --- | --- |
| 1 | Spec Kitty | [`spec-kitty.md`](./spec-kitty.md) | first-class | verified |
| 2 | GitHub Spec Kit | [`github-spec-kit.md`](./github-spec-kit.md) | first-class | verified |
| 3 | OpenSpec | [`openspec.md`](./openspec.md) | first-class | verified |
| 4 | Kiro | [`kiro.md`](./kiro.md) | second-ring | verified |
| 5 | BMAD-METHOD | [`bmad.md`](./bmad.md) | second-ring | verified |
| 6 | GSD | [`gsd.md`](./gsd.md) | second-ring | verified |
| 7 | Spec Workflow MCP | [`spec-workflow-mcp.md`](./spec-workflow-mcp.md) | long-tail | verified |
| 8 | SDD Pilot | [`sdd-pilot.md`](./sdd-pilot.md) | long-tail | verified |
| 9 | spec2ship | [`spec2ship.md`](./spec2ship.md) | long-tail | verified |
| 10 | ChatDev | [`chatdev.md`](./chatdev.md) | long-tail | verified |
| 11 | PAUL | [`paul.md`](./paul.md) | long-tail | verified |
| 12 | fspec | [`fspec.md`](./fspec.md) | long-tail | verified |
| 13 | Cognition / Devin | [`cognition-devin.md`](./cognition-devin.md) | long-tail | verified |
| 14 | Microsoft Agent Framework | [`microsoft-agent-framework.md`](./microsoft-agent-framework.md) | long-tail | verified |
| 15 | Tessl | [`tessl.md`](./tessl.md) | long-tail | verified |

**Verified: 15 / 15. Research-needed: 0 / 15.**

## Removed tools (no public-source fingerprint surface)

The following tools appeared on the initial top-20 brief but were removed from the registry during the post-mission re-research pass. Each was searched freshly; none of them had a canonical upstream repository or documented public artifact set that could anchor a tool-specific detector. Per the project rule, what cannot be verified from public sources is scratched from the list rather than left in a permanent `research_needed` state.

| Tool | Reason for removal |
| --- | --- |
| Spec-Driven Develop | The name is the category descriptor itself ("spec-driven development"). The only candidate repos either map back to GitHub Spec Kit (already covered) or are MCP servers (e.g., `formulahendry/mcp-server-spec-driven-development`) whose generic naming would conflict with the descriptor; no canonical, tool-specific marker set exists. |
| whenwords | No canonical upstream repository, npm/PyPI package, or product page found. |
| Intent | Augment Code ships a hosted product called "Intent" with an `auggie` CLI, but the product is a hosted workspace and documents no local config directory or workspace-local artifact surface that a static text analyzer can detect. Combined with the extreme false-positive risk of the generic word "intent" (Android Intent, NLP intent classification, etc.), the tool is unfingerprintable from public sources. |
| Agentic Code | The phrase is a category descriptor. No canonical upstream product with that exact name. |
| CodeSpeak | No canonical upstream repository or product page that ships a fingerprintable artifact set. |

## Reviewer quickcheck

```sh
grep -l "^- Status: verified" docs/research/sdd-fingerprints/*.md | wc -l
# Should match the verified count in the table above (15 as of the latest pass).
```

## Cross-link

- Mission spec: [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md`](../../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md)
- Research notes: [`kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md`](../../../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/research.md)
- WP09 consumer-facing registry: `docs/sdd-fingerprint-registry.md` (created by WP09)
