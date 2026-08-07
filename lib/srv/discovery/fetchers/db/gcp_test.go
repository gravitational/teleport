/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package db

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/gcp/gcptest"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestGCPFetcher_CloudSQL(t *testing.T) {
	t.Parallel()

	makeInstance := func(name string, opts ...func(*sqladmin.DatabaseInstance)) *sqladmin.DatabaseInstance {
		instance := &sqladmin.DatabaseInstance{
			Name:            name,
			Project:         "proj-1",
			Region:          "us-central1",
			State:           "RUNNABLE",
			DatabaseVersion: "POSTGRES_14",
			InstanceType:    "CLOUD_SQL_INSTANCE",
			IpAddresses:     []*sqladmin.IpMapping{{Type: "PRIMARY", IpAddress: "1.2.3.4"}},
			Settings:        &sqladmin.Settings{},
		}
		for _, opt := range opts {
			opt(instance)
		}
		return instance
	}
	withUserLabels := func(labels map[string]string) func(*sqladmin.DatabaseInstance) {
		return func(instance *sqladmin.DatabaseInstance) { instance.Settings.UserLabels = labels }
	}
	withRegion := func(region string) func(*sqladmin.DatabaseInstance) {
		return func(instance *sqladmin.DatabaseInstance) { instance.Region = region }
	}
	withProject := func(projectID string) func(*sqladmin.DatabaseInstance) {
		return func(instance *sqladmin.DatabaseInstance) { instance.Project = projectID }
	}

	wildcardLabels := types.Labels{types.Wildcard: {types.Wildcard}}

	tests := []struct {
		name      string
		instances []*sqladmin.DatabaseInstance
		projects  []gcp.Project
		matcher   types.GCPMatcher
		// wantFetchers is the expected number of fetchers, one per project.
		wantFetchers int
		// wantInstances are the instances expected to survive filtering.
		wantInstances []*sqladmin.DatabaseInstance
		// wantNames are the expected database names, asserted when set.
		wantNames []string
	}{
		{
			name: "matcher labels filter instances",
			instances: []*sqladmin.DatabaseInstance{
				makeInstance("prod-db", withUserLabels(map[string]string{"env": "prod"})),
				makeInstance("dev-db", withUserLabels(map[string]string{"env": "dev"})),
			},
			matcher: types.GCPMatcher{
				Types:      []string{types.GCPMatcherCloudSQL},
				ProjectIDs: []string{"proj-1"},
				Locations:  []string{types.Wildcard},
				Labels:     types.Labels{"env": {"prod"}},
			},
			wantFetchers: 1,
			wantInstances: []*sqladmin.DatabaseInstance{
				makeInstance("prod-db", withUserLabels(map[string]string{"env": "prod"})),
			},
			wantNames: []string{"prod-db-cloudsql-us-central1-proj-1"},
		},
		{
			name: "name-override label wins and skips the suffix",
			instances: []*sqladmin.DatabaseInstance{
				makeInstance("real-name", withUserLabels(map[string]string{types.GCPDatabaseNameOverrideLabel: "custom-name"})),
			},
			matcher: types.GCPMatcher{
				Types:      []string{types.GCPMatcherCloudSQL},
				ProjectIDs: []string{"proj-1"},
				Locations:  []string{types.Wildcard},
				Labels:     wildcardLabels,
			},
			wantFetchers: 1,
			wantInstances: []*sqladmin.DatabaseInstance{
				makeInstance("real-name", withUserLabels(map[string]string{types.GCPDatabaseNameOverrideLabel: "custom-name"})),
			},
			wantNames: []string{"custom-name"},
		},
		{
			// Matcher locations are passed to the Cloud SQL API as a region filter.
			name: "instances outside matcher locations are dropped",
			instances: []*sqladmin.DatabaseInstance{
				makeInstance("in-region"),
				makeInstance("other-region", withRegion("europe-west1")),
			},
			matcher: types.GCPMatcher{
				Types:      []string{types.GCPMatcherCloudSQL},
				ProjectIDs: []string{"proj-1"},
				Locations:  []string{"us-central1"},
				Labels:     wildcardLabels,
			},
			wantFetchers: 1,
			wantInstances: []*sqladmin.DatabaseInstance{
				makeInstance("in-region"),
			},
		},
		{
			// A wildcard expanding to zero visible projects yields no
			// databases and no error (a warning is logged).
			name: "wildcard project with no visible projects",
			instances: []*sqladmin.DatabaseInstance{
				makeInstance("db-1"),
			},
			projects: []gcp.Project{},
			matcher: types.GCPMatcher{
				Types:      []string{types.GCPMatcherCloudSQL},
				ProjectIDs: []string{types.Wildcard},
				Locations:  []string{types.Wildcard},
				Labels:     wildcardLabels,
			},
			wantFetchers: 1,
		},
		{
			// A wildcard project matcher is one fetcher that expands to every
			// visible project at fetch time.
			name: "wildcard project discovers all visible projects",
			instances: []*sqladmin.DatabaseInstance{
				makeInstance("db-1"),
				makeInstance("db-2", withProject("proj-2")),
			},
			projects: []gcp.Project{{ID: "proj-1"}, {ID: "proj-2"}},
			matcher: types.GCPMatcher{
				Types:      []string{types.GCPMatcherCloudSQL},
				ProjectIDs: []string{types.Wildcard},
				Locations:  []string{types.Wildcard},
				Labels:     wildcardLabels,
			},
			wantFetchers: 1,
			wantInstances: []*sqladmin.DatabaseInstance{
				makeInstance("db-1"),
				makeInstance("db-2", withProject("proj-2")),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clients := &gcptest.Clients{
				GCPSQL: &mocks.GCPSQLAdminClientMock{DatabaseInstances: tt.instances},
			}
			if tt.projects != nil {
				clients.GCPProjects = &mocks.GCPProjectsClientMock{Projects: tt.projects}
			}

			fetchers := mustMakeGCPFetchers(t, clients, []types.GCPMatcher{tt.matcher})
			require.Len(t, fetchers, tt.wantFetchers)
			for _, fetcher := range fetchers {
				require.Equal(t, types.GCPMatcherCloudSQL, fetcher.FetcherType())
			}

			want := common.DiscoverCloudSQLDatabases(t.Context(), logtest.NewLogger(), tt.wantInstances)
			require.Len(t, want, len(tt.wantInstances))
			for _, db := range want {
				common.ApplyGCPDatabaseNameSuffix(db, types.GCPMatcherCloudSQL)
			}

			got := mustGetDatabases(t, fetchers)
			require.ElementsMatch(t, want, got)

			if tt.wantNames != nil {
				names := make([]string, 0, len(got))
				for _, db := range got {
					names = append(names, db.GetName())
				}
				require.ElementsMatch(t, tt.wantNames, names)
			}
		})
	}
}

func TestGCPFetcher_Accessors(t *testing.T) {
	t.Parallel()

	mock := &mocks.GCPSQLAdminClientMock{}
	clients := &gcptest.Clients{GCPSQL: mock}

	matcher := types.GCPMatcher{
		Types:      []string{types.GCPMatcherCloudSQL},
		ProjectIDs: []string{"proj-1"},
		Locations:  []string{types.Wildcard},
		Labels:     types.Labels{"env": {"prod"}},
	}

	const discoveryConfigName = "my-dc"
	fetchers, err := MakeGCPFetchers(logtest.NewLogger(), clients, []types.GCPMatcher{matcher}, discoveryConfigName)
	require.NoError(t, err)
	require.Len(t, fetchers, 1)

	fetcher := fetchers[0]
	require.Equal(t, types.KindDatabase, fetcher.ResourceType())
	require.Empty(t, fetcher.IntegrationName())
	require.Equal(t, discoveryConfigName, fetcher.GetDiscoveryConfigName())
	require.NotEmpty(t, fetcher.String())
	require.Contains(t, fetcher.String(), "gcpFetcher")
}

func TestGCPFetcherConfig_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()
	valid := func() gcpFetcherConfig {
		return gcpFetcherConfig{
			Type:       types.GCPMatcherCloudSQL,
			GCPClients: &gcptest.Clients{},
			ProjectID:  "proj-1",
			Labels:     types.Labels{"*": {"*"}},
		}
	}

	t.Run("valid config defaults the logger", func(t *testing.T) {
		cfg := valid()
		require.NoError(t, cfg.CheckAndSetDefaults())
		require.NotNil(t, cfg.Logger)
	})

	t.Run("missing required fields are rejected", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*gcpFetcherConfig)
		}{
			{name: "missing type", mutate: func(c *gcpFetcherConfig) { c.Type = "" }},
			{name: "missing clients", mutate: func(c *gcpFetcherConfig) { c.GCPClients = nil }},
			{name: "missing project", mutate: func(c *gcpFetcherConfig) { c.ProjectID = "" }},
			{name: "missing labels", mutate: func(c *gcpFetcherConfig) { c.Labels = nil }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := valid()
				tt.mutate(&cfg)
				err := cfg.CheckAndSetDefaults()
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
			})
		}
	})
}

func TestGCPFetcherConfig_RegionFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		locations []string
		want      []string
	}{
		{name: "empty passes no filter", locations: nil, want: nil},
		{name: "wildcard passes no filter", locations: []string{types.Wildcard}, want: nil},
		{name: "wildcard among locations passes no filter", locations: []string{"us-central1", types.Wildcard}, want: nil},
		{name: "locations pass through", locations: []string{"us-central1", "europe-west1"}, want: []string{"us-central1", "europe-west1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := gcpFetcherConfig{Locations: tt.locations}
			require.Equal(t, tt.want, cfg.regionFilter())
		})
	}
}

func TestMakeGCPFetchers(t *testing.T) {
	t.Parallel()
	clients := &gcptest.Clients{GCPSQL: &mocks.GCPSQLAdminClientMock{}}

	t.Run("one fetcher per project", func(t *testing.T) {
		matcher := types.GCPMatcher{
			Types:      []string{types.GCPMatcherCloudSQL},
			ProjectIDs: []string{"proj-1", "proj-2"},
			Labels:     types.Labels{"*": {"*"}},
		}
		fetchers, err := MakeGCPFetchers(logtest.NewLogger(), clients, []types.GCPMatcher{matcher}, "")
		require.NoError(t, err)
		require.Len(t, fetchers, 2)
		for _, f := range fetchers {
			require.Equal(t, types.GCPMatcherCloudSQL, f.FetcherType())
		}
	})

	t.Run("duplicate projects are deduplicated", func(t *testing.T) {
		matcher := types.GCPMatcher{
			Types:      []string{types.GCPMatcherCloudSQL},
			ProjectIDs: []string{"proj-1", "proj-1"},
			Labels:     types.Labels{"*": {"*"}},
		}
		fetchers, err := MakeGCPFetchers(logtest.NewLogger(), clients, []types.GCPMatcher{matcher}, "")
		require.NoError(t, err)
		require.Len(t, fetchers, 1)
	})

	t.Run("wildcard project is a single fetcher expanded at fetch time", func(t *testing.T) {
		matcher := types.GCPMatcher{
			Types:      []string{types.GCPMatcherCloudSQL},
			ProjectIDs: []string{types.Wildcard},
			Labels:     types.Labels{"*": {"*"}},
		}
		fetchers, err := MakeGCPFetchers(logtest.NewLogger(), clients, []types.GCPMatcher{matcher}, "")
		require.NoError(t, err)
		require.Len(t, fetchers, 1)
	})

	t.Run("non-database matcher type is rejected", func(t *testing.T) {
		matcher := types.GCPMatcher{
			Types:      []string{types.GCPMatcherKubernetes}, // not a database matcher
			ProjectIDs: []string{"proj-1"},
			Labels:     types.Labels{"*": {"*"}},
		}
		_, err := MakeGCPFetchers(logtest.NewLogger(), clients, []types.GCPMatcher{matcher}, "")
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	})
}

func TestIsGCPMatcherType(t *testing.T) {
	t.Parallel()
	require.True(t, IsGCPMatcherType(types.GCPMatcherCloudSQL))
	require.False(t, IsGCPMatcherType(types.GCPMatcherKubernetes))
	require.False(t, IsGCPMatcherType("nonexistent"))
}
