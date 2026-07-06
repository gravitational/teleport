// Package sessionsearch provides web API representations of session search
// results returned by the SessionSearchService gRPC API.
package sessionsearch

import (
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	sessionsearchv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// Session severity levels as exposed to the web UI. These are short, lowercase
// tokens rather than the raw proto enum names (e.g. "high" not
// "RISK_LEVEL_HIGH"). An unspecified severity maps to the empty string so it is
// omitted from the JSON response.
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// SeverityString converts a RiskLevel proto enum into its web UI token.
// Unspecified (or unknown) severities return the empty string.
func SeverityString(level summarizerv1.RiskLevel) string {
	switch level {
	case summarizerv1.RiskLevel_RISK_LEVEL_LOW:
		return SeverityLow
	case summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM:
		return SeverityMedium
	case summarizerv1.RiskLevel_RISK_LEVEL_HIGH:
		return SeverityHigh
	case summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL:
		return SeverityCritical
	default:
		return ""
	}
}

// SeverityFromString converts a web UI severity token into its RiskLevel proto
// enum. Empty or unknown tokens return RISK_LEVEL_UNSPECIFIED.
func SeverityFromString(s string) summarizerv1.RiskLevel {
	switch s {
	case SeverityLow:
		return summarizerv1.RiskLevel_RISK_LEVEL_LOW
	case SeverityMedium:
		return summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM
	case SeverityHigh:
		return summarizerv1.RiskLevel_RISK_LEVEL_HIGH
	case SeverityCritical:
		return summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL
	default:
		return summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED
	}
}

// SessionSummary is the web API representation of a single session search result.
type SessionSummary struct {
	SessionID          string              `json:"sessionId"`
	Kind               string              `json:"kind,omitempty"`
	SessionStart       time.Time           `json:"sessionStart"`
	SessionEnd         *time.Time          `json:"sessionEnd,omitempty"`
	Username           string              `json:"username,omitempty"`
	UserTraits         map[string]any      `json:"userTraits,omitempty"`
	UserRoles          []string            `json:"userRoles,omitempty"`
	AccessRequestIDs   []string            `json:"accessRequestIds,omitempty"`
	Participants       []string            `json:"participants,omitempty"`
	ResourceKind       string              `json:"resourceKind,omitempty"`
	ResourceLabels     map[string]string   `json:"resourceLabels,omitempty"`
	ResourceID         string              `json:"resourceId,omitempty"`
	ResourceName       string              `json:"resourceName,omitempty"`
	ResourceProperties *ResourceProperties `json:"resourceProperties,omitempty"`
	Severity           string              `json:"severity,omitempty"`
	HostID             string              `json:"hostId,omitempty"`
}

// ResourceProperties holds session-kind-specific properties.
type ResourceProperties struct {
	SSH        *SSHProperties        `json:"ssh,omitempty"`
	Kubernetes *KubernetesProperties `json:"kubernetes,omitempty"`
	Database   *DatabaseProperties   `json:"database,omitempty"`
}

// SSHProperties are the kind-specific properties for SSH sessions.
type SSHProperties struct {
	ServerHostname string `json:"serverHostname,omitempty"`
	ServerAddr     string `json:"serverAddr,omitempty"`
}

// KubernetesProperties are the kind-specific properties for Kubernetes sessions.
type KubernetesProperties struct {
	PodNamespace string `json:"podNamespace,omitempty"`
	PodName      string `json:"podName,omitempty"`
}

// DatabaseProperties are the kind-specific properties for database sessions.
type DatabaseProperties struct {
	DatabaseName string `json:"databaseName,omitempty"`
}

func asOptionalTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

func makeUserTraits(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

// MakeSessionSummary converts a SessionSummary proto into its web API representation.
func MakeSessionSummary(s *sessionsearchv1.SessionSummary) SessionSummary {
	return SessionSummary{
		SessionID:          s.GetSessionId(),
		Kind:               s.GetKind(),
		SessionStart:       s.GetSessionStart().AsTime(),
		SessionEnd:         asOptionalTime(s.GetSessionEnd()),
		Username:           s.GetUsername(),
		UserTraits:         makeUserTraits(s.GetUserTraits()),
		UserRoles:          s.GetUserRoles(),
		AccessRequestIDs:   s.GetAccessRequestIds(),
		Participants:       s.GetParticipants(),
		ResourceKind:       s.GetResourceKind(),
		ResourceLabels:     s.GetResourceLabels(),
		ResourceID:         s.GetResourceId(),
		ResourceName:       s.GetResourceName(),
		ResourceProperties: makeResourceProperties(s.GetResourceProperties()),
		Severity:           SeverityString(s.GetSeverity()),
		HostID:             s.GetHostId(),
	}
}

func makeResourceProperties(p *sessionsearchv1.ResourceProperties) *ResourceProperties {
	if p == nil {
		return nil
	}
	switch p.WhichType() {
	case sessionsearchv1.ResourceProperties_Ssh_case:
		ssh := p.GetSsh()
		return &ResourceProperties{SSH: &SSHProperties{
			ServerHostname: ssh.GetServerHostname(),
			ServerAddr:     ssh.GetServerAddr(),
		}}
	case sessionsearchv1.ResourceProperties_Kubernetes_case:
		k := p.GetKubernetes()
		return &ResourceProperties{Kubernetes: &KubernetesProperties{
			PodNamespace: k.GetPodNamespace(),
			PodName:      k.GetPodName(),
		}}
	case sessionsearchv1.ResourceProperties_Database_case:
		d := p.GetDatabase()
		return &ResourceProperties{Database: &DatabaseProperties{
			DatabaseName: d.GetDatabaseName(),
		}}
	default:
		return nil
	}
}
