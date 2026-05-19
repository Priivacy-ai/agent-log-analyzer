package analyzer

// Pure detection layer for MCP/skill exposure, calls, and footprint.
//
// PRIVACY: every function in this file returns only counts, allowlist IDs,
// and closed-enum labels. Names of unknown MCP servers, unknown skills, raw
// schema text, file paths, and user content are NEVER stored on the returned
// structs. Unknown counters increment local maps that are discarded before
// return.

import (
	"bytes"
	"regexp"
	"strings"
)

// mcpExposure summarises MCP servers/tools exposed in a transcript header.
// Unknown names are counted but never stored.
type mcpExposure struct {
	KnownIDs         []string // sorted, lowercased, underscored — allowlist IDs only
	UnknownCount     int
	ExposedToolCount int  // 0 when only server count is known
	ExposedToolKnown bool // true when a header block was parsed
	SchemaTextBytes  int  // bytes of header block; 0 if no header
	InferenceSource  string
}

// skillExposure summarises skills exposed in a transcript header.
// Unknown names are counted but never stored.
type skillExposure struct {
	KnownIDs        []string
	UnknownCount    int
	SchemaTextBytes int
	InferenceSource string
}

// mcpCalls summarises MCP tool invocations observed across the transcript.
// Unknown names are counted but never stored.
type mcpCalls struct {
	TotalCalls         int
	KnownCallCount     int
	UnknownCallCount   int
	UniqueKnownIDs     []string // sorted, allowlist IDs only
	UniqueUnknownCount int
	UniqueServerCount  int // distinct server names observed
	UniqueToolCount    int // distinct server::tool pairs observed
}

// skillExecutions summarises skill executions detected from parsed lines.
// Unknown names are counted but never stored.
type skillExecutions struct {
	ExecutedCount    int
	KnownExecutedIDs []string // sorted
	UnknownExecuted  int      // distinct unknown names, count only
}

// Fixed per-item token-cost constants for the fallback footprint path
// (see plan §D-2).
const (
	mcpServerOverheadTokens = 250
	mcpToolTokens           = 150
	skillTokens             = 400
)

// Header pattern compilation (case-insensitive). Compiled at package init.
var (
	mcpHeaderPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)available mcp servers?:`),
		regexp.MustCompile(`(?i)mcp tools? available`),
		regexp.MustCompile(`(?i)following deferred tools? are now available`),
	}
	skillHeaderPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)following skills are available`),
		regexp.MustCompile(`(?i)available skills:`),
	}

	// Conservative bullet-style candidate extractor used inside header blocks.
	// Captures the leading identifier from lines like "- foo", "* bar", "• baz",
	// or "  qux:" / "  qux -- description".
	bulletCandidateRe = regexp.MustCompile(`^[\s\-*•]*([a-z0-9_][a-z0-9_:-]+)`)

	// Reused: matches an mcp__server__tool token (server and tool captured).
	// Same shape as ecosystem.go:101 with the tool segment captured.
	mcpCallPairRe = regexp.MustCompile(`mcp__([A-Za-z0-9_-]+)__([A-Za-z0-9_-]+)`)

	// Path-avoidance regex — byte-for-byte identical to ecosystem.go:120.
	// Do not rewrite for cleanliness; downstream behaviour depends on this
	// exact shape.
	slashCommandRe = regexp.MustCompile(`(?:^|[\s"'(:])/(?:[A-Za-z][A-Za-z0-9_-]{2,})`)
)

// detectMCPExposureFromHeaders scans the transcript for MCP availability
// headers and returns counts + allowlist IDs. Unknown names are counted but
// never stored.
func detectMCPExposureFromHeaders(input []byte, registry signatureRegistry) mcpExposure {
	out := mcpExposure{}
	if len(input) == 0 {
		return out
	}
	known := normalizedAllowlistSet(idsFromSignatures(registry.MCPServers))
	knownIDs := map[string]bool{}
	unknownIDs := map[string]bool{}
	exposedTools := map[string]bool{}
	matched := false
	totalBytes := 0

	for _, pattern := range mcpHeaderPatterns {
		for _, loc := range pattern.FindAllIndex(input, -1) {
			matched = true
			block := extractHeaderBlock(input, loc[1])
			totalBytes += len(block)

			// Count mcp__server__tool tokens inside the block.
			for _, m := range mcpCallPairRe.FindAllSubmatch(block, -1) {
				if len(m) < 3 {
					continue
				}
				server := normalizeID(strings.ToLower(string(m[1])))
				tool := strings.ToLower(string(m[2]))
				exposedTools[server+"::"+tool] = true
			}

			// Walk lines for bullet-style entries.
			for _, raw := range bytes.Split(block, []byte("\n")) {
				lower := bytes.ToLower(bytes.TrimRight(raw, "\r"))
				m := bulletCandidateRe.FindSubmatch(lower)
				if len(m) < 2 {
					continue
				}
				candidate := normalizeID(string(m[1]))
				if candidate == "" {
					continue
				}
				// Skip noise tokens that frequently appear in headers but
				// are not server IDs (e.g. the leading "mcp" word, blank).
				if candidate == "mcp" || candidate == "available" {
					continue
				}
				if known[candidate] {
					knownIDs[candidate] = true
				} else {
					unknownIDs[candidate] = true
				}
			}
		}
	}

	if !matched {
		return out
	}
	out.KnownIDs = sortedKeys(knownIDs)
	out.UnknownCount = len(unknownIDs)
	out.SchemaTextBytes = totalBytes
	out.InferenceSource = InferenceSourceHeader
	out.ExposedToolKnown = true
	out.ExposedToolCount = len(exposedTools)
	// Discard unknownIDs map before return — names never leave this function.
	return out
}

// detectSkillExposureFromHeaders scans the transcript for skill availability
// headers and returns counts + allowlist IDs.
func detectSkillExposureFromHeaders(input []byte, registry signatureRegistry) skillExposure {
	out := skillExposure{}
	if len(input) == 0 {
		return out
	}
	known := normalizedAllowlistSet(registry.KnownSlashCommandIDs())
	knownIDs := map[string]bool{}
	unknownIDs := map[string]bool{}
	matched := false
	totalBytes := 0

	for _, pattern := range skillHeaderPatterns {
		for _, loc := range pattern.FindAllIndex(input, -1) {
			matched = true
			block := extractHeaderBlock(input, loc[1])
			totalBytes += len(block)

			for _, raw := range bytes.Split(block, []byte("\n")) {
				lower := bytes.ToLower(bytes.TrimRight(raw, "\r"))
				m := bulletCandidateRe.FindSubmatch(lower)
				if len(m) < 2 {
					continue
				}
				candidate := normalizeID(strings.TrimPrefix(string(m[1]), "gstack-"))
				if candidate == "" {
					continue
				}
				if candidate == "available" {
					continue
				}
				if known[candidate] {
					knownIDs[candidate] = true
				} else {
					unknownIDs[candidate] = true
				}
			}
		}
	}

	if !matched {
		return out
	}
	out.KnownIDs = sortedKeys(knownIDs)
	out.UnknownCount = len(unknownIDs)
	out.SchemaTextBytes = totalBytes
	out.InferenceSource = InferenceSourceHeader
	return out
}

// detectMCPCallsFromToolUse counts MCP server/tool invocations across the raw
// input and via parsed tool-use lines.
func detectMCPCallsFromToolUse(input []byte, lines []parsedLine, registry signatureRegistry) mcpCalls {
	out := mcpCalls{}
	known := normalizedAllowlistSet(idsFromSignatures(registry.MCPServers))

	uniqueKnown := map[string]bool{}
	uniqueUnknown := map[string]bool{}
	uniqueServers := map[string]bool{}
	uniquePairs := map[string]bool{}

	record := func(serverRaw, toolRaw string) {
		server := normalizeID(strings.ToLower(serverRaw))
		tool := strings.ToLower(toolRaw)
		if server == "" {
			return
		}
		out.TotalCalls++
		uniqueServers[server] = true
		uniquePairs[server+"::"+tool] = true
		if known[server] {
			out.KnownCallCount++
			uniqueKnown[server] = true
		} else {
			out.UnknownCallCount++
			uniqueUnknown[server] = true
		}
	}

	// 1. Scan raw input for mcp__server__tool patterns.
	for _, m := range mcpCallPairRe.FindAllSubmatch(input, -1) {
		if len(m) < 3 {
			continue
		}
		record(string(m[1]), string(m[2]))
	}

	// 2. Scan parsed tool-use lines where ToolName starts with mcp__.
	for _, line := range lines {
		if !line.IsTool {
			continue
		}
		if !strings.HasPrefix(line.ToolName, "mcp__") {
			continue
		}
		for _, m := range mcpCallPairRe.FindAllStringSubmatch(line.ToolName, -1) {
			if len(m) < 3 {
				continue
			}
			record(m[1], m[2])
		}
	}

	out.UniqueKnownIDs = sortedKeys(uniqueKnown)
	out.UniqueUnknownCount = len(uniqueUnknown)
	out.UniqueServerCount = len(uniqueServers)
	out.UniqueToolCount = len(uniquePairs)
	// uniqueUnknown discarded; names never leave this function.
	return out
}

// detectSkillExecutionsFromLines counts skill executions while preserving the
// existing path-avoidance behaviour at ecosystem.go:119.
func detectSkillExecutionsFromLines(lines []parsedLine, registry signatureRegistry) skillExecutions {
	out := skillExecutions{}
	known := normalizedAllowlistSet(registry.KnownSlashCommandIDs())
	knownIDs := map[string]bool{}
	unknownIDs := map[string]bool{}

	for _, line := range lines {
		if line.IsTool {
			continue
		}
		for _, raw := range slashCommandRe.FindAllString(line.Text, -1) {
			// Trim/skip logic copied byte-for-byte from
			// ecosystem.go:131-135. Do not rewrite.
			raw = strings.TrimLeft(raw, " \t\n\r\"'(:")
			matchEnd := strings.Index(line.Text, raw) + len(raw)
			if matchEnd > len(raw)-1 && matchEnd < len(line.Text) && line.Text[matchEnd] == '/' {
				continue
			}
			name := strings.TrimPrefix(strings.ToLower(raw), "/")
			name = strings.TrimPrefix(name, "gstack-")
			normalised := normalizeID(name)
			if normalised == "" {
				continue
			}
			out.ExecutedCount++
			if known[normalised] {
				knownIDs[normalised] = true
			} else {
				unknownIDs[normalised] = true
			}
		}
	}

	out.KnownExecutedIDs = sortedKeys(knownIDs)
	out.UnknownExecuted = len(unknownIDs)
	// unknownIDs discarded; names never leave this function.
	return out
}

// estimateMCPFootprintTokens returns a hybrid token-count estimate for MCP
// exposure. Prefers measured schema bytes when available, falls back to a
// fixed per-item cost. Returns known=false when no signal is available.
func estimateMCPFootprintTokens(schemaBytes, serverCount, toolCount int) (int, bool) {
	if schemaBytes > 0 {
		return schemaBytes / 4, true
	}
	if serverCount >= 0 {
		toolPart := toolCount
		if toolPart < 0 {
			toolPart = 0
		}
		return serverCount*mcpServerOverheadTokens + toolPart*mcpToolTokens, true
	}
	return 0, false
}

// estimateSkillFootprintTokens returns a hybrid token-count estimate for skill
// exposure. Same algorithm as the MCP variant.
func estimateSkillFootprintTokens(schemaBytes, skillCount int) (int, bool) {
	if schemaBytes > 0 {
		return schemaBytes / 4, true
	}
	if skillCount >= 0 {
		return skillCount * skillTokens, true
	}
	return 0, false
}

// extractHeaderBlock returns the bytes following position `start` up to the
// next blank line or up to 200 lines, whichever comes first. The returned
// slice is a sub-slice of input.
func extractHeaderBlock(input []byte, start int) []byte {
	if start < 0 || start >= len(input) {
		return nil
	}
	const maxLines = 200
	rest := input[start:]
	lineCount := 0
	pos := 0
	for pos < len(rest) {
		// Find next newline.
		nl := bytes.IndexByte(rest[pos:], '\n')
		if nl < 0 {
			pos = len(rest)
			break
		}
		end := pos + nl
		// Trim trailing CR.
		trimmed := bytes.TrimRight(rest[pos:end], "\r ")
		if len(trimmed) == 0 && lineCount > 0 {
			// Blank line terminates the block (but allow the first line, which
			// is the rest-of-header line after the matched phrase).
			return rest[:pos]
		}
		lineCount++
		pos = end + 1
		if lineCount >= maxLines {
			return rest[:pos]
		}
	}
	return rest[:pos]
}

// idsFromSignatures returns the IDs of a signature slice.
func idsFromSignatures(sigs []signature) []string {
	out := make([]string, 0, len(sigs))
	for _, s := range sigs {
		out = append(out, s.id)
	}
	return out
}

// normalizedAllowlistSet builds a set of normalised IDs.
func normalizedAllowlistSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[normalizeID(id)] = true
	}
	return set
}
