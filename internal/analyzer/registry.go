package analyzer

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"sync"
)

//go:embed signatures/*.json
var signatureFS embed.FS

// SDDDetectorChunks returns the raw bytes of every embedded JSON file
// matching `signatures/sdd_detectors*.json`, ordered by filename. Each
// chunk is expected to decode as a JSON array of detector records; the
// `sdd` package concatenates them at load time. Exposing this as a sibling
// of `signatureFS` keeps a single source of truth for the embed pattern
// rather than re-embedding inside `sdd`.
func SDDDetectorChunks() [][]byte {
	matches, err := fs.Glob(signatureFS, "signatures/sdd_detectors*.json")
	if err != nil {
		panic(fmt.Errorf("sdd: glob embedded signatures: %w", err))
	}
	sort.Strings(matches)
	chunks := make([][]byte, 0, len(matches))
	for _, path := range matches {
		data, err := signatureFS.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("sdd: read embedded %s: %w", path, err))
		}
		chunks = append(chunks, data)
	}
	return chunks
}

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
