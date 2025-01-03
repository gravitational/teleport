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
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
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
				ClientGetter: &mockEKSClientGetter{},
				FilterLabels: tt.args.filterLabels,
				Region:       tt.args.region,
				Logger:       utils.NewSlogLoggerForTests(),
			}
			fetcher, err := NewEKSFetcher(cfg)
			require.NoError(t, err)
			resources, err := fetcher.Get(context.Background())
			require.NoError(t, err)

			clusters := types.ResourcesWithLabels{}
			for _, r := range resources {
				if e, ok := r.(*DiscoveredEKSCluster); ok {
					clusters = append(clusters, e.GetKubeCluster())
				} else {
					clusters = append(clusters, r)
				}
			}

			require.Equal(t, tt.want.ToMap(), clusters.ToMap())
		})
	}
}

type mockEKSClientGetter struct{}

func (e *mockEKSClientGetter) GetAWSEKSClient(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (EKSClient, error) {
	return newPopulatedEKSMock(), nil
}

func (e *mockEKSClientGetter) GetAWSSTSClient(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (STSClient, error) {
	return &mockSTSAPI{}, nil
}

func (e *mockEKSClientGetter) GetAWSSTSPresignClient(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (kubeutils.STSPresignClient, error) {
	return &mockSTSPresignAPI{}, nil
}

type mockSTSPresignAPI struct{}

func (a *mockSTSPresignAPI) PresignGetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	panic("not implemented")
}

type mockSTSAPI struct {
	arn string
}

func (a *mockSTSAPI) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(a.arn),
	}, nil
}

func (a *mockSTSAPI) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	panic("not implemented")
}

type mockEKSAPI struct {
	EKSClient

	clusters []*ekstypes.Cluster
}

func (m *mockEKSAPI) ListClusters(ctx context.Context, req *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	var names []string
	for _, cluster := range m.clusters {
		names = append(names, aws.ToString(cluster.Name))
	}
	return &eks.ListClustersOutput{
		Clusters: names,
	}, nil
}

func (m *mockEKSAPI) DescribeCluster(_ context.Context, req *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	for _, cluster := range m.clusters {
		if aws.ToString(cluster.Name) == aws.ToString(req.Name) {
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

var eksMockClusters = []*ekstypes.Cluster{
	{
		Name:   aws.String("cluster1"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "prod",
			"location": "eu-west-1",
		},
	},
	{
		Name:   aws.String("cluster2"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster2"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "prod",
			"location": "eu-west-1",
		},
	},

	{
		Name:   aws.String("cluster3"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster3"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "stg",
			"location": "eu-west-1",
		},
	},
	{
		Name:   aws.String("cluster4"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "stg",
			"location": "eu-west-1",
		},
	},
}

func eksClustersToResources(t *testing.T, clusters ...*ekstypes.Cluster) types.ResourcesWithLabels {
	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		kubeCluster, err := common.NewKubeClusterFromAWSEKS(aws.ToString(cluster.Name), aws.ToString(cluster.Arn), cluster.Tags)
		require.NoError(t, err)
		require.True(t, kubeCluster.IsAWS())
		common.ApplyEKSNameSuffix(kubeCluster)
		kubeClusters = append(kubeClusters, kubeCluster)
	}
	return kubeClusters.AsResources()
}
