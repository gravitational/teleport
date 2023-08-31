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
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestGKEFetcher(t *testing.T) {
	type args struct {
		location     string
		filterLabels types.Labels
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
			},
			want: gkeClustersToResources(t, gkeMockClusters...),
		},
		{
			name: "list prod clusters",
			args: args{
				location: types.Wildcard,
				filterLabels: types.Labels{
					"env": []string{"prod"},
				},
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
			},
			want: gkeClustersToResources(t, gkeMockClusters[2:]...),
		},
		{
			name: "filter not found",
			args: args{
				location: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"none"},
				},
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
			},
			want: gkeClustersToResources(t, gkeMockClusters...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GKEFetcherConfig{
				Client:       newPopulatedGCPMock(),
				FilterLabels: tt.args.filterLabels,
				Location:     tt.args.location,
				Log:          logrus.New(),
			}
			fetcher, err := NewGKEFetcher(cfg)
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
	return m.clusters, nil
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
}

func gkeClustersToResources(t *testing.T, clusters ...gcp.GKECluster) types.ResourcesWithLabels {
	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		kubeCluster, err := services.NewKubeClusterFromGCPGKE(cluster)
		require.NoError(t, err)
		require.True(t, kubeCluster.IsGCP())
		common.ApplyGKENameSuffix(kubeCluster)
		kubeClusters = append(kubeClusters, kubeCluster)
	}
	return kubeClusters.AsResources()
}
