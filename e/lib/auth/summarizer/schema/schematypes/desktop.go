package schematypes

// DesktopScreenshotAnalysis is the structured output schema for analyzing a batch of desktop screenshots from a chunk
// of a session. Per-chunk synthesis fields are intentionally absent: only the events flow downstream, and the final
// session-level summary is produced by a separate synthesis pass over all events.
type DesktopScreenshotAnalysis struct {
	NotableSessionEvents []DesktopSessionEvent `json:"notable_session_events" jsonschema:"required" jsonschema_description:"List of significant desktop session events observed across the screenshot batch"`
}

// DesktopSessionEvent is the structured output schema for a notable event observed
// during a desktop session.
type DesktopSessionEvent struct {
	// Core classification
	Category string `json:"category" jsonschema:"required,enum=file_operation,enum=network,enum=process,enum=system_config,enum=data_access,enum=authentication,enum=other" jsonschema_description:"Primary category of the desktop activity. file_operation: file explorer, save/open dialogs. network: web browsing, network settings. process: launching/closing applications. system_config: settings panels, control panel. data_access: viewing documents, database clients. authentication: login screens, credential prompts. other: anything not fitting above"`

	// Time bounds
	StartTime string `json:"start_time" jsonschema:"required" jsonschema_description:"The time that the first screenshot was taken in the event"`
	EndTime   string `json:"end_time" jsonschema:"required" jsonschema_description:"The time that the last screenshot was taken in the event"`

	// Risk assessment
	RiskLevel string `json:"risk_level" jsonschema:"required,enum=none,enum=low,enum=medium,enum=high,enum=critical" jsonschema_description:"Context-aware risk level based on the event. Examples: opening a public website=none, accessing personal email=low, viewing confidential documents=high, displaying credentials=critical. Assess actual visible content not user intent"`
	RiskScore int    `json:"risk_score" jsonschema:"required" jsonschema_description:"Numeric risk score 0-100. Ranges: 0-20 benign, 20-40 low, 40-60 medium, 60-80 high, 80-100 critical"`
	//nolint:misspell // ignore MITRE
	ThreatCategory string `json:"threat_category" jsonschema:"required,enum=reconnaissance,enum=execution,enum=persistence,enum=privilege_escalation,enum=defense_evasion,enum=credential_access,enum=discovery,enum=lateral_movement,enum=collection,enum=exfiltration,enum=impact,enum=none" jsonschema_description:"Primary MITRE ATT&CK tactic. Use 'none' for benign activity"`

	// Timeline display
	TimelineTitle    string `json:"timeline_title" jsonschema:"required" jsonschema_description:"Concise action summary. Focus on WHAT not HOW. Examples: 'Opened \"finance-report.xlsx\"', 'Browsed \"github.com\"', 'Launched Excel'. Max 10 words, main action only"`
	TimelineSubtitle string `json:"timeline_subtitle" jsonschema:"required" jsonschema_description:"Empty string for routine activity. Only populate for notable failures or anomalies: 'Authentication failed', 'Permission denied', 'Connection error'. Max 4 words"`

	// Descriptions
	ShortDescription    string `json:"short_description" jsonschema:"required" jsonschema_description:"One-line factual description without pronouns. Example: 'Opened the finance report in Excel' not 'User opened the finance report in Excel'"`
	DetailedDescription string `json:"detailed_description" jsonschema:"required" jsonschema_description:"Detailed 1-2 line factual description. State actions observed, including the application, document or destination involved when visible. Avoid mere UI observations"`

	// Security indicators
	SuspiciousFlags    []string `json:"suspicious_flags" jsonschema:"required" jsonschema_description:"Security-relevant actions observed on screen. Examples: opening a credentials manager, accessing internal admin tools, viewing sensitive system files. Exclude routine application use"`
	SensitiveItems     []string `json:"sensitive_items" jsonschema:"required" jsonschema_description:"Sensitive files, documents, or data visible on screen. Examples: passwords, API keys, /etc/shadow, customer PII"`
	SuspiciousPatterns []string `json:"suspicious_patterns" jsonschema:"required" jsonschema_description:"Patterns indicating malicious activity. Examples: rapid credential entry attempts, browsing known malicious domains, copying sensitive data to external destinations"`
	IOCs               []string `json:"iocs" jsonschema:"required" jsonschema_description:"Concrete indicators of compromise visible on screen: suspicious URLs, IP addresses, domain names, file hashes, or signatures"`
	//nolint:misspell // ignore MITRE
	MitreAttackIDs []string `json:"mitre_attack_ids" jsonschema:"required" jsonschema_description:"Specific MITRE ATT&CK technique IDs (e.g., T1059, T1055). Only technique IDs, not tactic names"`

	// Security flags
	HasSensitiveData    bool `json:"has_sensitive_data" jsonschema:"required" jsonschema_description:"True if sensitive data was visible or accessed during the event"`
	PrivilegeEscalation bool `json:"privilege_escalation" jsonschema:"required" jsonschema_description:"True if the event shows privilege escalation (e.g., UAC prompt accepted, sudo password entered, switching to an admin account)"`
	DataExfiltration    bool `json:"data_exfiltration" jsonschema:"required" jsonschema_description:"True if data was transferred externally (e.g., uploading files, copying to webmail, posting to external sites)"`
	Persistence         bool `json:"persistence" jsonschema:"required" jsonschema_description:"True if persistence mechanisms were established (e.g., scheduling tasks, modifying startup programs, installing remote-access tools)"`

	// Desktop event details
	Applications      []string `json:"applications" jsonschema:"required" jsonschema_description:"Applications visible or interacted with during this event. Examples: 'Microsoft Excel', 'Google Chrome', 'PuTTY'"`
	VisibleURLs       []string `json:"visible_urls" jsonschema:"required" jsonschema_description:"URLs visible on screen, e.g. browser address bars, links in emails, or terminal output"`
	VisibleFilePaths  []string `json:"visible_file_paths" jsonschema:"required" jsonschema_description:"File paths visible on screen, e.g. in file explorer, save dialogs, terminals, or document titles"`
	ActiveWindowTitle string   `json:"active_window_title" jsonschema:"required" jsonschema_description:"The title of the active/focused window during this event"`

	// Server-populated; not produced by the LLM.
	InferenceErrorMessage string `jsonschema:"-"`
}

// DesktopSessionAnalysis is the structured output schema for the final synthesis of a desktop session, combining
// per-screenshot analyses into a session-level security review.
type DesktopSessionAnalysis struct {
	ShortDescription     string   `json:"short_description" jsonschema:"required" jsonschema_description:"A concise 1-2 sentence summary of the desktop session (max 200 characters). Examples: 'Routine work in spreadsheet and email applications', 'Investigation across browser, terminal, and database client', 'Suspicious credential access followed by data downloads'"`
	SessionDescription   string   `json:"session_description" jsonschema:"required" jsonschema_description:"Comprehensive summary of session activities in markdown format. Use professional yet accessible tone, highlight primary applications, documents, and patterns observed. No pronouns or usernames - just describe what was done."`
	SuspiciousActivities []string `json:"suspicious_activities" jsonschema:"required" jsonschema_description:"Document suspicious ACTIONS observed on screen: opened credentials manager and copied entries, navigated to known malicious domains, copied files to webmail, accepted UAC prompts to install unverified software. DO NOT include: merely seeing app icons, viewing public websites, normal application use"`
	SecurityIncidents    []string `json:"security_incidents" jsonschema:"required" jsonschema_description:"Security incidents from MODIFICATIONS or ACTIONS performed during the session: exfiltrated documents to personal email, installed remote-access software, disabled antivirus, escalated privileges. DO NOT include: discovering pre-existing issues not caused in this session"`
	CompromiseIndicators bool     `json:"compromise_indicators" jsonschema:"required" jsonschema_description:"Whether the session shows indicators of system compromise"`

	// Risk assessment
	RiskLevel string `json:"risk_level" jsonschema:"required,enum=none,enum=low,enum=medium,enum=high,enum=critical" jsonschema_description:"Context-aware risk level based on observed activity. Examples: routine work=none/low, viewing sensitive documents=medium, exfiltrating data=high, hands-on credential theft=critical. Assess actual activity not user role"`
	RiskScore int    `json:"risk_score" jsonschema:"required" jsonschema_description:"Numeric risk score 0-100. Ranges: 0-20 benign, 20-40 low, 40-60 medium, 60-80 high, 80-100 critical"`

	// Server-populated; not produced by the LLM.
	TooLarge                 bool `jsonschema:"-"`
	ScreenshotAnalysisFailed bool `jsonschema:"-"`
}
