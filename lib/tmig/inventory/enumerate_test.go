package inventory

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

func TestBuildSummary(t *testing.T) {
	agents := []Agent{
		{UUID: "1", Hostname: "h1", Services: []string{"ssh_service"}, Version: "17.3.1"},
		{UUID: "2", Hostname: "h2", Services: []string{"ssh_service", "db_service"}, Version: "17.3.1"},
		{UUID: "3", Hostname: "h3", Services: []string{"kubernetes_service"}, Version: "17.2.0"},
	}
	recon := map[string]classify.ReconResult{
		"1": {Reachable: true, InstallKind: classify.InstallSystemd, JoinMethod: "token"},
		"2": {Reachable: true, InstallKind: classify.InstallSystemd, JoinMethod: "iam"},
		"3": {Reachable: false, Err: "connection refused"},
	}
	summary := BuildSummary(agents, recon)
	require.Equal(t, 3, summary.Total)
	require.Equal(t, 2, summary.Reachable)
	require.Equal(t, 1, summary.Unreachable)
	require.Equal(t, 2, summary.ByInstallKind[classify.InstallSystemd])
	require.Equal(t, 1, summary.ByJoinMethod["token"])
	require.Equal(t, 1, summary.ByJoinMethod["iam"])
	require.Equal(t, 2, summary.ByVersion["17.3.1"])
	require.Equal(t, 1, summary.ByVersion["17.2.0"])
}

func TestBuildSummaryEmpty(t *testing.T) {
	summary := BuildSummary(nil, nil)
	require.Equal(t, 0, summary.Total)
}
