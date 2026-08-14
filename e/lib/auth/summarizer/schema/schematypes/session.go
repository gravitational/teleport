package schematypes

// SessionAnalysis is the structured output schema for analyzing a single command execution.
type SessionAnalysis struct {
	ShortDescription      string   `json:"short_description" jsonschema:"required" jsonschema_description:"A concise 1-2 sentence summary of the session (max 200 characters). Examples: 'Routine system maintenance including package updates and log rotation', 'Investigation and resolution of database connectivity issues with configuration changes', 'Suspicious reconnaissance activity followed by credential harvesting attempts'"`
	SessionDescription    string   `json:"session_description" jsonschema:"required" jsonschema_description:"Comprehensive summary of session activities in markdown format. Use professional yet accessible tone, highlight primary activities and patterns. No pronouns or usernames - just describe what was done."`
	SuspiciousActivities  []string `json:"suspicious_activities" jsonschema:"required" jsonschema_description:"Document suspicious ACTIONS/MODIFICATIONS performed: Edited /etc/hosts to add malicious domains, Modified system configurations, Downloaded and executed unverified binaries, Disabled security services, Created backdoor users. DO NOT include: observing wrong permissions in ls output, discovering existing misconfigurations"`
	SecurityIncidents     []string `json:"security_incidents" jsonschema:"required" jsonschema_description:"Security incidents from MODIFICATIONS made: Added suspicious entries to /etc/hosts, Successfully modified critical system files, Installed backdoor services, Extracted credentials. DO NOT include: discovering pre-existing issues not created in this session"`
	CompromiseIndicators  bool     `json:"compromise_indicators" jsonschema:"required" jsonschema_description:"Whether session shows indicators of system compromise"`
	NotableCommandIndexes []int    `json:"notable_command_indexes" jsonschema:"required" jsonschema_description:"Zero-based indexes of notable commands. Include high-risk operations, security events, important failures, system modifications, unusual activities. EXCLUDE: routine navigation (ls, pwd, cd), session management (exit, logout, clear), viewing help (man, --help), canceled commands (^C in input). Only include commands that actually did something significant"`

	// Risk assessment
	RiskLevel string `json:"risk_level" jsonschema:"required,enum=none,enum=low,enum=medium,enum=high,enum=critical" jsonschema_description:"Context-aware risk level. Examples: canceled commands=none/low, /etc/hosts with 127.0.0.1 dev.local=low, hijacking google.com=high, known malware domains=critical. Assess actual activity not privilege level"`
	RiskScore int    `json:"risk_score" jsonschema:"required" jsonschema_description:"Numeric risk score 0-100. Ranges: 0-20 benign, 20-40 low, 40-60 medium, 60-80 high, 80-100 critical"`

	// Server-populated; not produced by the LLM.
	TooLarge              bool `jsonschema:"-"`
	CommandAnalysisFailed bool `jsonschema:"-"`
}
