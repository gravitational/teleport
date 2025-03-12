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

package fetchers

import (
	"context"
	"testing"

	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGKEFetcher(t *testing.T) {
	type args struct {
		location     string
		filterLabels types.Labels
		projectID    string
		errs         map[string]error
	}
	tests := []struct {
		name string
		args args
		want types.ResourcesWithLabels
	}{
		{
			name: "list everything",
			args: args{
				location: types.Wildcard,
				filterLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
				projectID: "p1",
			},
			want: gkeClustersToResources(t, gkeMockClusters[:4]...),
		},
		{
			name: "list prod clusters",
			args: args{
				location: types.Wildcard,
				filterLabels: types.Labels{
					"env": []string{"prod"},
				},
				projectID: "p1",
			},
			want: gkeClustersToResources(t, gkeMockClusters[:2]...),
		},
		{
			name: "list stg clusters from central",
			args: args{
				location: "uswest2",
				filterLabels: types.Labels{
					"env":      []string{"stg"},
					"location": []string{"central-1"},
				},
				projectID: "p1",
			},
			want: gkeClustersToResources(t, gkeMockClusters[2:4]...),
		},
		{
			name: "filter not found",
			args: args{
				location: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"none"},
				},
				projectID: "p1",
			},
			want: gkeClustersToResources(t),
		},

		{
			name: "list everything with specified values",
			args: args{
				location: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"prod", "stg"},
				},
				projectID: "p1",
			},
			want: gkeClustersToResources(t, gkeMockClusters[:4]...),
		},
		{
			name: "list everything with wildcard project",
			args: args{
				location: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"prod", "stg"},
				},
				projectID: "*",
			},
			want: gkeClustersToResources(t, gkeMockClusters...),
		},
		{
			name: "list everything but one project misses permissions",
			args: args{
				location: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"prod", "stg"},
				},
				projectID: "*",
				errs: map[string]error{
					"p2": trace.AccessDenied("no access"),
				},
			},
			want: gkeClustersToResources(t, gkeMockClusters[:4]...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPopulatedGCPMock(tt.args.errs)
			cfg := GKEFetcherConfig{
				GKEClient:     client,
				ProjectClient: newPopulatedGCPProjectsMock(),
				FilterLabels:  tt.args.filterLabels,
				Location:      tt.args.location,
				ProjectID:     tt.args.projectID,
				Logger:        utils.NewSlogLoggerForTests(),
			}
			fetcher, err := NewGKEFetcher(context.Background(), cfg)
			require.NoError(t, err)
			resources, err := fetcher.Get(context.Background())
			require.NoError(t, err)

			require.Equal(t, tt.want.ToMap(), resources.ToMap())
		})
	}
}

type mockGKEAPI struct {
	gcp.GKEClient
	clusters []gcp.GKECluster
	errs     map[string]error
}

func (m *mockGKEAPI) ListClusters(ctx context.Context, projectID string, location string) ([]gcp.GKECluster, error) {
	if err, ok := m.errs[projectID]; ok {
		return nil, err
	}
	var clusters []gcp.GKECluster
	for _, cluster := range m.clusters {
		if cluster.ProjectID != projectID {
			continue
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func newPopulatedGCPMock(errs map[string]error) *mockGKEAPI {
	return &mockGKEAPI{
		clusters: gkeMockClusters,
		errs:     errs,
	}
}

var gkeMockClusters = []gcp.GKECluster{
	{
		Name:   "cluster1",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			"env":      "prod",
			"location": "central-1",
		},
		ProjectID:   "p1",
		Location:    "central-1",
		Description: "desc1",
	},
	{
		Name:   "cluster2",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			"env":      "prod",
			"location": "central-1",
		},
		ProjectID:   "p1",
		Location:    "central-1",
		Description: "desc1",
	},
	{
		Name:   "cluster3",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			"env":      "stg",
			"location": "central-1",
		},
		ProjectID:   "p1",
		Location:    "central-1",
		Description: "desc1",
	},
	{
		Name:   "cluster4",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			"env":      "stg",
			"location": "central-1",
		},
		ProjectID:   "p1",
		Location:    "central-1",
		Description: "desc1",
	},
	{
		Name:   "cluster5",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			"env":      "stg",
			"location": "central-1",
		},
		ProjectID:   "p2",
		Location:    "central-1",
		Description: "desc1",
	},
	{
		Name:   "cluster6",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			"env":      "stg",
			"location": "central-1",
		},
		ProjectID:   "p2",
		Location:    "central-1",
		Description: "desc1",
	},
}

func gkeClustersToResources(t *testing.T, clusters ...gcp.GKECluster) types.ResourcesWithLabels {
	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		kubeCluster, err := common.NewKubeClusterFromGCPGKE(cluster)
		require.NoError(t, err)
		require.True(t, kubeCluster.IsGCP())
		common.ApplyGKENameSuffix(kubeCluster)
		kubeClusters = append(kubeClusters, kubeCluster)
	}
	return kubeClusters.AsResources()
}

type mockProjectsAPI struct {
	gcp.ProjectsClient
	projects []gcp.Project
}

func (m *mockProjectsAPI) ListProjects(ctx context.Context) ([]gcp.Project, error) {
	return m.projects, nil
}

func newPopulatedGCPProjectsMock() *mockProjectsAPI {
	return &mockProjectsAPI{
		projects: []gcp.Project{
			{
				ID:   "p1",
				Name: "project1",
			},
			{
				ID:   "p2",
				Name: "project2",
			},
		},
	}
}
