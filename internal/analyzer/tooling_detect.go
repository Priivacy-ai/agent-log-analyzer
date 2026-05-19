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

// byteRange is a [Start, End) byte offset pair inside a parsed log buffer.
// In-memory only — never written to disk, JSON, logs, or telemetry. Used by
// the call detector to skip raw-byte rescan matches whose offsets fall inside
// an exposure-header block (see issue #70 / FR-005).
type byteRange struct {
	Start int
	End   int
}

// mcpExposure summarises MCP servers/tools exposed in a transcript header.
// Unknown names are counted but never stored.
type mcpExposure struct {
	KnownIDs         []string // sorted, lowercased, underscored — allowlist IDs only
	UnknownCount     int
	ExposedToolCount int  // 0 when only server count is known
	ExposedToolKnown bool // true when a header block was parsed
	SchemaTextBytes  int  // bytes of header block; 0 if no header
	InferenceSource  string
	// HeaderRanges records the byte offsets of detected exposure-header blocks
	// in the original input. In-memory only; no JSON tag, no serialization.
	// Consumed by detectMCPCallsFromToolUse to mask raw-byte rescan matches
	// that fall inside a header (see FR-005). Empty slice ⇒ rescan unchanged.
	HeaderRanges []byteRange
}

// skillExposure summarises skills exposed in a transcript header.
// Unknown names are counted but never stored.
type skillExposure struct {
	KnownIDs        []string
	UnknownCount    int
	SchemaTextBytes int
	InferenceSource string
	// HeaderRanges records the byte offsets of detected skill exposure-header
	// blocks in the original input. In-memory only; defensive symmetry with
	// mcpExposure.HeaderRanges so the call detector can mask candidate offsets
	// against the combined MCP+skill range set.
	HeaderRanges []byteRange
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
	var headerRanges []byteRange

	for _, pattern := range mcpHeaderPatterns {
		for _, loc := range pattern.FindAllIndex(input, -1) {
			matched = true
			block := extractHeaderBlock(input, loc[1])
			totalBytes += len(block)
			// Record the block's byte range in the original input. The block
			// starts immediately after the matched header phrase (loc[1]) and
			// runs for len(block) bytes. Used downstream by the call detector
			// to suppress double-counting of mcp__server__tool tokens that
			// only appear because they're advertised inside this header.
			if len(block) > 0 {
				headerRanges = append(headerRanges, byteRange{Start: loc[1], End: loc[1] + len(block)})
			}

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
	out.HeaderRanges = headerRanges
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
	var headerRanges []byteRange

	for _, pattern := range skillHeaderPatterns {
		for _, loc := range pattern.FindAllIndex(input, -1) {
			matched = true
			block := extractHeaderBlock(input, loc[1])
			totalBytes += len(block)
			// Defensive symmetry with detectMCPExposureFromHeaders: record the
			// block's byte range so the call detector can mask any candidate
			// mcp__server__tool offsets that happen to fall inside a skill
			// exposure header (today's skill headers don't carry such tokens,
			// but a future schema change could).
			if len(block) > 0 {
				headerRanges = append(headerRanges, byteRange{Start: loc[1], End: loc[1] + len(block)})
			}

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
	out.HeaderRanges = headerRanges
	return out
}

// insideAny reports whether off lies inside any range in ranges. The check is
// half-open: an offset equal to a range's End is NOT inside it. Runs in
// O(len(ranges)); header range count is bounded and small (typically 0..3),
// so this stays cheap inside the per-match hot loop of the call detector.
//
// When ranges is empty, the result is always false, so the masked rescan is
// byte-identical to an unmasked rescan. This is the load-bearing C-006 no-op
// guarantee for fixtures without exposure headers.
func insideAny(off int, ranges []byteRange) bool {
	for _, r := range ranges {
		if off >= r.Start && off < r.End {
			return true
		}
	}
	return false
}

// detectMCPCallsFromToolUse counts MCP server/tool invocations across the raw
// input and via parsed tool-use lines.
//
// Raw-byte rescan matches whose start offsets fall inside an MCP or skill
// exposure-header byte range are masked out: those tokens are advertisements
// (the header enumerating which tools are available), not actual calls. This
// fixes the issue #70 double-count where a `mcp__server__tool` identifier
// announced in a "Following deferred tools are now available" block was
// counted as a call even when zero tool_use records existed (FR-005).
//
// Parsed-line tool-use scans are outside header bytes by construction and
// remain unmasked.
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

	// Compute the combined exposure-header byte-range set once per call. We
	// re-run the header detectors here (rather than threading them through the
	// signature) so this fix stays contained to tooling_detect.go. The header
	// detectors are linear in len(input) and run at most a handful of regex
	// passes; the cost is negligible against the existing scan budget.
	//
	// If no exposure headers are present, combined is empty and insideAny
	// trivially returns false for every offset — preserving C-006 no-op
	// stability against the current behavior.
	mcpEx := detectMCPExposureFromHeaders(input, registry)
	skillEx := detectSkillExposureFromHeaders(input, registry)
	combined := make([]byteRange, 0, len(mcpEx.HeaderRanges)+len(skillEx.HeaderRanges))
	combined = append(combined, mcpEx.HeaderRanges...)
	combined = append(combined, skillEx.HeaderRanges...)

	// 1. Scan raw input for mcp__server__tool patterns. We use
	// FindAllSubmatchIndex (not FindAllSubmatch) so we have each match's
	// start offset and can mask matches that fall inside an exposure header.
	for _, idx := range mcpCallPairRe.FindAllSubmatchIndex(input, -1) {
		if len(idx) < 6 {
			continue
		}
		// Mask: skip raw-byte matches whose start offset lies inside any
		// exposure-header range. This is the actual fix for issue #70.
		if insideAny(idx[0], combined) {
			continue
		}
		server := input[idx[2]:idx[3]]
		tool := input[idx[4]:idx[5]]
		record(string(server), string(tool))
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
