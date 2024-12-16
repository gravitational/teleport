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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAKSFetcher(t *testing.T) {
	type args struct {
		regions []string
		// ResourceGroups are the Azure resource groups the clusters must belong to.
		resourceGroups []string
		// FilterLabels are the filter criteria.
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
				regions:        []string{types.Wildcard},
				resourceGroups: []string{types.Wildcard},
				filterLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
			},
			want: aksClustersToResources(t, append(aksMockClusters["group1"], aksMockClusters["group2"]...)...),
		},
		{
			name: "list prod clusters",
			args: args{
				regions:        []string{types.Wildcard},
				resourceGroups: []string{types.Wildcard},
				filterLabels: types.Labels{
					"env": []string{"prod"},
				},
			},
			want: aksClustersToResources(t, aksMockClusters["group1"]...),
		},
		{
			name: "list stg clusters from uswest2",
			args: args{
				regions:        []string{"uswest2"},
				resourceGroups: []string{types.Wildcard},
				filterLabels: types.Labels{
					"env": []string{"stg"},
				},
			},
			want: aksClustersToResources(t, aksMockClusters["group2"][1]),
		},
		{
			name: "list clusters from uswest2",
			args: args{
				regions:        []string{"uswest2"},
				resourceGroups: []string{types.Wildcard},
				filterLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
			},
			want: aksClustersToResources(t, aksMockClusters["group1"][1], aksMockClusters["group2"][1]),
		},
		{
			name: "list clusters from group 2 and uswest2",
			args: args{
				regions:        []string{"uswest2"},
				resourceGroups: []string{"group2"},
				filterLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
			},
			want: aksClustersToResources(t, aksMockClusters["group2"][1]),
		},
		{
			name: "list everything with specified values",
			args: args{
				regions:        []string{"uswest2", "uswest1"},
				resourceGroups: []string{"group1", "group2"},
				filterLabels: types.Labels{
					"env": []string{"prod", "stg"},
				},
			},
			want: aksClustersToResources(t, append(aksMockClusters["group1"], aksMockClusters["group2"]...)...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AKSFetcherConfig{
				Client:         newPopulatedAKSMock(),
				FilterLabels:   tt.args.filterLabels,
				Regions:        tt.args.regions,
				ResourceGroups: tt.args.resourceGroups,
				Logger:         utils.NewSlogLoggerForTests(),
			}
			fetcher, err := NewAKSFetcher(cfg)
			require.NoError(t, err)
			resources, err := fetcher.Get(context.Background())
			require.NoError(t, err)

			require.Equal(t, tt.want.ToMap(), resources.ToMap())
		})
	}
}

type mockAKSAPI struct {
	azure.AKSClient
	group map[string][]*azure.AKSCluster
}

func (m *mockAKSAPI) ListAll(ctx context.Context) ([]*azure.AKSCluster, error) {
	result := make([]*azure.AKSCluster, 0, 10)
	for _, v := range m.group {
		result = append(result, v...)
	}
	return result, nil
}

func (m *mockAKSAPI) ListWithinGroup(ctx context.Context, group string) ([]*azure.AKSCluster, error) {
	return m.group[group], nil
}

func newPopulatedAKSMock() *mockAKSAPI {
	return &mockAKSAPI{
		group: aksMockClusters,
	}
}

var aksMockClusters = map[string][]*azure.AKSCluster{
	"group1": {
		{
			Name:           "cluster1",
			GroupName:      "group1",
			TenantID:       "tenantID",
			Location:       "uswest1",
			SubscriptionID: "subID",
			Tags: map[string]string{
				"env":      "prod",
				"location": "uswest1",
			},
			Properties: azure.AKSClusterProperties{},
		},
		{
			Name:           "cluster2",
			GroupName:      "group1",
			TenantID:       "tenantID",
			Location:       "uswest2",
			SubscriptionID: "subID",
			Tags: map[string]string{
				"env":      "prod",
				"location": "uswest1",
			},
			Properties: azure.AKSClusterProperties{},
		},
	},
	"group2": {
		{
			Name:           "cluster3",
			GroupName:      "group1",
			TenantID:       "tenantID",
			Location:       "uswest1",
			SubscriptionID: "subID",
			Tags: map[string]string{
				"env":      "stg",
				"location": "uswest1",
			},
			Properties: azure.AKSClusterProperties{},
		},
		{
			Name:           "cluster4",
			GroupName:      "group1",
			TenantID:       "tenantID",
			Location:       "uswest2",
			SubscriptionID: "subID",
			Tags: map[string]string{
				"env":      "stg",
				"location": "uswest1",
			},
			Properties: azure.AKSClusterProperties{},
		},
	},
}

func aksClustersToResources(t *testing.T, clusters ...*azure.AKSCluster) types.ResourcesWithLabels {
	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		kubeCluster, err := common.NewKubeClusterFromAzureAKS(cluster)
		require.NoError(t, err)
		require.True(t, kubeCluster.IsAzure())
		common.ApplyAKSNameSuffix(kubeCluster)
		kubeClusters = append(kubeClusters, kubeCluster)
	}
	return kubeClusters.AsResources()
}
