package analyzer

import (
	"embed"
	"encoding/json"
	"regexp"
	"sync"
)

//go:embed signatures/*.json
var signatureFS embed.FS

type signatureRegistry struct {
	Frameworks      []signature
	MCPServers      []signature
	Skills          []signature
	Plugins         []signature
	CodingAgents    []signature
	PackageManagers []signature
	SlashCommandIDs []string
}

type signatureSpec struct {
	ID       string   `json:"id"`
	Patterns []string `json:"patterns"`
}

var (
	registryOnce sync.Once
	registryData signatureRegistry
)

func ecosystemRegistry() signatureRegistry {
	registryOnce.Do(func() {
		registryData = signatureRegistry{
			Frameworks:      loadSignatures("signatures/frameworks.json"),
			MCPServers:      loadSignatures("signatures/mcp_servers.json"),
			Skills:          loadSignatures("signatures/skills.json"),
			Plugins:         loadSignatures("signatures/plugins.json"),
			CodingAgents:    loadSignatures("signatures/coding_agents.json"),
			PackageManagers: loadSignatures("signatures/package_managers.json"),
		}
		for _, group := range [][]signature{registryData.Skills, registryData.Frameworks} {
			for _, signature := range group {
				registryData.SlashCommandIDs = append(registryData.SlashCommandIDs, signature.id)
			}
		}
	})
	return registryData
}

func (r signatureRegistry) KnownSlashCommandIDs() []string {
	return r.SlashCommandIDs
}

func loadSignatures(path string) []signature {
	data, err := signatureFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var specs []signatureSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		panic(err)
	}
	signatures := make([]signature, 0, len(specs))
	for _, spec := range specs {
		compiled := make([]*regexp.Regexp, 0, len(spec.Patterns))
		for _, pattern := range spec.Patterns {
			compiled = append(compiled, regexp.MustCompile(pattern))
		}
		signatures = append(signatures, signature{id: spec.ID, patterns: compiled})
	}
	return signatures
}
