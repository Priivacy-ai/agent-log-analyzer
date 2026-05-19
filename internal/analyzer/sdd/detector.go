// Package sdd defines the typed registry of spec-driven-development (SDD)
// tool detectors, plus the loader and validation logic that runs at process
// startup. Records live in this package; the report-emitted
// EcosystemFingerprint lives in the parent analyzer package to avoid an
// import cycle.
package sdd

import "regexp"

// SourceClass enumerates the kinds of evidence a Marker can match. The
// loader rejects any value outside this set with a panic at startup, since
// the registry is embedded and bad data is a build-time bug.
type SourceClass string

const (
	SourceConfigDir       SourceClass = "config_dir"
	SourceConfigFile      SourceClass = "config_file"
	SourcePackageManifest SourceClass = "package_manifest"
	SourceCommandName     SourceClass = "command_name"
	SourceSlashCommand    SourceClass = "slash_command"
	SourceMCPServerName   SourceClass = "mcp_server_name"
	SourceSkillName       SourceClass = "skill_name"
	SourcePluginManifest  SourceClass = "plugin_manifest"
	SourceCLIBinary       SourceClass = "cli_binary"
	SourceCLIVersionProbe SourceClass = "cli_version_probe"
)

// Confidence is the bucket a fingerprint may carry once evaluated.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Status records the registry-curation state of a detector. Only detectors
// whose Status is StatusVerified participate in evaluation.
type Status string

const (
	StatusVerified       Status = "verified"
	StatusResearchNeeded Status = "research_needed"
	StatusBlocked        Status = "blocked"
)

// SourceRef cites where a detector's claim about a tool was validated.
// Required (non-empty list) for detectors whose Status is StatusVerified.
type SourceRef struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
	Note string `json:"note,omitempty"`
}

// Marker is one piece of evidence the evaluator looks for. Pattern is used
// for every SourceClass except cli_binary and cli_version_probe, which use
// Binary (and optionally VersionArgs).
type Marker struct {
	SourceClass SourceClass `json:"source_class"`
	Pattern     string      `json:"pattern,omitempty"`
	Binary      string      `json:"binary,omitempty"`
	VersionArgs []string    `json:"version_args,omitempty"`
	Negative    bool        `json:"negative,omitempty"`
	Note        string      `json:"note,omitempty"`

	// compiled is populated by the loader for non-CLI markers; never
	// serialized.
	compiled *regexp.Regexp
}

// Compiled returns the regex compiled by the loader for this marker, or nil
// if the marker has no pattern (cli_binary / cli_version_probe markers).
func (m *Marker) Compiled() *regexp.Regexp {
	return m.compiled
}

// ConfidenceRule expresses one of the conditions that, if satisfied, awards
// the named Confidence level to the detector.
type ConfidenceRule struct {
	Confidence              Confidence    `json:"confidence"`
	RequiresAnyOf           []SourceClass `json:"requires_any_of,omitempty"`
	RequiresAllOf           []SourceClass `json:"requires_all_of,omitempty"`
	RequiresDistinctClasses int           `json:"requires_distinct_classes,omitempty"`
}

// SDDDetector is one registry entry. Each detector maps a public,
// allowlisted ID to a bundle of markers and confidence rules.
type SDDDetector struct {
	ID                 string           `json:"id"`
	DisplayName        string           `json:"display_name"`
	Aliases            []string         `json:"aliases,omitempty"`
	Category           string           `json:"category"`
	CompetitorPriority int              `json:"competitor_priority"`
	Status             Status           `json:"status"`
	SourceReferences   []SourceRef      `json:"source_references"`
	Markers            []Marker         `json:"markers"`
	ConfidenceRules    []ConfidenceRule `json:"confidence_rules"`
}
