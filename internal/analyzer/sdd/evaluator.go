package sdd

import (
	"context"
	"sort"
	"strings"
	"time"
)

// perProbeTimeout bounds each individual cli_version_probe invocation so a
// detector with multiple version probes cannot exhaust the parent context
// and starve later probes (NFR-002 — see RISK-1 mission-review finding).
const perProbeTimeout = 2 * time.Second

// Fingerprint mirrors analyzer.EcosystemFingerprint field-for-field. Defined
// locally in this package so the evaluator can return a typed result without
// importing the parent analyzer package (which would create an import cycle
// because analyzer/ecosystem.go imports sdd). The wiring in
// analyzer.DetectEcosystem converts []sdd.Fingerprint → []EcosystemFingerprint.
//
// Field semantics are documented in data-model.md ("Report-emitted record");
// see also research.md §R-05 for confidence-rule rationale.
type Fingerprint struct {
	ID            string
	Confidence    string
	Sources       []string
	EvidenceCount int
	Active        bool
	Installed     bool
	VersionBucket string
}

// runtimeTouchSources are the source classes that prove a tool is being
// invoked by the user/agent at runtime — not just present on disk. Active
// status requires confidence=high AND at least one of these.
var runtimeTouchSources = map[SourceClass]struct{}{
	SourceSlashCommand:  {},
	SourceMCPServerName: {},
	SourceCLIBinary:     {},
}

// Evaluate scores every verified detector in registry against text + slashHits
// using probe for CLI presence/version queries. The caller (analyzer package)
// is responsible for filtering registry to verified detectors; LoadRegistry()
// already does this. The returned slice is sorted by
// (competitor_priority asc, ID asc).
//
// slashHits is the slice of slash tokens the caller has already extracted
// from parsed transcript lines (see analyzer.extractSlashTokens). Slash-
// command markers are matched against both the raw text AND the joined
// slashHits list — slashHits catches cases where the upstream parser has
// normalized or split a slash invocation into a token that no longer
// appears verbatim in `text`. See research.md §R-10 (call site).
func Evaluate(
	ctx context.Context,
	text string,
	slashHits []string,
	probe CLIProbe,
	registry []SDDDetector,
) []Fingerprint {
	out := make([]Fingerprint, 0, len(registry))
	priorities := make(map[string]int, len(registry))

	// Pre-join slashHits with newlines so regex anchors (\b, ^, $ in multi-
	// line mode) behave reasonably. The empty case yields "" — which
	// MatchString tolerates and never matches positive slash patterns.
	slashCorpus := strings.Join(slashHits, "\n")

	for _, d := range registry {
		priorities[d.ID] = d.CompetitorPriority

		// First pass: determine which cli_binary markers matched. We need
		// this before scanning cli_version_probe markers, since a version
		// probe only fires when its same-binary cli_binary peer matched.
		cliBinaryHit := map[string]bool{}
		for _, m := range d.Markers {
			if m.SourceClass != SourceCLIBinary {
				continue
			}
			if probe.LookPath(m.Binary) {
				cliBinaryHit[m.Binary] = true
			}
		}

		matched := map[SourceClass]int{}
		var versionBucket string
		var negativeHit bool

		for i := range d.Markers {
			m := &d.Markers[i]
			hit := false

			switch m.SourceClass {
			case SourceCLIBinary:
				hit = cliBinaryHit[m.Binary]
			case SourceCLIVersionProbe:
				if !cliBinaryHit[m.Binary] {
					continue
				}
				probeCtx, cancel := context.WithTimeout(ctx, perProbeTimeout)
				raw, ok := probe.Version(probeCtx, m.Binary, m.VersionArgs)
				cancel()
				if !ok {
					continue
				}
				bucket := NormalizeVersionBucket(raw)
				if bucket == "" {
					continue
				}
				versionBucket = bucket
				hit = true
			default:
				re := m.Compiled()
				if re == nil {
					continue
				}
				if re.MatchString(text) {
					hit = true
				} else if m.SourceClass == SourceSlashCommand && slashCorpus != "" && re.MatchString(slashCorpus) {
					// Fallback to caller-extracted slash tokens for cases
					// where the raw transcript text has been normalized.
					hit = true
				}
			}

			if !hit {
				continue
			}
			if m.Negative {
				negativeHit = true
				// Don't break — we still scan the rest in case validation
				// would surface issues, but the detector is vetoed.
			} else {
				matched[m.SourceClass]++
			}
		}

		if negativeHit {
			continue
		}
		if len(matched) == 0 {
			continue
		}

		tier := scoreConfidence(matched, d.ConfidenceRules)

		// Build sorted, deduplicated sources slice. Map keys are already
		// distinct, so we just collect and sort.
		sources := make([]string, 0, len(matched))
		evidenceCount := 0
		for cls, n := range matched {
			sources = append(sources, string(cls))
			evidenceCount += n
		}
		sort.Strings(sources)

		active := tier == ConfidenceHigh && hasRuntimeTouch(matched)

		out = append(out, Fingerprint{
			ID:            d.ID,
			Confidence:    string(tier),
			Sources:       sources,
			EvidenceCount: evidenceCount,
			Active:        active,
			Installed:     anyCLIBinary(cliBinaryHit),
			VersionBucket: versionBucket,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := priorities[out[i].ID], priorities[out[j].ID]
		if pi != pj {
			return pi < pj
		}
		return out[i].ID < out[j].ID
	})

	return out
}

// scoreConfidence walks ConfidenceRules and returns the highest tier whose
// requirements are satisfied by matched. If no rule matches but evidence
// exists, defaults to ConfidenceLow per research.md §R-05.
func scoreConfidence(matched map[SourceClass]int, rules []ConfidenceRule) Confidence {
	tierRank := map[Confidence]int{
		ConfidenceHigh:   3,
		ConfidenceMedium: 2,
		ConfidenceLow:    1,
	}
	best := Confidence("")
	bestRank := 0
	for _, r := range rules {
		if !ruleSatisfied(r, matched) {
			continue
		}
		if rank := tierRank[r.Confidence]; rank > bestRank {
			bestRank = rank
			best = r.Confidence
		}
	}
	if best == "" {
		return ConfidenceLow
	}
	return best
}

func ruleSatisfied(r ConfidenceRule, matched map[SourceClass]int) bool {
	if r.RequiresDistinctClasses > 0 {
		if len(matched) < r.RequiresDistinctClasses {
			return false
		}
	}
	for _, cls := range r.RequiresAllOf {
		if matched[cls] == 0 {
			return false
		}
	}
	if len(r.RequiresAnyOf) > 0 {
		anyOK := false
		for _, cls := range r.RequiresAnyOf {
			if matched[cls] > 0 {
				anyOK = true
				break
			}
		}
		if !anyOK {
			return false
		}
	}
	return true
}

func hasRuntimeTouch(matched map[SourceClass]int) bool {
	for cls := range runtimeTouchSources {
		if matched[cls] > 0 {
			return true
		}
	}
	return false
}

func anyCLIBinary(hits map[string]bool) bool {
	for _, v := range hits {
		if v {
			return true
		}
	}
	return false
}
