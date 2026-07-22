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
	"fmt"
	"slices"
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

func TestSimplifyAzureMatchers(t *testing.T) {
	matchers := []types.AzureMatcher{
		{
			Subscriptions: []string{"sub-1", types.Wildcard, "sub-1"},
			Regions:       []string{"eu-west-1", "eu-west-2"},
			Types:         []string{"mysql", "mysql", "postgres"},
			ResourceTags:  types.Labels{"env": []string{"prod"}},
			Params: &types.InstallerParams{
				JoinMethod: types.JoinMethodAzure,
				JoinToken:  "token-1",
				Azure: &types.AzureInstallerParams{
					ClientID: "client-1",
				},
			},
			Integration: "integration-1",
		},
		{
			ResourceGroups: []string{
				"rg-1",
				types.Wildcard,
				"rg-1",
			},
			Types:       []string{"redis"},
			Integration: "integration-2",
		},
	}

	simplified := SimplifyAzureMatchers(matchers)

	want := []types.AzureMatcher{
		{
			Subscriptions:  []string{types.Wildcard},
			ResourceGroups: []string{types.Wildcard},
			Regions:        []string{"eu-west-1", "eu-west-2"},
			Types:          []string{"mysql", "postgres"},
			ResourceTags:   types.Labels{"env": []string{"prod"}},
			Params: &types.InstallerParams{
				JoinMethod: types.JoinMethodAzure,
				JoinToken:  "token-1",
				Azure: &types.AzureInstallerParams{
					ClientID: "client-1",
				},
			},
			Integration: "integration-1",
		},
		{
			Subscriptions:  []string{types.Wildcard},
			ResourceGroups: []string{types.Wildcard},
			Regions:        []string{types.Wildcard},
			Types:          []string{"redis"},
			Integration:    "integration-2",
		},
	}

	require.Equal(t, want, simplified)
}

// TestSimplifyAzureMatchersDoesNotMutateInput guards against a regression where
// SimplifyAzureMatchers normalized regions through an aliased input slice,
// mutating the caller's matcher and racing with concurrent readers.
func TestSimplifyAzureMatchersDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  []string{"sub-1"},
			ResourceGroups: []string{"rg-1"},
			// "East US" normalizes to "eastus", so any in-place write is observable by value comparison.
			Regions: []string{"East US"},
			Types:   []string{"vm"},
		},
	}
	regionsBackingArray := matchers[0].Regions

	simplified := SimplifyAzureMatchers(matchers)

	require.Equal(t, []string{"East US"}, matchers[0].Regions,
		"input regions slice must not be mutated")
	require.Equal(t, "East US", regionsBackingArray[0],
		"input regions backing array must not be mutated")
	require.Equal(t, []string{"eastus"}, simplified[0].Regions,
		"output regions must be normalized")
}

// TestSimplifyAzureMatchersDeduplicatesNormalizedRegions guards against a
// regression where deduplication ran before normalization, leaving case-variant
// inputs like "East US" and "eastus" as distinct entries in the output.
func TestSimplifyAzureMatchersDeduplicatesNormalizedRegions(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  []string{"sub-1"},
			ResourceGroups: []string{"rg-1"},
			// Four inputs, three of which normalize to "eastus", one to "eastus2".
			// Byte-equality dedup before normalization would let all four through.
			Regions: []string{"East US", "eastus", "EASTUS", "East US 2"},
			Types:   []string{"vm"},
		},
	}

	simplified := SimplifyAzureMatchers(matchers)

	require.ElementsMatch(t, []string{"eastus", "eastus2"}, simplified[0].Regions,
		"case-variant regions must collapse to one normalized entry per location")
}

// TestSimplifyAzureMatchersCollapsesRegionsWildcard locks in the Regions
// wildcard branch: when any region entry is the wildcard, the output must
// collapse to a single wildcard rather than keep the explicit entries
// alongside it. Without this test the wildcard collapse for Regions has no
// fixture coverage (existing tests only exercise the wildcard branch via an
// empty Regions slice).
func TestSimplifyAzureMatchersCollapsesRegionsWildcard(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  []string{"sub-1"},
			ResourceGroups: []string{"rg-1"},
			Regions:        []string{"East US", types.Wildcard, "westus"},
			Types:          []string{"vm"},
		},
	}

	simplified := SimplifyAzureMatchers(matchers)

	require.Equal(t, []string{types.Wildcard}, simplified[0].Regions,
		"any wildcard in regions must collapse the slice to a single wildcard")
}

// TestSimplifyAzureMatchersTrimsBeforeDedupe guards against a regression where
// Subscriptions and ResourceGroups deduped without first trimming whitespace,
// letting hand-edited config like " sub-1" survive byte-equality dedup as a
// distinct entry from "sub-1" and reach downstream as duplicate scopes.
func TestSimplifyAzureMatchersTrimsBeforeDedupe(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  []string{" sub-1", "sub-1", "sub-2  ", "  ", "sub-2"},
			ResourceGroups: []string{"rg-a", "  rg-a", "rg-b ", "", "\t"},
			Regions:        []string{"eu-west-1"},
			Types:          []string{"vm"},
		},
	}

	simplified := SimplifyAzureMatchers(matchers)
	require.Len(t, simplified, 1)

	require.Equal(t, []string{"sub-1", "sub-2"}, simplified[0].Subscriptions,
		"whitespace variants must collapse to a single trimmed entry, "+
			"first-occurrence order preserved")
	require.Equal(t, []string{"rg-a", "rg-b"}, simplified[0].ResourceGroups,
		"resource group whitespace variants must collapse to a single trimmed entry")
	require.Equal(t, []string{"eu-west-1"}, simplified[0].Regions)
	require.Equal(t, []string{"vm"}, simplified[0].Types)
}

// TestSimplifyAzureMatchersTrimmedWildcardCollapses guards against a
// regression where a hand-edited wildcard with stray whitespace (" * ")
// failed to trigger the wildcard collapse because the slices.Contains check
// ran against untrimmed inputs.
func TestSimplifyAzureMatchersTrimmedWildcardCollapses(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  []string{" * ", "sub-1"},
			ResourceGroups: []string{"\t*\n", "rg-a"},
			Regions:        []string{"eu-west-1"},
			Types:          []string{"vm"},
		},
	}

	simplified := SimplifyAzureMatchers(matchers)
	require.Len(t, simplified, 1)

	require.Equal(t, []string{types.Wildcard}, simplified[0].Subscriptions,
		"trimmed wildcard alongside concrete entries must collapse to a single wildcard")
	require.Equal(t, []string{types.Wildcard}, simplified[0].ResourceGroups,
		"trimmed wildcard alongside concrete entries must collapse to a single wildcard")
}

// TestSimplifyAzureMatchersAllEmptyEntriesDoNotWidenToWildcard guards
// against the silent escalation pathology: a malformed selector list where
// every entry trims to empty (e.g. [" "], ["", "\t"]) MUST NOT widen to the
// wildcard. Widening would turn a config typo (`subscriptions: [" "]`) into
// "discover every subscription in the tenant", which is the opposite of the
// operator's intent. The original input is preserved instead so the
// downstream Azure SDK call surfaces the malformed scope as an invalid-scope
// error.
//
// The truly empty case (len(input) == 0) still collapses to wildcard, since
// that is the documented "discover everything" convention.
func TestSimplifyAzureMatchersAllEmptyEntriesDoNotWidenToWildcard(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  []string{"", "  ", "\t"},
			ResourceGroups: []string{" "},
			Regions:        []string{"eu-west-1"},
			Types:          []string{"vm"},
		},
	}

	simplified := SimplifyAzureMatchers(matchers)
	require.Len(t, simplified, 1)

	require.Equal(t, []string{"", "  ", "\t"}, simplified[0].Subscriptions,
		"all-whitespace subscriptions must NOT widen to wildcard; "+
			"preserve the malformed input so the SDK rejects it instead of "+
			"silently discovering every subscription in the tenant")
	require.Equal(t, []string{" "}, simplified[0].ResourceGroups,
		"all-whitespace resource groups must NOT widen to wildcard; "+
			"preserve the malformed input so the SDK rejects it instead of "+
			"silently discovering every resource group")
}

// TestSimplifyAzureMatchersTrulyEmptyCollapsesToWildcard pins the documented
// convention for the truly empty case (len == 0): match everything. This is
// the opposite of the all-whitespace case and must remain distinct.
func TestSimplifyAzureMatchersTrulyEmptyCollapsesToWildcard(t *testing.T) {
	t.Parallel()
	matchers := []types.AzureMatcher{
		{
			Subscriptions:  nil,
			ResourceGroups: []string{},
			Regions:        []string{"eu-west-1"},
			Types:          []string{"vm"},
		},
	}

	simplified := SimplifyAzureMatchers(matchers)
	require.Len(t, simplified, 1)

	require.Equal(t, []string{types.Wildcard}, simplified[0].Subscriptions,
		"truly empty subscription list (len == 0) collapses to wildcard "+
			"per documented 'discover everything' convention")
	require.Equal(t, []string{types.Wildcard}, simplified[0].ResourceGroups,
		"truly empty resource group list (len == 0) collapses to wildcard "+
			"per documented 'discover everything' convention")
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

func TestMatchResourceByHealthStatus(t *testing.T) {
	var dbServers []types.ResourceWithLabels
	for i := range 3 {
		name := fmt.Sprintf("db-server-%d", i)
		dbServer, err := types.NewDatabaseServerV3(types.Metadata{
			Name: name,
		}, types.DatabaseServerSpecV3{
			HostID:   name,
			Hostname: name,
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{Name: "foo"},
				Spec: types.DatabaseSpecV3{
					URI:      "localhost:12345",
					Protocol: "postgres",
				},
			},
		})
		require.NoError(t, err)
		switch i {
		case 0:
			dbServer.SetTargetHealth(types.TargetHealth{
				Status: string(types.TargetHealthStatusHealthy),
			})
		case 1:
			dbServer.SetTargetHealth(types.TargetHealth{
				Status:  string(types.TargetHealthStatusUnhealthy),
				Message: "failed to fizz the buzz",
			})
		case 2:
			// health status is supported, but none reported (e.g. old db agent)
		}
		dbServers = append(dbServers, dbServer)
	}

	server, err := types.NewServerWithLabels("server", types.KindNode, types.ServerSpecV2{
		Hostname: "test-hostname",
		Addr:     "test-addr",
		CmdLabels: map[string]types.CommandLabelV2{
			"version": {
				Result: "v8",
			},
		},
	}, map[string]string{
		"env": "prod",
		"os":  "mac",
	})
	require.NoError(t, err)

	tests := []struct {
		name             string
		filterExpression string
		resources        []types.ResourceWithLabels
		matchedNames     []string
		wantErr          string
	}{
		{
			name:             "healthy db status",
			resources:        dbServers,
			filterExpression: `health.status == "healthy"`,
			matchedNames:     []string{"db-server-0"},
		},
		{
			name:             "unhealthy db status",
			resources:        dbServers,
			filterExpression: `health.status == "unhealthy"`,
			matchedNames:     []string{"db-server-1"},
		},
		{
			name:             "db health status is empty",
			resources:        dbServers,
			filterExpression: `health.status == ""`,
			matchedNames:     []string{"db-server-2"},
		},
		{
			name:             "db combo",
			resources:        dbServers,
			filterExpression: `contains(split("healthy,unhealthy", ","), health.status)`,
			matchedNames:     []string{"db-server-0", "db-server-1"},
		},
		{
			name:             "server health is empty",
			resources:        []types.ResourceWithLabels{server},
			filterExpression: `!exists(health.status)`,
			matchedNames:     []string{"server"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filterExpression, err := NewResourceExpression(test.filterExpression)
			require.NoError(t, err)

			filter := MatchResourceFilter{
				ResourceKind:        types.KindDatabaseServer,
				PredicateExpression: filterExpression,
			}

			var matched []types.ResourceWithLabels
			for _, r := range test.resources {
				match, err := MatchResourceByFilters(r, filter, nil)
				require.NoError(t, err)
				if match {
					matched = append(matched, r)
				}
			}
			require.Equal(t,
				test.matchedNames,
				slices.Collect(types.ResourceNames(matched)),
			)
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
				cluster, err := types.NewKubernetesClusterV3FromLegacyCluster("", &types.KubernetesCluster{
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
			name: "SAMLIdPServiceProvider",
			resource: func() types.ResourceWithLabels {
				sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{Name: "foo"}, types.SAMLIdPServiceProviderSpecV1{ACSURL: "host", EntityID: "host"})
				require.NoError(t, err)
				return sp
			},
			filters: MatchResourceFilter{
				ResourceKind:        types.KindSAMLIdPServiceProvider,
				PredicateExpression: filterExpression,
			},
		},
		{
			name: "MCP server with mcp kind filter",
			resource: func() types.ResourceWithLabels {
				return newAppServerFromApp(t, newMCPServerApp(t, "foo"))
			},
			filters: MatchResourceFilter{
				Kinds:               []string{types.KindMCP},
				PredicateExpression: filterExpression,
			},
		},
		{
			name: "MCP server with app kind filter",
			resource: func() types.ResourceWithLabels {
				return newAppServerFromApp(t, newMCPServerApp(t, "foo"))
			},
			filters: MatchResourceFilter{
				Kinds:               []string{types.KindApp},
				PredicateExpression: filterExpression,
			},
		},
	}

	for _, tc := range testcases {
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

func TestMatchResourcesByFilters(t *testing.T) {
	appServers := make(types.AppServers, 5)
	oddOrEven := func(i int) string {
		if i%2 == 1 {
			return "odd"
		}
		return "even"
	}
	for i := range len(appServers) {
		app, err := types.NewAppV3(types.Metadata{
			Name:   fmt.Sprintf("app-%d", i),
			Labels: map[string]string{"group": oddOrEven(i)},
		}, types.AppSpecV3{
			URI: "http://localhost:8888",
		})
		require.NoError(t, err)
		appServers[i] = newAppServerFromApp(t, app)
	}

	evenAppServers, err := MatchResourcesByFilters(appServers, MatchResourceFilter{
		ResourceKind: types.KindAppServer,
		Labels:       map[string]string{"group": "even"},
	})

	require.NoError(t, err)
	require.IsType(t, types.AppServers{}, evenAppServers)
	require.Equal(t,
		[]string{"app-0", "app-2", "app-4"},
		slices.Collect(types.ResourceNames(evenAppServers)),
	)
}

func newMCPServerApp(t *testing.T, name string) *types.AppV3 {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name: name,
	}, types.AppSpecV3{
		MCP: &types.MCP{
			Command:       "test",
			RunAsHostUser: "test",
		},
	})
	require.NoError(t, err)
	return app
}

func newAppServerFromApp(t *testing.T, app *types.AppV3) types.AppServer {
	appServer, err := types.NewAppServerV3FromApp(app, "_", "_")
	require.NoError(t, err)
	return appServer
}
