package schematypes

import (
	"time"
)

// This file defines the schema for command analysis results.
// It should be kept in sync with `api/proto/teleport/summarizer/v1/summarizer.proto`

// CommandAnalysis is the structured output schema for analyzing a single command execution.
type CommandAnalysis struct {
	// Core identification
	Command  string `json:"command" jsonschema:"required" jsonschema_description:"The exact command that was executed"`
	Category string `json:"category" jsonschema:"required,enum=file_operation,enum=network,enum=process,enum=system_config,enum=data_access,enum=authentication,enum=other,enum=package_management,enum=container,enum=source_control,enum=scheduling,enum=monitoring,enum=user_management,enum=transfer,enum=development" jsonschema_description:"Primary category of the command operation"`
	Success  bool   `json:"success" jsonschema:"required" jsonschema_description:"True if command completed successfully. False if canceled (^C in input), failed, or interrupted. Note: ^C in output is normal output, not cancellation"`

	// Risk assessment
	RiskLevel string `json:"risk_level" jsonschema:"required,enum=none,enum=low,enum=medium,enum=high,enum=critical" jsonschema_description:"Context-aware risk level. Examples: canceled commands=none/low, /etc/hosts with 127.0.0.1 dev.local=low, hijacking google.com=high, known malware domains=critical. Assess actual activity not privilege level"`
	RiskScore int    `json:"risk_score" jsonschema:"required" jsonschema_description:"Numeric risk score 0-100. Ranges: 0-20 benign, 20-40 low, 40-60 medium, 60-80 high, 80-100 critical"`
	//nolint:misspell // ignore MITRE
	ThreatCategory string `json:"threat_category" jsonschema:"required,enum=reconnaissance,enum=execution,enum=persistence,enum=privilege_escalation,enum=defense_evasion,enum=credential_access,enum=discovery,enum=lateral_movement,enum=collection,enum=exfiltration,enum=impact,enum=none" jsonschema_description:"Primary MITRE ATT&CK tactic. Use 'none' for benign commands"`

	// Timeline display
	TimelineTitle    string `json:"timeline_title" jsonschema:"required" jsonschema_description:"Concise action summary with backticks for technical elements. Focus on WHAT not HOW. Examples: 'Installed \"vim\", 'Listed \"/etc\" directory', 'Cloned \"repo-name\" repository'. Max 10 words, main action only"`
	TimelineSubtitle string `json:"timeline_subtitle" jsonschema:"required" jsonschema_description:"Empty string for successful commands. Only populate for critical failures: 'Command not found', 'Permission denied', 'Cancelled by user', 'Connection timeout'. Max 4 words"`

	// Descriptions
	ShortDescription string `json:"short_description" jsonschema:"required" jsonschema_description:"One-line factual description without pronouns. Example: 'Listed directory contents' not 'User listed directory contents'"`
	Description      string `json:"description" jsonschema:"required" jsonschema_description:"Detailed 1-2 line factual description. State actions performed, not UI observations. For editors, describe saved changes not screen transitions. If canceled (^C in input): 'Command canceled and did not execute'"`

	// Security indicators
	ErrorMessages      []string `json:"error_messages" jsonschema:"required" jsonschema_description:"Error messages from command execution. Include actual error text from stderr or failure output"`
	SuspiciousFlags    []string `json:"suspicious_flags" jsonschema:"required" jsonschema_description:"Security-relevant actions performed. Examples: modifying /etc/hosts with suspicious domains, editing cron jobs, disabling security features. Exclude pre-existing conditions or normal operations"`
	SensitiveItems     []string `json:"sensitive_items" jsonschema:"required" jsonschema_description:"Sensitive files or data that were READ, WRITTEN, or MODIFIED. Examples: /etc/shadow, SSH keys, .aws/credentials. Directory listings don't count"`
	SuspiciousPatterns []string `json:"suspicious_patterns" jsonschema:"required" jsonschema_description:"Patterns indicating malicious activity. Examples: DNS hijacking entries, backdoor user creation, security bypass attempts. Focus on intent and impact"`
	IOCs               []string `json:"iocs" jsonschema:"required" jsonschema_description:"Concrete indicators of compromise. Include: suspicious IPs, domains, URLs, file hashes, or signatures found in command actions"`
	//nolint:misspell // ignore MITRE
	MitreAttackIDs []string `json:"mitre_attack_ids" jsonschema:"required" jsonschema_description:"Specific MITRE ATT&CK technique IDs (e.g., T1059, T1055). Only technique IDs, not tactic names"`

	// Security flags
	HasSensitiveData    bool `json:"has_sensitive_data" jsonschema:"required" jsonschema_description:"True if sensitive data was accessed (read/write/modify). Listing directories containing sensitive files doesn't count"`
	PrivilegeEscalation bool `json:"privilege_escalation" jsonschema:"required" jsonschema_description:"True if privilege escalation was attempted or achieved (sudo to root, setuid, capability manipulation)"`
	DataExfiltration    bool `json:"data_exfiltration" jsonschema:"required" jsonschema_description:"True if data was transferred externally (scp, curl POST, base64 encoding for transfer)"`
	Persistence         bool `json:"persistence" jsonschema:"required" jsonschema_description:"True if persistence mechanisms were created (cron jobs, systemd services, shell profiles, SSH keys)"`

	// Server-populated; not produced by the LLM.
	StartOffset           time.Duration `jsonschema:"-"`
	EndOffset             time.Duration `jsonschema:"-"`
	InferenceErrorMessage string        `jsonschema:"-"`
}
