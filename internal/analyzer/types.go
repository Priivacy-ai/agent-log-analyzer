package analyzer

type Report struct {
	JobID           string             `json:"job_id"`
	Version         string             `json:"version"`
	Score           int                `json:"score"`
	EstimatedWaste  WasteRange         `json:"estimated_waste_pct"`
	Metrics         Metrics            `json:"metrics"`
	Findings        []Finding          `json:"findings"`
	Ecosystem       Ecosystem          `json:"ecosystem"`
	Redactions      map[string]int     `json:"redactions"`
	SecurityReceipt SecurityReceipt    `json:"security_receipt"`
	Timeline        []TimelinePoint    `json:"timeline"`
	ImmediateFixes  []string           `json:"immediate_fixes"`
	AggregateEvent  AggregateSafeEvent `json:"aggregate_event"`
}

type WasteRange struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

type Metrics struct {
	Turns               int `json:"turns"`
	EstimatedTokens     int `json:"estimated_tokens"`
	ToolOutputTokens    int `json:"tool_output_tokens"`
	Rereads             int `json:"rereads"`
	RetryDepthMax       int `json:"retry_depth_max"`
	ContextGrowthEvents int `json:"context_growth_events"`
	FailedCommands      int `json:"failed_commands"`
	SessionCount        int `json:"session_count,omitempty"`
}

type Finding struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Severity       string          `json:"severity"`
	CostImpact     string          `json:"cost_impact"`
	Evidence       FindingEvidence `json:"evidence"`
	Recommendation string          `json:"recommendation"`
	Deterministic  bool            `json:"deterministic"`
}

type FindingEvidence struct {
	Count       int      `json:"count,omitempty"`
	TokenShare  int      `json:"token_share_pct,omitempty"`
	TopFiles    []string `json:"top_files,omitempty"`
	Description string   `json:"description,omitempty"`
}

type Ecosystem struct {
	Client                string   `json:"client"`
	CodingAgents          []string `json:"coding_agents"`
	OperatingSystem       string   `json:"operating_system"`
	Shell                 string   `json:"shell"`
	WorkflowFrameworks    []string `json:"workflow_frameworks"`
	MCPServersKnown       []string `json:"mcp_servers_known"`
	UnknownMCPServerCount int      `json:"unknown_mcp_server_count"`
	KnownSkills           []string `json:"known_skills"`
	UnknownSkillCount     int      `json:"unknown_skill_count"`
	KnownPlugins          []string `json:"known_plugins"`
	UnknownPluginCount    int      `json:"unknown_plugin_count"`
	PackageManagers       []string `json:"package_managers"`
	VersionControl        string   `json:"version_control"`
}

type SecurityReceipt struct {
	RawTranscriptSentToLLM bool   `json:"raw_transcript_sent_to_llm"`
	OutboundDuringAnalysis bool   `json:"outbound_during_analysis"`
	SecretsRedacted        int    `json:"secrets_redacted"`
	RawLogTTL              string `json:"raw_log_ttl"`
}

type TimelinePoint struct {
	Turn            int `json:"turn"`
	EstimatedTokens int `json:"estimated_tokens"`
	ToolTokens      int `json:"tool_tokens"`
	Rereads         int `json:"rereads"`
	Retries         int `json:"retries"`
}

type AggregateSafeEvent struct {
	Event           string            `json:"event"`
	ParserType      string            `json:"parser_type"`
	InputSizeBucket string            `json:"input_size_bucket"`
	TurnBucket      string            `json:"turn_bucket"`
	ScoreBucket     string            `json:"score_bucket"`
	WasteBucket     string            `json:"waste_bucket"`
	Findings        map[string]string `json:"findings"`
	Redactions      map[string]int    `json:"redactions"`
	Ecosystem       Ecosystem         `json:"ecosystem"`
}
