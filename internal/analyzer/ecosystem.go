package analyzer

import (
	"bytes"
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer/sdd"
)

type signature struct {
	id       string
	patterns []*regexp.Regexp
}

// init wires the SDD chunk loader so the sdd package can read the embedded
// signatures without importing analyzer (which would create an import cycle
// because this file now imports sdd). Assigning ChunksProvider here is the
// single source of truth; tests in the sdd package leave it nil and rely on
// the empty-registry path.
func init() {
	sdd.ChunksProvider = SDDDetectorChunks
}

func DetectEcosystem(input []byte, lines []parsedLine) Ecosystem {
	text := string(input)
	registry := ecosystemRegistry()
	codingAgents := detectMany(text, registry.CodingAgents)
	eco := Ecosystem{
		Client:             primaryClient(codingAgents),
		CodingAgents:       codingAgents,
		OperatingSystem:    detectOS(text),
		Shell:              detectShell(text),
		WorkflowFrameworks: detectMany(text, registry.Frameworks),
		MCPServersKnown:    detectMany(text, registry.MCPServers),
		KnownSkills:        detectMany(text, registry.Skills),
		KnownPlugins:       detectMany(text, registry.Plugins),
		PackageManagers:    detectMany(text, registry.PackageManagers),
		VersionControl:     detectVCS(text),
	}
	eco.UnknownMCPServerCount = countUnknownMCP(input, eco.MCPServersKnown)
	eco.UnknownSkillCount = countUnknownSlashCommands(lines, registry.KnownSlashCommandIDs())

	// SDD fingerprint pass (WP03). The probe call site is bounded by a 5s
	// ceiling per NFR-002; individual probe.Version invocations should set
	// tighter per-call deadlines as the registry grows.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe := sdd.NewRealProbe()
	slashHits := extractSlashTokens(lines)
	sddFps := sdd.Evaluate(ctx, text, slashHits, probe, sdd.LoadRegistry())
	if len(sddFps) > 0 {
		fps := make([]EcosystemFingerprint, len(sddFps))
		for i, f := range sddFps {
			fps[i] = EcosystemFingerprint{
				ID:            f.ID,
				Confidence:    f.Confidence,
				Sources:       f.Sources,
				EvidenceCount: f.EvidenceCount,
				Active:        f.Active,
				Installed:     f.Installed,
				VersionBucket: f.VersionBucket,
			}
		}
		eco.WorkflowFingerprints = fps
	}

	return eco
}

// extractSlashTokens walks parsed lines and returns the deduplicated,
// lowercased list of slash-prefixed command tokens (e.g. "/specify" → "specify").
// Modeled on countUnknownSlashCommands but returns the tokens instead of a
// count, for use by the SDD evaluator's slash_command markers. Tool-emitted
// lines are skipped to keep this aligned with the user-text surface.
func extractSlashTokens(lines []parsedLine) []string {
	re := regexp.MustCompile(`(?:^|[\s"'(:])/(?:[A-Za-z][A-Za-z0-9_-]{2,})`)
	seen := map[string]bool{}
	for _, line := range lines {
		if line.IsTool {
			continue
		}
		for _, raw := range re.FindAllString(line.Text, -1) {
			raw = strings.TrimLeft(raw, " \t\n\r\"'(:")
			name := strings.TrimPrefix(strings.ToLower(raw), "/")
			if name == "" {
				continue
			}
			seen[name] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func detectMany(text string, signatures []signature) []string {
	seen := map[string]bool{}
	for _, signature := range signatures {
		for _, pattern := range signature.patterns {
			if pattern.MatchString(text) {
				seen[signature.id] = true
				break
			}
		}
	}
	return sortedKeys(seen)
}

func primaryClient(codingAgents []string) string {
	if len(codingAgents) == 0 {
		return "unknown"
	}
	return codingAgents[0]
}

func detectOS(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "darwin") || strings.Contains(lower, "macos"):
		return "macos"
	case strings.Contains(lower, "wsl"):
		return "wsl"
	case strings.Contains(lower, "windows") || strings.Contains(lower, "powershell"):
		return "windows"
	case strings.Contains(lower, "linux") || strings.Contains(lower, "/home/"):
		return "linux"
	default:
		return "unknown"
	}
}

func detectShell(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "zsh"):
		return "zsh"
	case strings.Contains(lower, "bash"):
		return "bash"
	case strings.Contains(lower, "fish"):
		return "fish"
	case strings.Contains(lower, "powershell") || strings.Contains(lower, "pwsh"):
		return "powershell"
	default:
		return "unknown"
	}
}

func detectVCS(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "jj "):
		return "jj"
	case strings.Contains(lower, "git "):
		return "git"
	default:
		return "unknown"
	}
}

func countUnknownMCP(input []byte, known []string) int {
	re := regexp.MustCompile(`mcp__([A-Za-z0-9_-]+)__`)
	knownSet := map[string]bool{}
	for _, item := range known {
		knownSet[normalizeID(item)] = true
	}
	unknown := map[string]bool{}
	for _, match := range re.FindAllSubmatch(input, -1) {
		if len(match) < 2 {
			continue
		}
		name := normalizeID(string(bytes.ToLower(match[1])))
		if !knownSet[name] {
			unknown[name] = true
		}
	}
	return len(unknown)
}

func countUnknownSlashCommands(lines []parsedLine, known []string) int {
	re := regexp.MustCompile(`(?:^|[\s"'(:])/(?:[A-Za-z][A-Za-z0-9_-]{2,})`)
	knownSet := map[string]bool{}
	for _, item := range known {
		knownSet[normalizeID(item)] = true
	}
	unknown := map[string]bool{}
	for _, line := range lines {
		if line.IsTool {
			continue
		}
		for _, raw := range re.FindAllString(line.Text, -1) {
			raw = strings.TrimLeft(raw, " \t\n\r\"'(:")
			matchEnd := strings.Index(line.Text, raw) + len(raw)
			if matchEnd > len(raw)-1 && matchEnd < len(line.Text) && line.Text[matchEnd] == '/' {
				continue
			}
			name := strings.TrimPrefix(strings.ToLower(raw), "/")
			name = strings.TrimPrefix(name, "gstack-")
			if !knownSet[normalizeID(name)] {
				unknown[normalizeID(name)] = true
			}
		}
	}
	return len(unknown)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizeID(value string) string {
	return strings.ReplaceAll(strings.ToLower(value), "-", "_")
}
