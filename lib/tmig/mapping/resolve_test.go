// lib/tmig/mapping/resolve_test.go
package mapping

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/config"
)

func TestResolveExactMatch(t *testing.T) {
	mappings := []config.Mapping{
		{Selector: map[string]string{"resource_group": "team-a"}, Scope: "/dgxc/team-a", InstallSuffix: "dgxc-team-a"},
	}
	host := HostLabels{"resource_group": "team-a", "env": "prod"}
	result := Resolve(host, mappings)
	require.False(t, result.Orphan)
	require.Nil(t, result.Conflict)
	require.Equal(t, &mappings[0], result.Matched)
}

func TestResolveOrphan(t *testing.T) {
	mappings := []config.Mapping{
		{Selector: map[string]string{"resource_group": "team-a"}, Scope: "/dgxc/team-a", InstallSuffix: "dgxc-team-a"},
	}
	host := HostLabels{"resource_group": "team-b", "env": "prod"}
	result := Resolve(host, mappings)
	require.True(t, result.Orphan)
	require.Nil(t, result.Matched)
	require.Nil(t, result.Conflict)
}

func TestResolveConflict(t *testing.T) {
	mappings := []config.Mapping{
		{Selector: map[string]string{"env": "prod"}, Scope: "/dgxc/team-a", InstallSuffix: "a"},
		{Selector: map[string]string{"region": "us-east"}, Scope: "/dgxc/team-b", InstallSuffix: "b"},
	}
	host := HostLabels{"env": "prod", "region": "us-east"}
	result := Resolve(host, mappings)
	require.False(t, result.Orphan)
	require.Nil(t, result.Matched)
	require.Len(t, result.Conflict, 2)
}

func TestResolveMultiLabelSelector(t *testing.T) {
	mappings := []config.Mapping{
		{Selector: map[string]string{"env": "prod", "team": "alpha"}, Scope: "/s", InstallSuffix: "s"},
	}
	// Missing "team" label — should not match
	host := HostLabels{"env": "prod"}
	result := Resolve(host, mappings)
	require.True(t, result.Orphan)
}

func TestResolveEmptySelector(t *testing.T) {
	// Empty selector means "whole cluster" — matches everything
	mappings := []config.Mapping{
		{Selector: map[string]string{}, Scope: "/all", InstallSuffix: "all"},
	}
	host := HostLabels{"anything": "here"}
	result := Resolve(host, mappings)
	// Empty selector should not match per spec (selector is required),
	// but if it somehow gets through, it matches all.
	// Actually config validation rejects empty selectors,
	// so this tests the behavior if validation is bypassed.
	require.False(t, result.Orphan)
	require.Equal(t, &mappings[0], result.Matched)
}
