// Package remediation emits copy-pasteable Terraform/YAML for blocked PREREQ
// hosts, pipeline reconfig steps, and per-host manual commands.
package remediation

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/tmig/report"
)

// TokenParams describes parameters for a scoped token remediation.
type TokenParams struct {
	Scope         string
	JoinMethod    string
	Roles         []string
	TokenName     string
	AWS           *AWSParams
	GCP           *GCPParams
	RecoveryLimit int
}

// AWSParams holds AWS-specific join parameters.
type AWSParams struct {
	Account string
}

// GCPParams holds GCP-specific join parameters.
type GCPParams struct {
	ServiceAccount string
}

// PipelineParams describes parameters for a pipeline reconfig remediation.
type PipelineParams struct {
	Scope         string
	Matchers      string
	InstallSuffix string
	HostCount     int
}

// ManualParams describes parameters for per-host manual commands.
type ManualParams struct {
	Hostname        string
	InputConfig     string
	OutputConfig    string
	Proxy           string
	TokenName       string
	JoinMethod      string
	DataDir         string
	DisableServices []string
	InstallSuffix   string
}

// EmitScopedToken produces a Remediation with YAML and Terraform for a scoped token.
func EmitScopedToken(params TokenParams) report.Remediation {
	yaml := buildTokenYAML(params)
	tf := buildTokenTerraform(params)
	title := fmt.Sprintf("Create scoped %s token covering %s (role %s) in %s",
		params.JoinMethod, describeAllow(params), strings.Join(params.Roles, ","), params.Scope)
	note := "Your pipeline owns this token. tmig never creates long-lived tokens; apply via Terraform/operator."
	if params.RecoveryLimit > 0 {
		note = fmt.Sprintf("Suffixed install needs its OWN keypair (not shared). recovery.limit=%d surfaced so retries don't exhaust it.", params.RecoveryLimit)
	}
	return report.Remediation{
		ID:        fmt.Sprintf("rem-%s-%s", params.JoinMethod, sanitizeID(describeAllow(params))),
		Kind:      report.RemScopedTokenIaC,
		Title:     title,
		YAML:      yaml,
		Terraform: tf,
		Note:      note,
	}
}

// EmitPipeline produces a Remediation with pipeline reconfig commands.
func EmitPipeline(params PipelineParams) report.Remediation {
	commands := []string{
		fmt.Sprintf("# Stand up a scope-pinned Discovery Service joined to TARGET in %s,", params.Scope),
		fmt.Sprintf("# reusing the same matchers, with --install-suffix %s.", params.InstallSuffix),
		"# Future instances then land in the scope automatically.",
	}
	return report.Remediation{
		ID:       fmt.Sprintf("rem-pipeline-%s", sanitizeID(params.Scope)),
		Kind:     report.RemPipelineReconfig,
		Title:    fmt.Sprintf("Migrate the Discovery pipeline into %s (covers %d hosts)", params.Scope, params.HostCount),
		Commands: commands,
		Note:     fmt.Sprintf("One pipeline migration covers all %d hosts. tmig does not SSH these hosts individually.", params.HostCount),
	}
}

// EmitManualCommands produces a Remediation with per-host reconfiguration commands.
func EmitManualCommands(params ManualParams) report.Remediation {
	disableSvc := ""
	if len(params.DisableServices) > 0 {
		disableSvc = " --disable-service " + strings.Join(params.DisableServices, ",")
	}
	cmd := fmt.Sprintf("teleport reconfigure --input %s --output %s --proxy %s --token %s --join-method %s --data-dir %s%s",
		params.InputConfig, params.OutputConfig, params.Proxy, params.TokenName, params.JoinMethod, params.DataDir, disableSvc)
	startCmd := fmt.Sprintf("teleport-update --install-suffix %s enable && systemctl start teleport_%s.service",
		params.InstallSuffix, params.InstallSuffix)
	return report.Remediation{
		ID:           fmt.Sprintf("rem-manual-%s", sanitizeID(params.Hostname)),
		Kind:         report.RemManualCommands,
		Title:        fmt.Sprintf("Manual: %s", params.Hostname),
		HostsCovered: []string{params.Hostname},
		Commands:     []string{cmd, startCmd},
	}
}

func buildTokenYAML(params TokenParams) string {
	var sb strings.Builder
	sb.WriteString("kind: scoped_token\n")
	sb.WriteString(fmt.Sprintf("metadata: { name: %s }\n", params.TokenName))
	sb.WriteString(fmt.Sprintf("scope: %s\n", params.Scope))
	sb.WriteString("spec:\n")
	sb.WriteString(fmt.Sprintf("  assigned_scope: %s\n", params.Scope))
	sb.WriteString(fmt.Sprintf("  join_method: %s\n", params.JoinMethod))
	sb.WriteString(fmt.Sprintf("  roles: [%s]\n", strings.Join(params.Roles, ", ")))
	if params.AWS != nil {
		sb.WriteString("  aws:\n    allow:\n")
		sb.WriteString(fmt.Sprintf("      - aws_account: \"%s\"\n", params.AWS.Account))
	}
	if params.JoinMethod == "bound_keypair" {
		sb.WriteString("  bound_keypair:\n    onboarding:\n")
		sb.WriteString("      registration_secret: \"<one-time-secret-from-your-pipeline>\"\n")
		if params.RecoveryLimit > 0 {
			sb.WriteString("    recovery:\n      mode: standard\n")
			sb.WriteString(fmt.Sprintf("      limit: %d\n", params.RecoveryLimit))
		}
	}
	return sb.String()
}

func buildTokenTerraform(params TokenParams) string {
	if params.JoinMethod == "bound_keypair" {
		return "" // Terraform for bound_keypair is complex; emit YAML only
	}
	name := strings.ReplaceAll(params.TokenName, "-", "_")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("resource \"teleport_scoped_token\" \"%s\" {\n", name))
	sb.WriteString(fmt.Sprintf("  metadata = { name = \"%s\" }\n", params.TokenName))
	sb.WriteString(fmt.Sprintf("  scope    = \"%s\"\n", params.Scope))
	sb.WriteString("  spec = {\n")
	sb.WriteString(fmt.Sprintf("    assigned_scope = \"%s\"\n", params.Scope))
	sb.WriteString(fmt.Sprintf("    join_method    = \"%s\"\n", params.JoinMethod))
	sb.WriteString(fmt.Sprintf("    roles          = [%s]\n", quotedList(params.Roles)))
	if params.AWS != nil {
		sb.WriteString(fmt.Sprintf("    aws = { allow = [{ aws_account = \"%s\" }] }\n", params.AWS.Account))
	}
	sb.WriteString("  }\n}\n")
	return sb.String()
}

func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}

func describeAllow(params TokenParams) string {
	if params.AWS != nil {
		return "AWS " + params.AWS.Account
	}
	if params.GCP != nil {
		return "GCP " + params.GCP.ServiceAccount
	}
	return params.TokenName
}

func sanitizeID(s string) string {
	r := strings.NewReplacer("/", "-", " ", "-", ".", "-")
	return strings.Trim(r.Replace(strings.ToLower(s)), "-")
}
