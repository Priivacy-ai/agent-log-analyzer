package analyzer

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
)

type signature struct {
	id       string
	patterns []*regexp.Regexp
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
	return eco
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
