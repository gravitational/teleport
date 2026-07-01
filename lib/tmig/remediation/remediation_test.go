package remediation

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/report"
)

func TestEmitScopedTokenIAM(t *testing.T) {
	params := TokenParams{
		Scope:      "/dgxc/team-a",
		JoinMethod: "iam",
		Roles:      []string{"Node"},
		AWS:        &AWSParams{Account: "883900000000"},
		TokenName:  "team-a-iam",
	}
	rem := EmitScopedToken(params)
	require.Equal(t, report.RemScopedTokenIaC, rem.Kind)
	require.Contains(t, rem.YAML, "join_method: iam")
	require.Contains(t, rem.YAML, "883900000000")
	require.Contains(t, rem.YAML, "scope: /dgxc/team-a")
	require.Contains(t, rem.Terraform, "teleport_scoped_token")
	require.Contains(t, rem.Note, "tmig never creates long-lived tokens")
}

func TestEmitScopedTokenBoundKeypair(t *testing.T) {
	params := TokenParams{
		Scope:         "/dgxc/team-a",
		JoinMethod:    "bound_keypair",
		Roles:         []string{"Node"},
		TokenName:     "team-a-builders",
		RecoveryLimit: 3,
	}
	rem := EmitScopedToken(params)
	require.Contains(t, rem.YAML, "bound_keypair")
	require.Contains(t, rem.YAML, "limit: 3")
	require.Contains(t, rem.Note, "recovery.limit=3")
}

func TestEmitManualCommands(t *testing.T) {
	params := ManualParams{
		Hostname:        "win-build-07",
		InputConfig:     `C:\teleport\teleport.yaml`,
		OutputConfig:    `C:\teleport\teleport_dgxc-team-a.yaml`,
		Proxy:           "scoped.example.com:443",
		TokenName:       "<scoped-token-name>",
		JoinMethod:      "token",
		DataDir:         `C:\teleport\data_dgxc-team-a`,
		DisableServices: []string{"windows_desktop"},
		InstallSuffix:   "dgxc-team-a",
	}
	rem := EmitManualCommands(params)
	require.Equal(t, report.RemManualCommands, rem.Kind)
	require.NotEmpty(t, rem.Commands)
	require.Contains(t, rem.Commands[0], "reconfigure")
	require.Contains(t, rem.Commands[0], "scoped.example.com:443")
}

func TestGroupRemediations(t *testing.T) {
	hosts := []HostRemediation{
		{Hostname: "h1", Key: "iam-883900000000-Node-/dgxc/team-a", Params: TokenParams{Scope: "/dgxc/team-a", JoinMethod: "iam", Roles: []string{"Node"}, AWS: &AWSParams{Account: "883900000000"}, TokenName: "team-a-iam"}},
		{Hostname: "h2", Key: "iam-883900000000-Node-/dgxc/team-a", Params: TokenParams{Scope: "/dgxc/team-a", JoinMethod: "iam", Roles: []string{"Node"}, AWS: &AWSParams{Account: "883900000000"}, TokenName: "team-a-iam"}},
		{Hostname: "h3", Key: "iam-774100000000-Node-/dgxc/team-a", Params: TokenParams{Scope: "/dgxc/team-a", JoinMethod: "iam", Roles: []string{"Node"}, AWS: &AWSParams{Account: "774100000000"}, TokenName: "team-a-iam-secondary"}},
	}
	grouped := Group(hosts)
	require.Len(t, grouped, 2)
	// First group covers h1 and h2
	require.Len(t, grouped[0].HostsCovered, 2)
	require.Len(t, grouped[1].HostsCovered, 1)
}
