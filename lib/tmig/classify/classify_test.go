package classify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var defaultScopableMethods = []string{
	"token", "iam", "ec2", "gcp", "azure", "azure_devops", "oracle", "kubernetes", "bound_keypair",
}

func autoHost() ClassifyInput {
	return ClassifyInput{
		Recon: ReconResult{
			Reachable:         true,
			OS:                "Linux",
			HasSystemd:        true,
			HasTeleportUpdate: true,
			ConfigPath:        "/etc/teleport.yaml",
			RootPath:          true,
			JoinMethod:        "token",
			Services:          []string{"ssh_service"},
			InstallKind:       InstallSystemd,
		},
		ScopedTarget:    true,
		ScopableMethods: defaultScopableMethods,
	}
}

func TestClassifyAuto(t *testing.T) {
	out := Classify(autoHost())
	require.Equal(t, VerdictAuto, out.Verdict)
	require.Equal(t, StatusSatisfied, out.Status)
	require.Equal(t, AttentionNone, out.Attention)
	require.Equal(t, MarkerServerEnforced, out.MarkerTrust)
}

func TestClassifyManualNonLinux(t *testing.T) {
	input := autoHost()
	input.Recon.OS = "Windows"
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "non-Linux")
}

func TestClassifyManualNoSystemd(t *testing.T) {
	input := autoHost()
	input.Recon.HasSystemd = false
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "no systemd")
}

func TestClassifyManualNoTeleportUpdate(t *testing.T) {
	input := autoHost()
	input.Recon.HasTeleportUpdate = false
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "no teleport-update")
}

func TestClassifyManualNonStandardConfig(t *testing.T) {
	input := autoHost()
	input.Recon.ConfigPath = "/opt/teleport/config.yaml"
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "config not at /etc/teleport.yaml")
}

func TestClassifyManualNoRoot(t *testing.T) {
	input := autoHost()
	input.Recon.RootPath = false
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "no root")
}

func TestClassifyManualUnscopableJoinMethod(t *testing.T) {
	input := autoHost()
	input.Recon.JoinMethod = "github"
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "join method not scopable")
}

func TestClassifyManualUnscopableJoinMethodUnscopedTargetIsOK(t *testing.T) {
	input := autoHost()
	input.Recon.JoinMethod = "github"
	input.ScopedTarget = false
	out := Classify(input)
	// Unscoped target: method need not be scopable, so this host is AUTO
	require.Equal(t, VerdictAuto, out.Verdict)
}

func TestClassifyManualContainerInstall(t *testing.T) {
	input := autoHost()
	input.Recon.InstallKind = InstallContainer
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "container")
}

func TestClassifyPipeline(t *testing.T) {
	input := autoHost()
	input.Recon.InstallKind = InstallSystemd
	input.DiscoveryEnrolled = true
	out := Classify(input)
	require.Equal(t, VerdictPipeline, out.Verdict)
	require.Equal(t, AttentionPipeline, out.Attention)
}

func TestClassifyPrereqBlocked(t *testing.T) {
	input := autoHost()
	input.Recon.JoinMethod = "iam"
	input.CoveringTokenFound = false
	out := Classify(input)
	require.Equal(t, VerdictPrereq, out.Verdict)
	require.Equal(t, StatusBlocked, out.Status)
	require.Equal(t, AttentionIaCOnetime, out.Attention)
}

func TestClassifyPrereqSatisfied(t *testing.T) {
	input := autoHost()
	input.Recon.JoinMethod = "iam"
	input.CoveringTokenFound = true
	out := Classify(input)
	require.Equal(t, VerdictPrereq, out.Verdict)
	require.Equal(t, StatusSatisfied, out.Status)
	require.Equal(t, AttentionNone, out.Attention)
}

func TestClassifyOrphan(t *testing.T) {
	input := autoHost()
	input.Orphan = true
	out := Classify(input)
	require.Equal(t, VerdictManual, out.Verdict)
	require.Contains(t, out.Reason, "orphan")
}

func TestClassifyServiceStrip(t *testing.T) {
	input := autoHost()
	input.Recon.Services = []string{"ssh_service", "db_service"}
	input.ScopableRoles = map[string]bool{"Node": true, "Kube": true, "App": false, "Db": false}
	out := Classify(input)
	require.Equal(t, VerdictAuto, out.Verdict)
	require.Equal(t, StatusPartial, out.Status)
	require.Contains(t, out.StrippedServices, "db_service")
}

func TestClassifyMarkerTrustStaticForNonSSH(t *testing.T) {
	input := autoHost()
	input.Recon.Services = []string{"kubernetes_service"}
	out := Classify(input)
	require.Equal(t, VerdictAuto, out.Verdict)
	require.Equal(t, MarkerStaticLabel, out.MarkerTrust)
}

func TestClassifyMarkerTrustStaticForUnscopedTarget(t *testing.T) {
	input := autoHost()
	input.ScopedTarget = false
	out := Classify(input)
	require.Equal(t, MarkerStaticLabel, out.MarkerTrust)
}
