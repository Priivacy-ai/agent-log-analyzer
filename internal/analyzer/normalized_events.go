package analyzer

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type normalizedEvent struct {
	Source              string
	Role                string
	Kind                string
	Tool                string
	ToolArgsHash        string
	CallIDHash          string
	ParentIDHash        string
	ToolOutputBytes     int
	TokensIn            int
	TokensCachedIn      int
	TokensCacheCreation int
	TokensOut           int
	PatchLinesAdded     int
	PatchLinesRemoved   int
	Error               bool
	Turn                int
}

func normalizeEvents(source string, input []byte) []normalizedEvent {
	if source == "" {
		source = "unknown"
	}
	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	var events []normalizedEvent
	turn := 0
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			turn++
			events = append(events, normalizedEvent{
				Source:          source,
				Role:            "unknown",
				Kind:            "message",
				ToolOutputBytes: len(raw),
				Turn:            turn,
			})
			continue
		}
		turn++
		events = append(events, normalizeJSONObject(source, obj, turn)...)
	}
	return events
}

func normalizeJSONObject(source string, obj map[string]any, turn int) []normalizedEvent {
	role := boundedRole(firstJSONStringByKey(obj, "role"))
	kind := boundedKind(firstJSONStringByKey(obj, "type"))
	if kind == "" {
		kind = boundedKind(firstJSONStringByKey(obj, "kind"))
	}
	if kind == "" {
		kind = "message"
	}
	parentHash := hashIfPresent(firstJSONStringByKey(obj, "parentUuid"))
	if parentHash == "" {
		parentHash = hashIfPresent(firstJSONStringByKey(obj, "parentID"))
	}
	if parentHash == "" {
		parentHash = hashIfPresent(firstJSONStringByKey(obj, "parent_id"))
	}
	base := normalizedEvent{
		Source:       source,
		Role:         role,
		Kind:         kind,
		ParentIDHash: parentHash,
		Turn:         turn,
		Error:        jsonHasError(obj),
	}
	applyUsage(&base, obj)
	applyPatchStats(&base, obj)

	var events []normalizedEvent
	if baseHasSignal(base) || base.Kind == "message" || base.Kind == "token_count" || base.Kind == "patch" || base.Kind == "compact" || base.Kind == "subagent" {
		events = append(events, base)
	}
	if tool := extractToolEvent(source, obj, base); tool.Kind != "" {
		events = append(events, tool)
	}
	for _, nested := range nestedToolEvents(source, obj, base) {
		if nested.Kind != "" {
			events = append(events, nested)
		}
	}
	if len(events) == 0 || len(events) == 1 && events[0].Kind == "message" && !baseHasSignal(events[0]) {
		if boundedKind(directString(obj, "type")) == "function_call" {
			base.Kind = "tool_call"
			base.Tool = boundedTool(directString(obj, "name"))
			base.ToolArgsHash = hashJSONValue(obj["arguments"])
			base.CallIDHash = hashIfPresent(directString(obj, "call_id"))
		}
		if boundedKind(directString(obj, "type")) == "function_call_output" || boundedKind(directString(obj, "type")) == "tool_result" {
			base.Kind = "tool_result"
			base.ToolOutputBytes = outputBytes(obj)
			base.CallIDHash = hashIfPresent(directString(obj, "call_id"))
		}
		events = []normalizedEvent{base}
	}
	return events
}

func extractToolEvent(source string, obj map[string]any, base normalizedEvent) normalizedEvent {
	event := base
	clearEventQuantities(&event)
	toolName := directString(obj, "tool")
	if toolName == "" {
		toolName = directString(obj, "name")
	}
	if toolName == "" {
		return normalizedEvent{}
	}
	directKind := boundedKind(directString(obj, "type"))
	if directKind == "tool_result" || directKind == "function_call_output" {
		event.Kind = "tool_result"
		event.ToolOutputBytes = outputBytes(obj)
	} else {
		event.Kind = "tool_call"
		event.ToolArgsHash = hashJSONValue(toolArgsValue(source, obj))
		if source == "opencode" {
			event.ToolOutputBytes = outputBytes(obj)
		}
	}
	event.Tool = boundedTool(toolName)
	event.CallIDHash = hashIfPresent(directString(obj, "callID"))
	if event.CallIDHash == "" {
		event.CallIDHash = hashIfPresent(directString(obj, "call_id"))
	}
	if event.CallIDHash == "" {
		event.CallIDHash = hashIfPresent(directString(obj, "tool_use_id"))
	}
	return event
}

func nestedToolEvents(source string, obj map[string]any, base normalizedEvent) []normalizedEvent {
	var events []normalizedEvent
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			kind := boundedKind(stringValue(typed["type"]))
			if kind == "tool_use" || kind == "tool_result" || kind == "function_call" || kind == "function_call_output" {
				nested := base
				clearEventQuantities(&nested)
				nested.Kind = kind
				if kind == "tool_use" || kind == "function_call" {
					nested.Kind = "tool_call"
					nested.Tool = boundedTool(firstPresentString(typed, "name", "tool"))
					nested.ToolArgsHash = hashJSONValue(firstPresentValue(typed, "input", "arguments", "tool_input"))
				} else {
					nested.Kind = "tool_result"
					nested.Tool = boundedTool(firstPresentString(typed, "name", "tool"))
					nested.ToolOutputBytes = outputBytes(typed)
					nested.Error = nested.Error || jsonHasError(typed)
				}
				nested.CallIDHash = hashIfPresent(firstPresentString(typed, "id", "call_id", "callID", "tool_use_id"))
				events = append(events, nested)
				return
			}
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(obj)
	return events
}

func baseHasSignal(event normalizedEvent) bool {
	return event.TokensIn != 0 ||
		event.TokensCachedIn != 0 ||
		event.TokensCacheCreation != 0 ||
		event.TokensOut != 0 ||
		event.PatchLinesAdded != 0 ||
		event.PatchLinesRemoved != 0
}

func clearEventQuantities(event *normalizedEvent) {
	event.TokensIn = 0
	event.TokensCachedIn = 0
	event.TokensCacheCreation = 0
	event.TokensOut = 0
	event.PatchLinesAdded = 0
	event.PatchLinesRemoved = 0
	event.ToolOutputBytes = 0
}

func signalsFromEvents(events []normalizedEvent, sessionCount int) AnalysisSignals {
	signals := AnalysisSignals{NormalizedEventCount: len(events)}
	if sessionCount <= 0 {
		sessionCount = 1
	}
	var previous *normalizedEvent
	var currentRetryDepth int
	for i := range events {
		event := events[i]
		signals.CacheReadTokens += event.TokensCachedIn
		signals.CacheCreationTokens += event.TokensCacheCreation
		signals.InputTokens += event.TokensIn
		signals.OutputTokens += event.TokensOut
		signals.ToolOutputBytes += event.ToolOutputBytes
		signals.PatchLinesAdded += event.PatchLinesAdded
		signals.PatchLinesRemoved += event.PatchLinesRemoved
		switch event.Kind {
		case "tool_call":
			signals.ToolCallCount++
		case "tool_result":
			signals.ToolResultCount++
		}
		if event.TokensCacheCreation >= 8000 {
			signals.CacheInvalidationEvents++
		}
		if previous != nil && isArgsHashedRetry(*previous, event) {
			currentRetryDepth++
			if currentRetryDepth > signals.ArgsHashedRetryLoops {
				signals.ArgsHashedRetryLoops = currentRetryDepth
			}
		} else if event.Kind == "tool_call" {
			currentRetryDepth = 0
		}
		if event.Kind == "tool_call" {
			copyEvent := event
			previous = &copyEvent
		} else if event.Error && previous != nil {
			previous.Error = true
		}
	}
	if signals.InputTokens > 0 {
		miss := max(0, signals.InputTokens-signals.CacheReadTokens)
		signals.CacheMissRatioPct = min(100, int(float64(miss)/float64(signals.InputTokens)*100))
	}
	signals.PatchLinesTouched = signals.PatchLinesAdded + signals.PatchLinesRemoved
	signals.PatchYieldPer1KTokens = patchYieldPer1KTokens(signals.PatchLinesTouched, totalTokenVolume(signals))
	signals.SampleConfidence, signals.SampleWarnings = sampleConfidence(sessionCount, signals)
	if signals.SampleWarnings == nil {
		signals.SampleWarnings = []string{}
	}
	return signals
}

func appendSignalFindings(findings []Finding, signals AnalysisSignals) []Finding {
	existing := map[string]bool{}
	for _, finding := range findings {
		existing[finding.ID] = true
	}
	if signals.ArgsHashedRetryLoops >= 2 && !existing["args_hashed_retry_loop"] {
		findings = append(findings, Finding{
			ID:         "args_hashed_retry_loop",
			Title:      "Repeated identical tool retries",
			Severity:   severity(signals.ArgsHashedRetryLoops, 2, 5),
			CostImpact: "medium-high",
			Evidence: FindingEvidence{
				Count:       signals.ArgsHashedRetryLoops,
				Description: "Same tool and sanitized argument hash repeated after failure",
			},
			Recommendation: "Stop repeated identical tool calls after failure; inspect the invariant or change the approach before retrying.",
			Deterministic:  true,
		})
	}
	if signals.CacheInvalidationEvents >= 1 && !existing["cache_invalidation_spike"] {
		findings = append(findings, Finding{
			ID:         "cache_invalidation_spike",
			Title:      "Cache prefix invalidation spike",
			Severity:   severity(signals.CacheInvalidationEvents, 1, 3),
			CostImpact: "medium-high",
			Evidence: FindingEvidence{
				Count:       signals.CacheInvalidationEvents,
				Description: "Large cache-creation token event; cache miss ratio " + formatSignalPct(signals.CacheMissRatioPct),
			},
			Recommendation: "Treat cache-creation spikes as context-boundary events; compact or split the session after major tool-output or configuration changes.",
			Deterministic:  true,
		})
	}
	return findings
}

func isArgsHashedRetry(previous, current normalizedEvent) bool {
	return previous.Kind == "tool_call" &&
		current.Kind == "tool_call" &&
		previous.Tool != "" &&
		previous.Tool == current.Tool &&
		previous.ToolArgsHash != "" &&
		previous.ToolArgsHash == current.ToolArgsHash &&
		(previous.ParentIDHash == "" || current.ParentIDHash == "" || previous.ParentIDHash == current.ParentIDHash) &&
		previous.Error
}

func sampleConfidence(sessionCount int, signals AnalysisSignals) (string, []string) {
	var warnings []string
	confidence := "high"
	if sessionCount < 3 {
		confidence = "low"
		warnings = append(warnings, "fewer_than_3_sessions")
	} else if sessionCount < 10 {
		confidence = "medium"
		warnings = append(warnings, "fewer_than_10_sessions")
	}
	if signals.NormalizedEventCount < 20 {
		confidence = "low"
		warnings = append(warnings, "thin_event_sample")
	}
	return confidence, warnings
}

func applyUsage(event *normalizedEvent, obj map[string]any) {
	lastUsage := firstJSONMapByKey(obj, "last_token_usage")
	if len(lastUsage) > 0 {
		event.TokensIn += intFromAny(lastUsage["input_tokens"])
		event.TokensCachedIn += intFromAny(lastUsage["cached_input_tokens"])
		event.TokensCachedIn += intFromAny(lastUsage["cache_read_input_tokens"])
		event.TokensCacheCreation += intFromAny(lastUsage["cache_creation_input_tokens"])
		event.TokensOut += intFromAny(lastUsage["output_tokens"])
		return
	}
	event.TokensIn += firstJSONIntByKey(obj, "input_tokens")
	event.TokensIn += firstJSONIntByKey(obj, "prompt_tokens")
	event.TokensCachedIn += firstJSONIntByKey(obj, "cache_read_input_tokens")
	event.TokensCachedIn += firstJSONIntByKey(obj, "cached_input_tokens")
	event.TokensCacheCreation += firstJSONIntByKey(obj, "cache_creation_input_tokens")
	event.TokensOut += firstJSONIntByKey(obj, "output_tokens")
	event.TokensOut += firstJSONIntByKey(obj, "completion_tokens")
}

func applyPatchStats(event *normalizedEvent, obj map[string]any) {
	event.PatchLinesAdded += firstJSONIntByKey(obj, "lines_added")
	event.PatchLinesAdded += firstJSONIntByKey(obj, "additions")
	event.PatchLinesRemoved += firstJSONIntByKey(obj, "lines_removed")
	event.PatchLinesRemoved += firstJSONIntByKey(obj, "deletions")
	event.PatchLinesRemoved += firstJSONIntByKey(obj, "removals")
}

func boundedRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user", "assistant", "system", "tool":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return "unknown"
	}
}

func boundedKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "assistant", "user", "system", "message", "text", "reasoning", "thinking":
		return "message"
	case "tool", "tool_use", "function_call":
		return kind
	case "tool_result", "function_call_output":
		return kind
	case "token_count":
		return "token_count"
	case "patch", "patch_apply_end":
		return "patch"
	case "compacted", "context_compacted":
		return "compact"
	case "spawn_agent", "wait_agent", "close_agent", "send_input", "agent":
		return "subagent"
	default:
		return "message"
	}
}

func boundedTool(tool string) string {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return ""
	}
	tool = strings.ToLower(tool)
	var out []rune
	for _, r := range tool {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			out = append(out, r)
		}
		if len(out) >= 64 {
			break
		}
	}
	if len(out) == 0 {
		return "other"
	}
	return string(out)
}

func toolArgsValue(source string, obj map[string]any) any {
	if source == "opencode" {
		if state, ok := firstJSONMapByKey(obj, "state")["input"]; ok {
			return state
		}
	}
	return firstPresentValue(obj, "input", "arguments", "tool_input")
}

func outputBytes(obj map[string]any) int {
	total := 0
	for _, key := range []string{"output", "content", "result", "tool_output"} {
		if value, ok := obj[key]; ok {
			total += jsonValueSize(value)
		}
	}
	if state := firstJSONMapByKey(obj, "state"); len(state) > 0 {
		for _, key := range []string{"output", "metadata"} {
			if value, ok := state[key]; ok {
				total += jsonValueSize(value)
			}
		}
	}
	if total == 0 {
		total = jsonValueSize(obj)
	}
	return total
}

func jsonHasError(value any) bool {
	var found bool
	var walk func(any)
	walk = func(item any) {
		if found {
			return
		}
		switch typed := item.(type) {
		case bool:
			return
		case string:
			lower := strings.ToLower(typed)
			if lower == "error" || lower == "failed" || lower == "failure" {
				found = true
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		case map[string]any:
			for key, child := range typed {
				lowerKey := strings.ToLower(key)
				if lowerKey == "is_error" || lowerKey == "error" {
					if boolean, ok := child.(bool); ok && boolean {
						found = true
						return
					}
					if text, ok := child.(string); ok && text != "" {
						found = true
						return
					}
				}
				if lowerKey == "status" {
					if text, ok := child.(string); ok {
						lower := strings.ToLower(text)
						if lower == "error" || lower == "failed" || lower == "failure" {
							found = true
							return
						}
					}
				}
				walk(child)
			}
		}
	}
	walk(value)
	return found
}

func hashIfPresent(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func hashJSONValue(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func jsonValueSize(value any) int {
	switch typed := value.(type) {
	case string:
		return len(typed)
	case []any:
		total := 0
		for _, item := range typed {
			total += jsonValueSize(item)
		}
		return total
	case map[string]any:
		total := 0
		for _, item := range typed {
			total += jsonValueSize(item)
		}
		return total
	default:
		data, _ := json.Marshal(value)
		return len(data)
	}
}

func firstJSONIntByKey(value any, target string) int {
	target = strings.ToLower(target)
	var found int
	var walk func(any)
	walk = func(v any) {
		if found != 0 {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				item := typed[key]
				if strings.ToLower(key) == target {
					found = intFromAny(item)
					if found != 0 {
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func firstJSONMapByKey(value any, target string) map[string]any {
	target = strings.ToLower(target)
	var found map[string]any
	var walk func(any)
	walk = func(v any) {
		if found != nil {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				item := typed[key]
				if strings.ToLower(key) == target {
					if mapped, ok := item.(map[string]any); ok {
						found = mapped
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	if found == nil {
		return map[string]any{}
	}
	return found
}

func firstJSONValueByKey(value any, target string) any {
	target = strings.ToLower(target)
	var found any
	var walk func(any)
	walk = func(v any) {
		if found != nil {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				item := typed[key]
				if strings.ToLower(key) == target {
					found = item
					return
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func firstPresentValue(value map[string]any, keys ...string) any {
	for _, key := range keys {
		if item, ok := value[key]; ok {
			return item
		}
	}
	return nil
}

func firstPresentString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := stringValue(value[key]); text != "" {
			return text
		}
	}
	return ""
}

func directString(value map[string]any, key string) string {
	return stringValue(value[key])
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func mergeSignals(left, right AnalysisSignals) AnalysisSignals {
	merged := AnalysisSignals{
		NormalizedEventCount:    left.NormalizedEventCount + right.NormalizedEventCount,
		ToolCallCount:           left.ToolCallCount + right.ToolCallCount,
		ToolResultCount:         left.ToolResultCount + right.ToolResultCount,
		ArgsHashedRetryLoops:    max(left.ArgsHashedRetryLoops, right.ArgsHashedRetryLoops),
		CacheReadTokens:         left.CacheReadTokens + right.CacheReadTokens,
		CacheCreationTokens:     left.CacheCreationTokens + right.CacheCreationTokens,
		InputTokens:             left.InputTokens + right.InputTokens,
		OutputTokens:            left.OutputTokens + right.OutputTokens,
		CacheInvalidationEvents: left.CacheInvalidationEvents + right.CacheInvalidationEvents,
		ToolOutputBytes:         left.ToolOutputBytes + right.ToolOutputBytes,
		PatchLinesAdded:         left.PatchLinesAdded + right.PatchLinesAdded,
		PatchLinesRemoved:       left.PatchLinesRemoved + right.PatchLinesRemoved,
		SampleWarnings:          mergeStrings(left.SampleWarnings, right.SampleWarnings),
	}
	if merged.InputTokens > 0 {
		miss := max(0, merged.InputTokens-merged.CacheReadTokens)
		merged.CacheMissRatioPct = min(100, int(float64(miss)/float64(merged.InputTokens)*100))
	}
	merged.PatchLinesTouched = merged.PatchLinesAdded + merged.PatchLinesRemoved
	merged.PatchYieldPer1KTokens = patchYieldPer1KTokens(merged.PatchLinesTouched, totalTokenVolume(merged))
	merged.SampleConfidence = minConfidence(left.SampleConfidence, right.SampleConfidence)
	if merged.SampleConfidence == "" {
		merged.SampleConfidence = "unknown"
	}
	return merged
}

func totalTokenVolume(signals AnalysisSignals) int {
	return signals.InputTokens + signals.OutputTokens + signals.CacheReadTokens + signals.CacheCreationTokens
}

func patchYieldPer1KTokens(patchLinesTouched, totalTokens int) int {
	if patchLinesTouched <= 0 || totalTokens <= 0 {
		return 0
	}
	return int(math.Round(float64(patchLinesTouched) * 1000.0 / float64(totalTokens)))
}

func minConfidence(left, right string) string {
	rank := map[string]int{"unknown": 0, "low": 1, "medium": 2, "high": 3}
	if rank[left] <= rank[right] {
		if left == "" {
			return right
		}
		return left
	}
	return right
}

func formatSignalPct(value int) string {
	return fmt.Sprintf("%d%%", value)
}
