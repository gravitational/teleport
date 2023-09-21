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
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestEKSFetcher(t *testing.T) {
	type args struct {
		region       string
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
				region: types.Wildcard,
				filterLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
			},
			want: eksClustersToResources(t, eksMockClusters...),
		},
		{
			name: "list prod clusters",
			args: args{
				region: types.Wildcard,
				filterLabels: types.Labels{
					"env": []string{"prod"},
				},
			},
			want: eksClustersToResources(t, eksMockClusters[:2]...),
		},
		{
			name: "list stg clusters from eu-west-1",
			args: args{
				region: "uswest2",
				filterLabels: types.Labels{
					"env":      []string{"stg"},
					"location": []string{"eu-west-1"},
				},
			},
			want: eksClustersToResources(t, eksMockClusters[2:]...),
		},
		{
			name: "filter not found",
			args: args{
				region: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"none"},
				},
			},
			want: eksClustersToResources(t),
		},

		{
			name: "list everything with specified values",
			args: args{
				region: "uswest2",
				filterLabels: types.Labels{
					"env": []string{"prod", "stg"},
				},
			},
			want: eksClustersToResources(t, eksMockClusters...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := EKSFetcherConfig{
				Client:       newPopulatedEKSMock(),
				FilterLabels: tt.args.filterLabels,
				Region:       tt.args.region,
				Log:          logrus.New(),
			}
			fetcher, err := NewEKSFetcher(cfg)
			require.NoError(t, err)
			resources, err := fetcher.Get(context.Background())
			require.NoError(t, err)

			require.Equal(t, tt.want.ToMap(), resources.ToMap())
		})
	}
}

type mockEKSAPI struct {
	eksiface.EKSAPI
	clusters []*eks.Cluster
}

func (m *mockEKSAPI) ListClustersPagesWithContext(ctx aws.Context, req *eks.ListClustersInput, f func(*eks.ListClustersOutput, bool) bool, _ ...request.Option) error {
	var names []*string
	for _, cluster := range m.clusters {
		names = append(names, cluster.Name)
	}
	f(&eks.ListClustersOutput{
		Clusters: names[:len(names)/2],
	}, false)

	f(&eks.ListClustersOutput{
		Clusters: names[len(names)/2:],
	}, true)
	return nil
}

func (m *mockEKSAPI) DescribeClusterWithContext(_ aws.Context, req *eks.DescribeClusterInput, _ ...request.Option) (*eks.DescribeClusterOutput, error) {
	for _, cluster := range m.clusters {
		if aws.StringValue(cluster.Name) == aws.StringValue(req.Name) {
			return &eks.DescribeClusterOutput{
				Cluster: cluster,
			}, nil
		}
	}
	return nil, errors.New("cluster not found")
}

func newPopulatedEKSMock() *mockEKSAPI {
	return &mockEKSAPI{
		clusters: eksMockClusters,
	}
}

var eksMockClusters = []*eks.Cluster{

	{
		Name:   aws.String("cluster1"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
		Status: aws.String(eks.ClusterStatusActive),
		Tags: map[string]*string{
			"env":      aws.String("prod"),
			"location": aws.String("eu-west-1"),
		},
	},
	{
		Name:   aws.String("cluster2"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster2"),
		Status: aws.String(eks.ClusterStatusActive),
		Tags: map[string]*string{
			"env":      aws.String("prod"),
			"location": aws.String("eu-west-1"),
		},
	},

	{
		Name:   aws.String("cluster3"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster3"),
		Status: aws.String(eks.ClusterStatusActive),
		Tags: map[string]*string{
			"env":      aws.String("stg"),
			"location": aws.String("eu-west-1"),
		},
	},
	{
		Name:   aws.String("cluster4"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
		Status: aws.String(eks.ClusterStatusActive),
		Tags: map[string]*string{
			"env":      aws.String("stg"),
			"location": aws.String("eu-west-1"),
		},
	},
}

func eksClustersToResources(t *testing.T, clusters ...*eks.Cluster) types.ResourcesWithLabels {
	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		kubeCluster, err := services.NewKubeClusterFromAWSEKS(cluster)
		require.NoError(t, err)
		require.True(t, kubeCluster.IsAWS())
		common.ApplyEKSNameSuffix(kubeCluster)
		kubeClusters = append(kubeClusters, kubeCluster)
	}
	return kubeClusters.AsResources()
}
