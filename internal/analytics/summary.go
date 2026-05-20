package analytics

import (
	"bufio"
	"encoding/json"
	"io"
	"sort"
)

type Summary struct {
	SchemaVersion              string         `json:"schema_version"`
	EventCount                 int            `json:"event_count"`
	ScanTypes                  map[string]int `json:"scan_types,omitempty"`
	OperatingSystems           map[string]int `json:"operating_systems,omitempty"`
	Clients                    map[string]int `json:"clients,omitempty"`
	Shells                     map[string]int `json:"shells,omitempty"`
	PackageManagers            map[string]int `json:"package_managers,omitempty"`
	SDDTools                   map[string]int `json:"sdd_tools,omitempty"`
	SDDCooccurrence            map[string]int `json:"sdd_cooccurrence,omitempty"`
	MCPWarningBands            map[string]int `json:"mcp_warning_bands,omitempty"`
	SkillWarningBands          map[string]int `json:"skill_warning_bands,omitempty"`
	MCPConfigured              map[string]int `json:"mcp_configured,omitempty"`
	MCPExecuted                map[string]int `json:"mcp_executed,omitempty"`
	SkillExposed               map[string]int `json:"skill_exposed,omitempty"`
	SkillExecuted              map[string]int `json:"skill_executed,omitempty"`
	RecommendationClasses      map[string]int `json:"recommendation_classes,omitempty"`
	RecommendationTools        map[string]int `json:"recommendation_tools,omitempty"`
	RecommendationReasons      map[string]int `json:"recommendation_reasons,omitempty"`
	Findings                   map[string]int `json:"findings,omitempty"`
	ScoreBucketsBySDDTool      map[string]int `json:"score_buckets_by_sdd_tool,omitempty"`
	WasteBucketsBySDDTool      map[string]int `json:"waste_buckets_by_sdd_tool,omitempty"`
	SuppressedBelowCohortCount int            `json:"suppressed_below_cohort_count,omitempty"`
}

func SummarizeJSONL(r io.Reader, minCohort int) (Summary, error) {
	if minCohort < 1 {
		minCohort = 1
	}
	builder := summaryBuilder{
		ScanTypes:             map[string]int{},
		OperatingSystems:      map[string]int{},
		Clients:               map[string]int{},
		Shells:                map[string]int{},
		PackageManagers:       map[string]int{},
		SDDTools:              map[string]int{},
		SDDCooccurrence:       map[string]int{},
		MCPWarningBands:       map[string]int{},
		SkillWarningBands:     map[string]int{},
		MCPConfigured:         map[string]int{},
		MCPExecuted:           map[string]int{},
		SkillExposed:          map[string]int{},
		SkillExecuted:         map[string]int{},
		RecommendationClasses: map[string]int{},
		RecommendationTools:   map[string]int{},
		RecommendationReasons: map[string]int{},
		Findings:              map[string]int{},
		ScoreBucketsBySDDTool: map[string]int{},
		WasteBucketsBySDDTool: map[string]int{},
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return Summary{}, err
		}
		builder.add(event)
	}
	if err := scanner.Err(); err != nil {
		return Summary{}, err
	}
	return builder.finish(minCohort), nil
}

type summaryBuilder struct {
	EventCount            int
	ScanTypes             map[string]int
	OperatingSystems      map[string]int
	Clients               map[string]int
	Shells                map[string]int
	PackageManagers       map[string]int
	SDDTools              map[string]int
	SDDCooccurrence       map[string]int
	MCPWarningBands       map[string]int
	SkillWarningBands     map[string]int
	MCPConfigured         map[string]int
	MCPExecuted           map[string]int
	SkillExposed          map[string]int
	SkillExecuted         map[string]int
	RecommendationClasses map[string]int
	RecommendationTools   map[string]int
	RecommendationReasons map[string]int
	Findings              map[string]int
	ScoreBucketsBySDDTool map[string]int
	WasteBucketsBySDDTool map[string]int
}

func (b *summaryBuilder) add(event Event) {
	b.EventCount++
	inc(b.ScanTypes, event.ScanType)
	inc(b.OperatingSystems, event.Ecosystem.OperatingSystem)
	inc(b.Clients, event.Ecosystem.Client)
	inc(b.Shells, event.Ecosystem.Shell)
	for _, id := range event.Ecosystem.PackageManagers {
		inc(b.PackageManagers, id)
	}
	var sddIDs []string
	for _, fp := range event.Ecosystem.WorkflowFingerprints {
		if fp.ID == "" {
			continue
		}
		inc(b.SDDTools, fp.ID)
		inc(b.ScoreBucketsBySDDTool, fp.ID+"|"+event.Score)
		inc(b.WasteBucketsBySDDTool, fp.ID+"|"+event.Waste)
		sddIDs = append(sddIDs, fp.ID)
	}
	sort.Strings(sddIDs)
	for i := 0; i < len(sddIDs); i++ {
		for j := i + 1; j < len(sddIDs); j++ {
			inc(b.SDDCooccurrence, sddIDs[i]+"+"+sddIDs[j])
		}
	}
	inc(b.MCPWarningBands, event.Ecosystem.ToolingUtilization.MCP.WarningBand)
	inc(b.SkillWarningBands, event.Ecosystem.ToolingUtilization.Skill.WarningBand)
	for _, id := range event.Ecosystem.ToolingUtilization.MCP.KnownServerIDs {
		inc(b.MCPConfigured, id)
	}
	for _, id := range event.Ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs {
		inc(b.MCPExecuted, id)
	}
	for _, id := range event.Ecosystem.ToolingUtilization.Skill.KnownExposedIDs {
		inc(b.SkillExposed, id)
	}
	for _, id := range event.Ecosystem.ToolingUtilization.Skill.KnownExecutedIDs {
		inc(b.SkillExecuted, id)
	}
	addRecommendation(b, event.Recommendation.Primary)
	addRecommendation(b, event.Recommendation.Secondary)
	for id, severity := range event.Findings {
		inc(b.Findings, id+"|"+severity)
	}
}

func addRecommendation(b *summaryBuilder, slot *RecommendationSlot) {
	if slot == nil {
		return
	}
	inc(b.RecommendationClasses, slot.Class)
	if slot.ToolID != "" {
		inc(b.RecommendationTools, slot.ToolID)
	}
	inc(b.RecommendationReasons, slot.Reason)
}

func (b summaryBuilder) finish(minCohort int) Summary {
	suppressed := 0
	s := Summary{
		SchemaVersion:              SchemaVersion,
		EventCount:                 b.EventCount,
		ScanTypes:                  suppress(b.ScanTypes, minCohort, &suppressed),
		OperatingSystems:           suppress(b.OperatingSystems, minCohort, &suppressed),
		Clients:                    suppress(b.Clients, minCohort, &suppressed),
		Shells:                     suppress(b.Shells, minCohort, &suppressed),
		PackageManagers:            suppress(b.PackageManagers, minCohort, &suppressed),
		SDDTools:                   suppress(b.SDDTools, minCohort, &suppressed),
		SDDCooccurrence:            suppress(b.SDDCooccurrence, minCohort, &suppressed),
		MCPWarningBands:            suppress(b.MCPWarningBands, minCohort, &suppressed),
		SkillWarningBands:          suppress(b.SkillWarningBands, minCohort, &suppressed),
		MCPConfigured:              suppress(b.MCPConfigured, minCohort, &suppressed),
		MCPExecuted:                suppress(b.MCPExecuted, minCohort, &suppressed),
		SkillExposed:               suppress(b.SkillExposed, minCohort, &suppressed),
		SkillExecuted:              suppress(b.SkillExecuted, minCohort, &suppressed),
		RecommendationClasses:      suppress(b.RecommendationClasses, minCohort, &suppressed),
		RecommendationTools:        suppress(b.RecommendationTools, minCohort, &suppressed),
		RecommendationReasons:      suppress(b.RecommendationReasons, minCohort, &suppressed),
		Findings:                   suppress(b.Findings, minCohort, &suppressed),
		ScoreBucketsBySDDTool:      suppress(b.ScoreBucketsBySDDTool, minCohort, &suppressed),
		WasteBucketsBySDDTool:      suppress(b.WasteBucketsBySDDTool, minCohort, &suppressed),
		SuppressedBelowCohortCount: suppressed,
	}
	return s
}

func inc(m map[string]int, key string) {
	if key == "" {
		key = "unknown"
	}
	m[key]++
}

func suppress(in map[string]int, min int, suppressed *int) map[string]int {
	out := map[string]int{}
	for key, count := range in {
		if count < min {
			(*suppressed)++
			continue
		}
		out[key] = count
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
