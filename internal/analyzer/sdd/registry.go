package sdd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ChunksProvider is the indirection point that lets the analyzer package
// inject the embedded sdd_detectors*.json chunks without creating an import
// cycle. analyzer.DetectEcosystem assigns analyzer.SDDDetectorChunks here
// during init; in unit tests it may remain nil (LoadRegistry then yields an
// empty registry, which the WP01 TestLoadRegistryEmptyBase test relies on).
//
// Set this variable from exactly one place (the analyzer package wiring).
var ChunksProvider func() [][]byte

// idPattern matches the allowlisted detector ID shape: lowercase ASCII,
// snake_case, must start with a letter, at least three characters total.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,}$`)

// deniedVersionArgs are flags that must never appear in a Marker's
// VersionArgs. They could leak secrets, contact remote services, or alter
// configuration state — none of which is safe for a passive probe.
var deniedVersionArgs = map[string]struct{}{
	"--config":   {},
	"--registry": {},
	"--token":    {},
	"--server":   {},
	"--login":    {},
}

var (
	registryOnce     sync.Once
	registryVerified []SDDDetector
	registryBinaries []string
)

// LoadRegistry returns the validated, memoized registry of detectors. Only
// detectors whose Status is StatusVerified are returned; other statuses
// are accepted by the loader (so research_needed entries can sit in the
// JSON without breaking the build) but are filtered out before evaluation.
//
// Bad data is a build-time bug: any validation failure panics.
func LoadRegistry() []SDDDetector {
	registryOnce.Do(func() {
		all := make([]SDDDetector, 0)
		var chunks [][]byte
		if ChunksProvider != nil {
			chunks = ChunksProvider()
		}
		for _, chunk := range chunks {
			parsed, err := parseDetectors(chunk)
			if err != nil {
				panic(fmt.Errorf("sdd: load registry: %w", err))
			}
			all = append(all, parsed...)
		}
		registryVerified = make([]SDDDetector, 0, len(all))
		seenBinaries := map[string]struct{}{}
		for _, d := range all {
			if d.Status != StatusVerified {
				continue
			}
			registryVerified = append(registryVerified, d)
			for _, m := range d.Markers {
				if m.SourceClass == SourceCLIBinary && m.Binary != "" {
					if _, ok := seenBinaries[m.Binary]; !ok {
						seenBinaries[m.Binary] = struct{}{}
						registryBinaries = append(registryBinaries, m.Binary)
					}
				}
			}
		}
	})
	return registryVerified
}

// LookupBinaries returns every cli_binary marker name across all loaded,
// verified detectors. This is an introspection helper for tooling and
// tests that need to know which binary names the analyzer will probe —
// for example, a sandbox that wants to pre-stage allowlisted binaries.
//
// Note: the runtime allowlist invariant is enforced structurally, not by
// consulting this function. Evaluate() iterates only over markers from
// the loaded registry, so every cli_binary name it sees is by construction
// on the allowlist. This function exists as the canonical exported view
// of that allowlist for callers outside Evaluate's scope.
func LookupBinaries() []string {
	LoadRegistry()
	out := make([]string, len(registryBinaries))
	copy(out, registryBinaries)
	return out
}

// parseDetectors decodes one JSON chunk and validates every entry. It
// returns an error instead of panicking so tests can exercise the
// validation rules cleanly; the production loader wraps it and panics.
func parseDetectors(data []byte) ([]SDDDetector, error) {
	var detectors []SDDDetector
	if err := json.Unmarshal(data, &detectors); err != nil {
		return nil, fmt.Errorf("decode detectors: %w", err)
	}
	for i := range detectors {
		if err := validateDetector(&detectors[i]); err != nil {
			return nil, fmt.Errorf("detector %d (%q): %w", i, detectors[i].ID, err)
		}
	}
	return detectors, nil
}

func validateDetector(d *SDDDetector) error {
	if !idPattern.MatchString(d.ID) {
		return fmt.Errorf("invalid id %q: must match %s", d.ID, idPattern.String())
	}
	switch d.Status {
	case StatusVerified, StatusResearchNeeded, StatusBlocked:
		// ok
	default:
		return fmt.Errorf("invalid status %q", d.Status)
	}
	if d.Status == StatusVerified && len(d.SourceReferences) == 0 {
		return fmt.Errorf("status=verified requires at least one source_reference")
	}
	for i := range d.Markers {
		if err := validateMarker(&d.Markers[i]); err != nil {
			return fmt.Errorf("marker %d: %w", i, err)
		}
	}
	return nil
}

func validateMarker(m *Marker) error {
	switch m.SourceClass {
	case SourceConfigDir, SourceConfigFile, SourcePackageManifest,
		SourceCommandName, SourceSlashCommand, SourceMCPServerName,
		SourceSkillName, SourcePluginManifest,
		SourceCLIBinary, SourceCLIVersionProbe:
		// ok
	default:
		return fmt.Errorf("invalid source_class %q", m.SourceClass)
	}

	switch m.SourceClass {
	case SourceCLIBinary:
		if m.Binary == "" {
			return fmt.Errorf("source_class=cli_binary requires binary")
		}
	case SourceCLIVersionProbe:
		if m.Binary == "" {
			return fmt.Errorf("source_class=cli_version_probe requires binary")
		}
		if len(m.VersionArgs) == 0 {
			m.VersionArgs = []string{"--version"}
		}
		for _, arg := range m.VersionArgs {
			if _, denied := deniedVersionArgs[arg]; denied {
				return fmt.Errorf("version_args contains denied flag %q", arg)
			}
			if strings.Contains(arg, "/") {
				return fmt.Errorf("version_args entry %q must not contain '/'", arg)
			}
		}
	default:
		if m.Pattern == "" {
			return fmt.Errorf("source_class=%s requires pattern", m.SourceClass)
		}
		compiled, err := regexp.Compile(m.Pattern)
		if err != nil {
			return fmt.Errorf("compile pattern %q: %w", m.Pattern, err)
		}
		m.compiled = compiled
	}
	return nil
}
