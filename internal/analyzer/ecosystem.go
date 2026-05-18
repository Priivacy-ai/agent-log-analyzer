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

var frameworkSignatures = []signature{
	sig("bmad", `(?i)\bBMAD\b`, `(?i)bmad-method`),
	sig("openspec", `(?i)\bOpenSpec\b`, `(?i)\bopenspec\b`),
	sig("spec_kit", `(?i)\bSpec Kit\b`, `(?i)\bspec-kit\b`),
	sig("spec_kitty", `(?i)\bSpec Kitty\b`, `(?i)\bspec-kitty\b`, `(?i)\.spec-kitty`),
	sig("superpowers", `(?i)\bsuperpowers\b`, `(?i)obra/superpowers`),
}

var mcpSignatures = []signature{
	sig("github", `mcp__github__`, `(?i)\bmcp.*github\b`),
	sig("filesystem", `mcp__filesystem__`, `(?i)\bmcp.*filesystem\b`),
	sig("playwright", `mcp__playwright__`, `(?i)\bmcp.*playwright\b`),
	sig("browser", `mcp__browser__`, `(?i)\bmcp.*browser\b`),
	sig("context7", `mcp__context7__`, `(?i)\bcontext7\b`),
	sig("sequential_thinking", `mcp__sequential[_-]?thinking__`, `(?i)\bsequential thinking\b`),
	sig("notion", `mcp__notion__`, `(?i)\bmcp.*notion\b`),
	sig("linear", `mcp__linear__`, `(?i)\bmcp.*linear\b`),
	sig("slack", `mcp__slack__`, `(?i)\bmcp.*slack\b`),
	sig("figma", `mcp__figma__`, `(?i)\bmcp.*figma\b`),
	sig("postgres", `mcp__postgres__`, `(?i)\bmcp.*postgres\b`),
	sig("supabase", `mcp__supabase__`, `(?i)\bmcp.*supabase\b`),
}

var skillSignatures = []signature{
	sig("qa", `(?i)(^|\s)/(qa|gstack-qa)\b`, `(?i)\bskill.?qa\b`),
	sig("ship", `(?i)(^|\s)/(ship|gstack-ship)\b`, `(?i)\bskill.?ship\b`),
	sig("review", `(?i)(^|\s)/(review|gstack-review)\b`, `(?i)\bskill.?review\b`),
	sig("investigate", `(?i)(^|\s)/(investigate|gstack-investigate)\b`),
	sig("security", `(?i)(^|\s)/(security|security-review|cso)\b`),
}

var pluginSignatures = []signature{
	sig("browser", `(?i)\bplugin.?browser\b`, `(?i)@browser`),
	sig("notion", `(?i)\bplugin.?notion\b`, `(?i)@notion`),
	sig("github", `(?i)\bplugin.?github\b`, `(?i)@github`),
}

func sig(id string, patterns ...string) signature {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(pattern))
	}
	return signature{id: id, patterns: compiled}
}

func DetectEcosystem(input []byte, lines []parsedLine) Ecosystem {
	text := string(input)
	eco := Ecosystem{
		Client:             detectClient(text),
		OperatingSystem:    detectOS(text),
		Shell:              detectShell(text),
		WorkflowFrameworks: detectMany(text, frameworkSignatures),
		MCPServersKnown:    detectMany(text, mcpSignatures),
		KnownSkills:        detectMany(text, skillSignatures),
		KnownPlugins:       detectMany(text, pluginSignatures),
		PackageManagers:    detectPackageManagers(text),
		VersionControl:     detectVCS(text),
	}
	eco.UnknownMCPServerCount = countUnknownMCP(input, eco.MCPServersKnown)
	eco.UnknownSkillCount = countUnknownSlashCommands(lines, eco.KnownSkills)
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

func detectClient(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "claude code"):
		return "claude_code"
	case strings.Contains(lower, "codex"):
		return "codex"
	case strings.Contains(lower, "cursor"):
		return "cursor"
	case strings.Contains(lower, "windsurf"):
		return "windsurf"
	default:
		return "unknown"
	}
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

func detectPackageManagers(text string) []string {
	signatures := []signature{
		sig("npm", `\bnpm\b`, `package-lock\.json`),
		sig("pnpm", `\bpnpm\b`, `pnpm-lock\.yaml`),
		sig("yarn", `\byarn\b`, `yarn\.lock`),
		sig("bun", `\bbun\b`, `bun\.lockb?`),
		sig("uv", `\buv\b`, `uv\.lock`),
		sig("pip", `\bpip\b`, `requirements\.txt`),
		sig("poetry", `\bpoetry\b`, `poetry\.lock`),
		sig("cargo", `\bcargo\b`, `Cargo\.toml`),
		sig("go", `\bgo test\b`, `go\.mod`),
	}
	return detectMany(text, signatures)
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
		knownSet[item] = true
	}
	unknown := map[string]bool{}
	for _, match := range re.FindAllSubmatch(input, -1) {
		if len(match) < 2 {
			continue
		}
		name := string(bytes.ToLower(match[1]))
		if !knownSet[name] {
			unknown[name] = true
		}
	}
	return len(unknown)
}

func countUnknownSlashCommands(lines []parsedLine, known []string) int {
	re := regexp.MustCompile(`/(?:[A-Za-z][A-Za-z0-9_-]{2,})`)
	knownSet := map[string]bool{}
	for _, item := range known {
		knownSet[item] = true
	}
	unknown := map[string]bool{}
	for _, line := range lines {
		for _, raw := range re.FindAllString(line.Text, -1) {
			name := strings.TrimPrefix(strings.ToLower(raw), "/")
			if !knownSet[name] {
				unknown[name] = true
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
