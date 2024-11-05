/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fetchers

import (
	"context"
	"testing"

	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestGKEFetcher(t *testing.T) {
	type args struct {
		location     string
		filterLabels types.Labels
		projectID    string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GKEFetcherConfig{
				GKEClient:     newPopulatedGCPMock(),
				ProjectClient: newPopulatedGCPProjectsMock(),
				FilterLabels:  tt.args.filterLabels,
				Location:      tt.args.location,
				ProjectID:     tt.args.projectID,
				Log:           logrus.New(),
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
}

func (m *mockGKEAPI) ListClusters(ctx context.Context, projectID string, location string) ([]gcp.GKECluster, error) {
	var clusters []gcp.GKECluster
	for _, cluster := range m.clusters {
		if cluster.ProjectID != projectID {
			continue
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func newPopulatedGCPMock() *mockGKEAPI {
	return &mockGKEAPI{
		clusters: gkeMockClusters,
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
