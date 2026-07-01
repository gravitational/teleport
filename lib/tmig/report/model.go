// Package report defines the readiness report data model and a pure builder
// that computes the full report from classified host inputs.
package report

import (
	"time"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

// Report is the complete readiness report.
type Report struct {
	RunID         string           `json:"run_id"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Source        ClusterIdentity  `json:"source"`
	Target        ClusterIdentity  `json:"target"`
	ScopesEnabled bool             `json:"scopes_enabled"`
	Scopable      Capability       `json:"scopable"`
	Summary       Summary          `json:"summary"`
	Mappings      []MappingSummary `json:"mappings"`
	Hosts         []HostReport     `json:"hosts"`
	Remediations  []Remediation    `json:"remediations"`
	Warnings      []string         `json:"warnings"`
}

// ClusterIdentity identifies a cluster endpoint.
type ClusterIdentity struct {
	Name          string `json:"name"`
	ClusterID     string `json:"cluster_id"`
	Proxy         string `json:"proxy"`
	CAFingerprint string `json:"ca_fingerprint"`
	User          string `json:"user"`
	Version       string `json:"version"`
	ScopePinned   bool   `json:"scope_pinned"`
}

// Capability describes what the cluster supports for scoped migration.
type Capability struct {
	Roles        map[string]bool `json:"roles"`
	Methods      []string        `json:"methods"`
	TableVersion string          `json:"table_version"`
	Drift        []DriftEntry    `json:"drift"`
}

// DriftEntry records a mismatch between expected and actual capability.
type DriftEntry struct {
	Item     string `json:"item"`
	Expected bool   `json:"expected"`
	Actual   bool   `json:"actual"`
}

// Summary provides aggregate counts across all hosts.
type Summary struct {
	Total         int                      `json:"total"`
	ByVerdict     map[classify.Verdict]int `json:"by_verdict"`
	Orphans       int                      `json:"orphans"`
	Blocked       int                      `json:"blocked"`
	ReadyToEnroll int                      `json:"ready_to_enroll"`
	Attention     AttentionRollup          `json:"attention"`
}

// AttentionRollup provides a breakdown of how many hosts need what kind of attention.
type AttentionRollup struct {
	AutomaticHosts       int `json:"automatic_hosts"`
	IaCActions           int `json:"iac_actions"`
	IaCHostsCovered      int `json:"iac_hosts_covered"`
	PipelineActions      int `json:"pipeline_actions"`
	PipelineHostsCovered int `json:"pipeline_hosts_covered"`
	ManualHosts          int `json:"manual_hosts"`
}

// MappingSummary describes a single scope mapping and how many hosts it matched.
type MappingSummary struct {
	Scope         string            `json:"scope"`
	Selector      map[string]string `json:"selector"`
	MatchedHosts  int               `json:"matched_hosts"`
	InstallSuffix string            `json:"install_suffix"`
}

// HostReport is the per-host section of the report.
type HostReport struct {
	HostUUID         string                  `json:"host_uuid"`
	Hostname         string                  `json:"hostname"`
	Mapping          string                  `json:"mapping"`
	Verdict          classify.Verdict        `json:"verdict"`
	Status           classify.Status         `json:"status"`
	Attention        classify.AttentionClass `json:"attention"`
	Reason           string                  `json:"reason"`
	JoinMethod       string                  `json:"join_method"`
	Services         []string                `json:"services"`
	StrippedServices []string                `json:"stripped_services"`
	MarkerTrust      classify.MarkerTrust    `json:"marker_trust"`
	ConfigDiff       string                  `json:"config_diff"`
	RewriteMode      string                  `json:"rewrite_mode"`
	RemediationRef   string                  `json:"remediation_ref"`
}

// Remediation describes a single remediation action that covers one or more hosts.
type Remediation struct {
	ID           string          `json:"id"`
	Kind         RemediationKind `json:"kind"`
	Title        string          `json:"title"`
	HostsCovered []string        `json:"hosts_covered"`
	Terraform    string          `json:"terraform,omitempty"`
	YAML         string          `json:"yaml,omitempty"`
	TCTL         string          `json:"tctl,omitempty"`
	Commands     []string        `json:"commands,omitempty"`
	Note         string          `json:"note,omitempty"`
}

// RemediationKind categorizes remediation actions.
type RemediationKind string

const (
	RemScopedTokenIaC   RemediationKind = "SCOPED_TOKEN_IAC"
	RemPipelineReconfig RemediationKind = "PIPELINE_RECONFIG"
	RemManualCommands   RemediationKind = "MANUAL_HOST_COMMANDS"
)
