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

