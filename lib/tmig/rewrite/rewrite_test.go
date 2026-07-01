package rewrite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/classify"
	"github.com/gravitational/teleport/lib/tmig/config"
)

func TestStubRewriterPreview(t *testing.T) {
	stub := NewStubRewriter()
	host := classify.ReconResult{
		ConfigPath: "/etc/teleport.yaml",
		JoinMethod: "token",
		Services:   []string{"ssh_service"},
	}
	mapping := config.Mapping{
		Scope:         "/dgxc/team-a",
		InstallSuffix: "dgxc-team-a",
	}
	result, err := stub.Preview(context.Background(), host, mapping, "scoped.example.com:443")
	require.NoError(t, err)
	require.NotEmpty(t, result.Diff)
	require.Equal(t, "stub", result.Mode)
	require.True(t, result.Valid)
	// Must not contain secrets
	require.NotContains(t, result.Diff, "secret")
}

func TestStubRewriterDiffContainsExpectedEdits(t *testing.T) {
	stub := NewStubRewriter()
	host := classify.ReconResult{
		ConfigPath: "/etc/teleport.yaml",
		JoinMethod: "token",
		Services:   []string{"ssh_service", "auth_service"},
	}
	mapping := config.Mapping{Scope: "/dgxc/team-a", InstallSuffix: "dgxc-team-a"}
	result, _ := stub.Preview(context.Background(), host, mapping, "scoped.example.com:443")
	// Should show proxy change
	require.Contains(t, result.Diff, "scoped.example.com:443")
	// Should show data_dir change
	require.Contains(t, result.Diff, "dgxc-team-a")
}
