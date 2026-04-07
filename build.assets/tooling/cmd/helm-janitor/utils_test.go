package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectCharts(t *testing.T) {
	tests := []struct {
		name       string
		chartNames string
		rootDir    string
		expected   []Chart
		expectErr  require.ErrorAssertionFunc
	}{
		{
			name:       "no chart name should select all charts",
			chartNames: "",
			rootDir:    "",
			expected:   chartsWithPath("."),
			expectErr:  require.NoError,
		},
		{
			name:       "single chart name",
			chartNames: "teleport-kube-agent",
			rootDir:    "",
			expected: []Chart{
				{
					Name:          "teleport-kube-agent",
					Path:          "examples/chart/teleport-kube-agent",
					ReferencePath: "docs/pages/includes/helm-reference/zz_generated.teleport-kube-agent.mdx",
				},
			},
			expectErr: require.NoError,
		},
		{
			name:       "multiple chart name",
			chartNames: "teleport-kube-agent,teleport-relay",
			rootDir:    "",
			expected: []Chart{
				{
					Name:          "teleport-kube-agent",
					Path:          "examples/chart/teleport-kube-agent",
					ReferencePath: "docs/pages/includes/helm-reference/zz_generated.teleport-kube-agent.mdx",
				},
				{
					Name:          "teleport-relay",
					Path:          "examples/chart/teleport-relay",
					ReferencePath: "docs/pages/includes/helm-reference/zz_generated.teleport-relay.mdx",
				},
			},
			expectErr: require.NoError,
		},
		{
			name:       "single chart name with root dir",
			chartNames: "teleport-kube-agent",
			rootDir:    "/tmp/teleport",
			expected: []Chart{
				{
					Name:          "teleport-kube-agent",
					Path:          "/tmp/teleport/examples/chart/teleport-kube-agent",
					ReferencePath: "/tmp/teleport/docs/pages/includes/helm-reference/zz_generated.teleport-kube-agent.mdx",
				},
			},
			expectErr: require.NoError,
		},
		{
			name:       "single library chart",
			chartNames: "teleport-kube-updater",
			rootDir:    "",
			expected: []Chart{
				{
					Name:      "teleport-kube-updater",
					Path:      "examples/chart/teleport-kube-updater",
					IsLibrary: true,
				},
			},
			expectErr: require.NoError,
		},
		{
			name:       "unknown chart name",
			chartNames: "unknown-chart",
			rootDir:    "",
			expected:   nil,
			expectErr:  require.Error,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := selectCharts(test.chartNames, test.rootDir)
			test.expectErr(t, err)
			require.Equal(t, test.expected, result)
		})
	}

}
