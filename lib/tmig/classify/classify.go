// Package classify implements a pure, table-driven verdict engine that
// classifies hosts into AUTO/PREREQ/PIPELINE/MANUAL based on gathered facts.
// It performs no I/O.
package classify

import "strings"

// Verdict for a host.
type Verdict string

const (
	VerdictAuto     Verdict = "AUTO"
	VerdictPrereq   Verdict = "PREREQ"
	VerdictPipeline Verdict = "PIPELINE"
	VerdictManual   Verdict = "MANUAL"
)

// Status of a host within its verdict.
type Status string

const (
	StatusSatisfied Status = "SATISFIED"
	StatusBlocked   Status = "BLOCKED"
	StatusPartial   Status = "PARTIAL"
	StatusPending   Status = "PENDING"
	StatusError     Status = "ERROR"
)

// AttentionClass determines how much human effort a host needs.
type AttentionClass string

const (
	AttentionNone       AttentionClass = "NONE"
	AttentionIaCOnetime AttentionClass = "IAC_ONETIME"
	AttentionPipeline   AttentionClass = "PIPELINE"
	AttentionManual     AttentionClass = "MANUAL_PERHOST"
)

// MarkerTrust indicates how the migration marker is secured.
type MarkerTrust string

const (
	MarkerServerEnforced MarkerTrust = "SERVER_ENFORCED"
	MarkerStaticLabel    MarkerTrust = "STATIC_LABEL"
)

// InstallKind describes how the agent was deployed.
type InstallKind string

const (
	InstallSystemd    InstallKind = "systemd"
	InstallKubernetes InstallKind = "kubernetes"
	InstallSupervisor InstallKind = "supervisor"
	InstallContainer  InstallKind = "container"
	InstallOther      InstallKind = "other"
)

// ReconResult is the output of the SSH probe. Defined here for the classifier's
// consumption; the recon package will produce these.
type ReconResult struct {
	HostUUID          string
	Reachable         bool
	Err               string
	OS                string
	HasSystemd        bool
	HasTeleportUpdate bool
	ConfigPath        string
	ConfigReadable    bool
	JoinMethod        string
	Services          []string
	RootPath          bool
	ListenAddrs       []string
	BinaryVersion     string
	InstallKind       InstallKind
}

// ClassifyInput contains everything the classifier needs for one host.
type ClassifyInput struct {
	Recon              ReconResult
	Orphan             bool
	ScopedTarget       bool
	ScopableMethods    []string
	ScopableRoles      map[string]bool // e.g. {"Node": true, "Kube": true, "App": false, "Db": false}
	DiscoveryEnrolled  bool
	CoveringTokenFound bool
}

// ClassifyOutput is the classifier's decision for one host.
type ClassifyOutput struct {
	Verdict          Verdict
	Status           Status
	Attention        AttentionClass
	Reason           string
	MarkerTrust      MarkerTrust
	StrippedServices []string
}

// serviceToRole maps teleport service names to their role category.
// windows_desktop_service maps to "WindowsDesktop" which is never present
// in ScopableRoles, ensuring it is always stripped regardless.
var serviceToRole = map[string]string{
	"ssh_service":             "Node",
	"kubernetes_service":      "Kube",
	"app_service":             "App",
	"db_service":              "Db",
	"windows_desktop_service": "WindowsDesktop",
}

// Classify assigns exactly one verdict to a host. Pure, no I/O.
func Classify(input ClassifyInput) ClassifyOutput {
	// Orphans are always MANUAL.
	if input.Orphan {
		return ClassifyOutput{
			Verdict:     VerdictManual,
			Status:      StatusPending,
			Attention:   AttentionManual,
			Reason:      "matches no mapping selector — reported as orphan, never migrated",
			MarkerTrust: MarkerStaticLabel,
		}
	}

	// Unreachable hosts are pending.
	if !input.Recon.Reachable {
		return ClassifyOutput{
			Verdict:     VerdictManual,
			Status:      StatusPending,
			Attention:   AttentionManual,
			Reason:      "host unreachable: " + input.Recon.Err,
			MarkerTrust: MarkerStaticLabel,
		}
	}

	// MANUAL signals (first match wins).
	if reason := manualReason(input); reason != "" {
		return ClassifyOutput{
			Verdict:     VerdictManual,
			Status:      StatusBlocked,
			Attention:   AttentionManual,
			Reason:      reason,
			MarkerTrust: MarkerStaticLabel,
		}
	}

	// PIPELINE: discovery-enrolled hosts.
	if input.DiscoveryEnrolled {
		return ClassifyOutput{
			Verdict:     VerdictPipeline,
			Status:      StatusSatisfied,
			Attention:   AttentionPipeline,
			Reason:      "discovery-enrolled; migrate via pipeline reconfig, not per-host SSH",
			MarkerTrust: MarkerStaticLabel,
		}
	}

	// Determine stripped services.
	stripped := strippedServices(input.Recon.Services, input.ScopableRoles)
	status := StatusSatisfied
	if len(stripped) > 0 {
		status = StatusPartial
	}

	// PREREQ: delegated join methods.
	if isDelegatedMethod(input.Recon.JoinMethod) {
		if !input.CoveringTokenFound {
			return ClassifyOutput{
				Verdict:          VerdictPrereq,
				Status:           StatusBlocked,
				Attention:        AttentionIaCOnetime,
				Reason:           "joins via " + input.Recon.JoinMethod + "; no covering scoped token found",
				MarkerTrust:      MarkerStaticLabel,
				StrippedServices: stripped,
			}
		}
		return ClassifyOutput{
			Verdict:          VerdictPrereq,
			Status:           status,
			Attention:        AttentionNone,
			Reason:           "joins via " + input.Recon.JoinMethod + "; covering scoped token found",
			MarkerTrust:      MarkerStaticLabel,
			StrippedServices: stripped,
		}
	}

	// AUTO: token-joined, all gates passed.
	trust := markerTrust(input)
	reason := buildAutoReason(input)
	if len(stripped) > 0 {
		reason += "; " + strings.Join(stripped, ", ") + " stripped (no scoped story)"
	}

	return ClassifyOutput{
		Verdict:          VerdictAuto,
		Status:           status,
		Attention:        AttentionNone,
		Reason:           reason,
		MarkerTrust:      trust,
		StrippedServices: stripped,
	}
}

func manualReason(input ClassifyInput) string {
	r := input.Recon
	if !strings.EqualFold(r.OS, "Linux") && r.OS != "" {
		return "non-Linux OS: " + r.OS
	}
	if !r.HasSystemd {
		return "no systemd"
	}
	if !r.HasTeleportUpdate {
		return "no teleport-update"
	}
	if r.ConfigPath != "/etc/teleport.yaml" && r.ConfigPath != "" {
		return "config not at /etc/teleport.yaml (found: " + r.ConfigPath + ")"
	}
	if !r.RootPath {
		return "no root/sudo path"
	}
	if input.ScopedTarget && !isMethodScopable(r.JoinMethod, input.ScopableMethods) {
		return "join method not scopable: " + r.JoinMethod
	}
	if r.InstallKind == InstallContainer || r.InstallKind == InstallSupervisor {
		return string(r.InstallKind) + " install (not automatable)"
	}
	return ""
}

func isMethodScopable(method string, scopable []string) bool {
	for _, m := range scopable {
		if m == method {
			return true
		}
	}
	return false
}

func isDelegatedMethod(method string) bool {
	delegated := map[string]bool{
		"iam": true, "ec2": true, "gcp": true, "azure": true,
		"azure_devops": true, "oracle": true, "kubernetes": true, "bound_keypair": true,
	}
	return delegated[method]
}

func strippedServices(services []string, scopableRoles map[string]bool) []string {
	if len(scopableRoles) == 0 {
		return nil
	}
	var stripped []string
	for _, svc := range services {
		role, ok := serviceToRole[svc]
		if !ok {
			continue
		}
		if supported, exists := scopableRoles[role]; exists && !supported {
			stripped = append(stripped, svc)
		} else if !exists && svc == "windows_desktop_service" {
			// windows_desktop_service role "WindowsDesktop" is never in
			// ScopableRoles, so always strip it when ScopableRoles is provided.
			stripped = append(stripped, svc)
		}
	}
	return stripped
}

func markerTrust(input ClassifyInput) MarkerTrust {
	if !input.ScopedTarget {
		return MarkerStaticLabel
	}
	for _, svc := range input.Recon.Services {
		if svc == "ssh_service" {
			return MarkerServerEnforced
		}
	}
	return MarkerStaticLabel
}

func buildAutoReason(_ ClassifyInput) string {
	return "token join, systemd, teleport-update present, config at /etc/teleport.yaml, root"
}
