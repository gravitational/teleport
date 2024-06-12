/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestMatchResourceLabels tests matching a resource against a selector.
func TestMatchResourceLabels(t *testing.T) {
	tests := []struct {
		description    string
		selectors      []ResourceMatcher
		databaseLabels map[string]string
		match          bool
	}{
		{
			description: "wildcard selector matches empty labels",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{types.Wildcard: []string{types.Wildcard}}},
			},
			databaseLabels: nil,
			match:          true,
		},
		{
			description: "wildcard selector matches any label",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{types.Wildcard: []string{types.Wildcard}}},
			},
			databaseLabels: map[string]string{
				uuid.New().String(): uuid.New().String(),
				uuid.New().String(): uuid.New().String(),
			},
			match: true,
		},
		{
			description: "selector doesn't match empty labels",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{"env": []string{"dev"}}},
			},
			databaseLabels: nil,
			match:          false,
		},
		{
			description: "selector matches specific label",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{"env": []string{"dev"}}},
			},
			databaseLabels: map[string]string{"env": "dev"},
			match:          true,
		},
		{
			description: "selector doesn't match label",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{"env": []string{"dev"}}},
			},
			databaseLabels: map[string]string{"env": "prod"},
			match:          false,
		},
		{
			description: "selector matches label",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{"env": []string{"dev", "prod"}}},
			},
			databaseLabels: map[string]string{"env": "prod"},
			match:          true,
		},
		{
			description: "selector doesn't match multiple labels",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{
					"env":     []string{"dev"},
					"cluster": []string{"root"},
				}},
			},
			databaseLabels: map[string]string{"cluster": "root"},
			match:          false,
		},
		{
			description: "selector matches multiple labels",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{
					"env":     []string{"dev"},
					"cluster": []string{"root"},
				}},
			},
			databaseLabels: map[string]string{"cluster": "root", "env": "dev"},
			match:          true,
		},
		{
			description: "one of multiple selectors matches",
			selectors: []ResourceMatcher{
				{Labels: types.Labels{"env": []string{"dev"}}},
				{Labels: types.Labels{"cluster": []string{"root"}}},
			},
			databaseLabels: map[string]string{"cluster": "root"},
			match:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name:   "test",
				Labels: test.databaseLabels,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			})
			require.NoError(t, err)

			require.Equal(t, test.match, MatchResourceLabels(test.selectors, database.GetAllLabels()))
		})
	}
}

func TestMatchResourceByFilters_Helper(t *testing.T) {
	t.Parallel()

	server, err := types.NewServerWithLabels("banana", types.KindNode, types.ServerSpecV2{
		Hostname:    "foo",
		Addr:        "bar",
		PublicAddrs: []string{"foo.example.com:3080"},
	}, map[string]string{"env": "prod", "os": "mac"})
	require.NoError(t, err)

	resource := types.ResourceWithLabels(server)

	testcases := []struct {
		name                string
		predicateExpression string
		filters             MatchResourceFilter
		assertErr           require.ErrorAssertionFunc
		assertMatch         require.BoolAssertionFunc
	}{
		{
			name:        "empty filters",
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name:                "all match",
			predicateExpression: `resource.spec.hostname == "foo"`,
			filters: MatchResourceFilter{
				SearchKeywords: []string{"banana"},
				Labels:         map[string]string{"os": "mac"},
			},
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name:                "no match",
			predicateExpression: `labels.env == "no-match"`,
			filters: MatchResourceFilter{
				SearchKeywords: []string{"no", "match"},
				Labels:         map[string]string{"no": "match"},
			},
			assertErr:   require.NoError,
			assertMatch: require.False,
		},
		{
			name: "search keywords hostname match",
			filters: MatchResourceFilter{
				SearchKeywords: []string{"foo"},
			},
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name: "search keywords addr match",
			filters: MatchResourceFilter{
				SearchKeywords: []string{"bar"},
			},
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name: "search keywords public addr match",
			filters: MatchResourceFilter{
				SearchKeywords: []string{"foo.example.com"},
			},
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name:                "expression match",
			predicateExpression: `labels.env == "prod" && exists(labels.os)`,
			assertErr:           require.NoError,
			assertMatch:         require.True,
		},
		{
			name:                "no expression match",
			predicateExpression: `labels.env == "no-match"`,
			assertErr:           require.NoError,
			assertMatch:         require.False,
		},
		{
			name:                "error in expr",
			predicateExpression: `labels.env == prod`,
			assertErr:           require.Error,
			assertMatch:         require.False,
		},
		{
			name: "label match",
			filters: MatchResourceFilter{
				Labels: map[string]string{"os": "mac"},
			},
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name: "no label match",
			filters: MatchResourceFilter{
				Labels: map[string]string{"no": "match"},
			},
			assertErr:   require.NoError,
			assertMatch: require.False,
		},
		{
			name: "search match",
			filters: MatchResourceFilter{
				SearchKeywords: []string{"mac", "env"},
			},
			assertErr:   require.NoError,
			assertMatch: require.True,
		},
		{
			name: "no search match",
			filters: MatchResourceFilter{
				SearchKeywords: []string{"no", "match"},
			},
			assertErr:   require.NoError,
			assertMatch: require.False,
		},
		{
			name:                "partial match is no match: search",
			predicateExpression: `resource.spec.hostname == "foo"`,
			filters: MatchResourceFilter{
				Labels:         map[string]string{"os": "mac"},
				SearchKeywords: []string{"no", "match"},
			},
			assertErr:   require.NoError,
			assertMatch: require.False,
		},
		{
			name:                "partial match is no match: labels",
			predicateExpression: `resource.spec.hostname == "foo"`,
			filters: MatchResourceFilter{
				Labels:         map[string]string{"no": "match"},
				SearchKeywords: []string{"mac", "env"},
			},
			assertErr:   require.NoError,
			assertMatch: require.False,
		},
		{
			name:                "partial match is no match: expression",
			predicateExpression: `labels.env == "no-match"`,
			filters: MatchResourceFilter{
				Labels:         map[string]string{"os": "mac"},
				SearchKeywords: []string{"mac", "env"},
			},
			assertErr:   require.NoError,
			assertMatch: require.False,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.predicateExpression != "" {
				parser, err := NewResourceExpression(tc.predicateExpression)
				require.NoError(t, err)
				tc.filters.PredicateExpression = parser
			}

			match, err := matchResourceByFilters(resource, tc.filters)
			tc.assertErr(t, err)
			tc.assertMatch(t, match)
		})
	}
}

func TestMatchAndFilterKubeClusters(t *testing.T) {
	t.Parallel()

	getKubeServers := func() []types.KubeServer {
		cluster1, err := types.NewKubernetesClusterV3(
			types.Metadata{
				Name:   "cluster-1",
				Labels: map[string]string{"env": "prod", "os": "mac"},
			},
			types.KubernetesClusterSpecV3{},
		)
		require.NoError(t, err)

		cluster2, err := types.NewKubernetesClusterV3(
			types.Metadata{
				Name:   "cluster-2",
				Labels: map[string]string{"env": "staging", "os": "mac"},
			},
			types.KubernetesClusterSpecV3{},
		)
		require.NoError(t, err)
		cluster3, err := types.NewKubernetesClusterV3(
			types.Metadata{
				Name:   "cluster-3",
				Labels: map[string]string{"env": "prod", "os": "mac"},
			},
			types.KubernetesClusterSpecV3{},
		)

		require.NoError(t, err)
		var servers []types.KubeServer
		for _, cluster := range []*types.KubernetesClusterV3{cluster1, cluster2, cluster3} {
			server, err := types.NewKubernetesServerV3FromCluster(cluster, "_", "_")
			require.NoError(t, err)
			servers = append(servers, server)
		}
		return servers
	}

	testcases := []struct {
		name                string
		predicateExpression string
		expectedLen         int
		assertMatch         require.BoolAssertionFunc
	}{
		{
			name:        "empty values",
			expectedLen: 3,
			assertMatch: require.True,
		},
		{
			name:                "all match",
			expectedLen:         3,
			predicateExpression: `labels.os == "mac"`,
			assertMatch:         require.True,
		},
		{
			name:                "some match",
			expectedLen:         2,
			predicateExpression: `labels.env == "prod"`,
			assertMatch:         require.True,
		},
		{
			name:                "single match",
			expectedLen:         1,
			predicateExpression: `labels.env == "staging"`,
			assertMatch:         require.True,
		},
		{
			name:                "no match",
			predicateExpression: `labels.env == "no-match"`,
			assertMatch:         require.False,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var filters MatchResourceFilter
			if tc.predicateExpression != "" {
				expression, err := NewResourceExpression(tc.predicateExpression)
				require.NoError(t, err)

				filters.PredicateExpression = expression
			}

			kubeServers := getKubeServers()
			atLeastOneMatch := false
			matchedServers := make([]types.KubeServer, 0, len(kubeServers))
			for _, kubeServer := range kubeServers {
				match, err := matchAndFilterKubeClusters(types.ResourceWithLabels(kubeServer), filters)
				require.NoError(t, err)
				if match {
					atLeastOneMatch = true
					matchedServers = append(matchedServers, kubeServer)
				}
			}
			tc.assertMatch(t, atLeastOneMatch)

			require.Len(t, matchedServers, tc.expectedLen)
		})
	}
}

// TestMatchResourceByFilters tests supported resource kinds and
// if a resource has contained resources, those contained resources
// are filtered instead.
func TestMatchResourceByFilters(t *testing.T) {
	t.Parallel()

	filterExpression, err := NewResourceExpression(`resource.metadata.name == "foo"`)
	require.NoError(t, err)

	testcases := []struct {
		name           string
		wantNotImplErr bool
		filters        MatchResourceFilter
		resource       func() types.ResourceWithLabels
	}{
		{
			name: "no filter should return true",
			resource: func() types.ResourceWithLabels {
				server, err := types.NewServer("foo", types.KindNode, types.ServerSpecV2{})
				require.NoError(t, err)
				return server
			},
			filters: MatchResourceFilter{ResourceKind: types.KindNode},
		},
		{
			name: "unsupported resource kind",
			resource: func() types.ResourceWithLabels {
				badResource, err := types.NewConnectionDiagnosticV1("123", map[string]string{},
					types.ConnectionDiagnosticSpecV1{
						Message: types.DiagnosticMessageSuccess,
					})
				require.NoError(t, err)
				return badResource
			},
			filters: MatchResourceFilter{
				ResourceKind:   "unsupported",
				SearchKeywords: []string{"nothing"},
			},
			wantNotImplErr: true,
		},
		{
			name: "app server",
			resource: func() types.ResourceWithLabels {
				appServer, err := types.NewAppServerV3(types.Metadata{
					Name: "_",
				}, types.AppServerSpecV3{
					HostID: "_",
					App: &types.AppV3{
						Metadata: types.Metadata{Name: "foo"},
						Spec:     types.AppSpecV3{URI: "_"},
					},
				})
				require.NoError(t, err)
				return appServer
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindAppServer,
				PredicateExpression: filterExpression,
			},
		},
		{
			name: "db server",
			resource: func() types.ResourceWithLabels {
				dbServer, err := types.NewDatabaseServerV3(types.Metadata{
					Name: "_",
				}, types.DatabaseServerSpecV3{
					HostID:   "_",
					Hostname: "_",
					Database: &types.DatabaseV3{
						Metadata: types.Metadata{Name: "foo"},
						Spec: types.DatabaseSpecV3{
							URI:      "_",
							Protocol: "_",
						},
					},
				})
				require.NoError(t, err)
				return dbServer
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindDatabaseServer,
				PredicateExpression: filterExpression,
			},
		},

		{
			name: "kube cluster",
			resource: func() types.ResourceWithLabels {
				cluster, err := types.NewKubernetesClusterV3FromLegacyCluster("_", &types.KubernetesCluster{
					Name: "foo",
				})
				require.NoError(t, err)
				return cluster
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindKubernetesCluster,
				PredicateExpression: filterExpression,
			},
		},
		{
			name: "node",
			resource: func() types.ResourceWithLabels {
				server, err := types.NewServer("foo", types.KindNode, types.ServerSpecV2{})
				require.NoError(t, err)
				return server
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindNode,
				PredicateExpression: filterExpression,
			},
		},
		{
			name: "windows desktop",
			resource: func() types.ResourceWithLabels {
				desktop, err := types.NewWindowsDesktopV3("foo", nil, types.WindowsDesktopSpecV3{Addr: "_"})
				require.NoError(t, err)
				return desktop
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindWindowsDesktop,
				PredicateExpression: filterExpression,
			},
		},

		{
			name: "AppServerOrSAMLIdPServiceProvider (App Server)r",
			resource: func() types.ResourceWithLabels {
				appServer, err := types.NewAppServerV3(types.Metadata{
					Name: "_",
				}, types.AppServerSpecV3{
					HostID: "_",
					App: &types.AppV3{
						Metadata: types.Metadata{Name: "foo"},
						Spec:     types.AppSpecV3{URI: "_"},
					},
				})
				require.NoError(t, err)
				return appServer
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindAppOrSAMLIdPServiceProvider,
				PredicateExpression: filterExpression,
			},
		},
		{
			name: "AppServerOrSAMLIdPServiceProvider (Service Provider)",
			resource: func() types.ResourceWithLabels {
				appOrSP, err := types.NewSAMLIdPServiceProvider(types.Metadata{Name: "foo"}, types.SAMLIdPServiceProviderSpecV1{EntityDescriptor: "<></>", EntityID: "_"})
				require.NoError(t, err)
				return appOrSP
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindAppOrSAMLIdPServiceProvider,
				PredicateExpression: filterExpression,
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resource := tc.resource()
			match, err := MatchResourceByFilters(resource, tc.filters, nil)

			switch tc.wantNotImplErr {
			case true:
				require.True(t, trace.IsNotImplemented(err))
				require.False(t, match)
			default:
				require.NoError(t, err)
				require.True(t, match)
			}
		})
	}
}

func TestResourceMatchersToTypes(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []ResourceMatcher
		out  []*types.DatabaseResourceMatcher
	}{
		{
			name: "empty",
			in:   []ResourceMatcher{},
			out:  []*types.DatabaseResourceMatcher{},
		},
		{
			name: "single element with single label",
			in: []ResourceMatcher{
				{Labels: types.Labels{"elem1": []string{"elem1"}}},
			},
			out: []*types.DatabaseResourceMatcher{
				{Labels: &types.Labels{"elem1": []string{"elem1"}}},
			},
		},
		{
			name: "single element with multiple labels",
			in: []ResourceMatcher{
				{Labels: types.Labels{"elem2": []string{"elem1", "elem2"}}},
			},
			out: []*types.DatabaseResourceMatcher{
				{Labels: &types.Labels{"elem2": []string{"elem1", "elem2"}}},
			},
		},
		{
			name: "multiple elements",
			in: []ResourceMatcher{
				{Labels: types.Labels{"elem1": []string{"elem1"}}},
				{Labels: types.Labels{"elem2": []string{"elem1", "elem2"}}},
				{Labels: types.Labels{"elem3": []string{"elem1", "elem2", "elem3"}}},
			},
			out: []*types.DatabaseResourceMatcher{
				{Labels: &types.Labels{"elem1": []string{"elem1"}}},
				{Labels: &types.Labels{"elem2": []string{"elem1", "elem2"}}},
				{Labels: &types.Labels{"elem3": []string{"elem1", "elem2", "elem3"}}},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.out, ResourceMatchersToTypes(tt.in))
		})
	}
}
