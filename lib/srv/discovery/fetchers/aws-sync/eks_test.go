/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

var date = time.Date(2024, 0o3, 12, 0, 0, 0, 0, time.UTC)

const (
	principalARN   = "arn:iam:teleport"
	accessEntryARN = "arn:iam:access_entry"
)

type mockedEKSClient struct {
	clusters                 []*ekstypes.Cluster
	accessEntries            []*ekstypes.AccessEntry
	associatedAccessPolicies []ekstypes.AssociatedAccessPolicy
}

func (m *mockedEKSClient) DescribeCluster(ctx context.Context, input *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	for _, cluster := range m.clusters {
		if aws.ToString(cluster.Name) == aws.ToString(input.Name) {
			return &eks.DescribeClusterOutput{
				Cluster: cluster,
			}, nil
		}
	}
	return nil, nil
}

func (m *mockedEKSClient) ListClusters(ctx context.Context, input *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	clusterNames := make([]string, 0, len(m.clusters))
	for _, cluster := range m.clusters {
		clusterNames = append(clusterNames, aws.ToString(cluster.Name))
	}
	return &eks.ListClustersOutput{
		Clusters: clusterNames,
	}, nil
}

func (m *mockedEKSClient) ListAccessEntries(ctx context.Context, input *eks.ListAccessEntriesInput, optFns ...func(*eks.Options)) (*eks.ListAccessEntriesOutput, error) {
	accessEntries := make([]string, 0, len(m.accessEntries))
	for _, accessEntry := range m.accessEntries {
		accessEntries = append(accessEntries, aws.ToString(accessEntry.AccessEntryArn))
	}
	return &eks.ListAccessEntriesOutput{
		AccessEntries: accessEntries,
	}, nil
}

func (m *mockedEKSClient) ListAssociatedAccessPolicies(ctx context.Context, input *eks.ListAssociatedAccessPoliciesInput, optFns ...func(*eks.Options)) (*eks.ListAssociatedAccessPoliciesOutput, error) {
	return &eks.ListAssociatedAccessPoliciesOutput{
		AssociatedAccessPolicies: m.associatedAccessPolicies,
	}, nil
}

func (m *mockedEKSClient) DescribeAccessEntry(ctx context.Context, input *eks.DescribeAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DescribeAccessEntryOutput, error) {
	return &eks.DescribeAccessEntryOutput{
		AccessEntry: &ekstypes.AccessEntry{
			PrincipalArn:   aws.String(principalARN),
			AccessEntryArn: aws.String(accessEntryARN),
			CreatedAt:      aws.Time(date),
			ModifiedAt:     aws.Time(date),
			ClusterName:    aws.String("cluster1"),
			Tags: map[string]string{
				"t1": "t2",
			},
			Type:             aws.String(string(ekstypes.AccessScopeTypeCluster)),
			Username:         aws.String("teleport"),
			KubernetesGroups: []string{"teleport"},
		},
	}, nil
}

func TestPollAWSEKSClusters(t *testing.T) {
	const (
		accountID = "12345678"
	)
	regions := []string{"eu-west-1"}
	cluster := &accessgraphv1alpha.AWSEKSClusterV1{
		Name:      "cluster1",
		Arn:       "arn:us-west1:eks:cluster1",
		CreatedAt: timestamppb.New(date),
		Status:    "ACTIVE",
		Tags: []*accessgraphv1alpha.AWSTag{
			{
				Key:   "tag1",
				Value: wrapperspb.String(""),
			},
			{
				Key:   "tag2",
				Value: wrapperspb.String("val2"),
			},
		},
		Region:    "eu-west-1",
		AccountId: "12345678",
	}
	tests := []struct {
		name string
		want *Resources
	}{
		{
			name: "poll eks clusters",
			want: &Resources{
				EKSClusters: []*accessgraphv1alpha.AWSEKSClusterV1{
					cluster,
				},
				AccessEntries: []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1{
					{
						Cluster:          cluster,
						AccessEntryArn:   "arn:iam:access_entry",
						CreatedAt:        timestamppb.New(date),
						KubernetesGroups: []string{"teleport"},
						ModifiedAt:       timestamppb.New(date),
						PrincipalArn:     "arn:iam:teleport",
						Tags: []*accessgraphv1alpha.AWSTag{
							{
								Key:   "t1",
								Value: wrapperspb.String("t2"),
							},
						},
						Type:      "cluster",
						Username:  "teleport",
						AccountId: "12345678",
					},
				},
				AssociatedAccessPolicies: []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1{
					{
						Cluster:      cluster,
						PrincipalArn: principalARN,
						Scope: &accessgraphv1alpha.AWSEKSAccessScopeV1{
							Type:       string(ekstypes.AccessScopeTypeCluster),
							Namespaces: []string{"ns1"},
						},
						AssociatedAt: timestamppb.New(date),
						ModifiedAt:   timestamppb.New(date),
						PolicyArn:    "policy_arn",
						AccountId:    "12345678",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getEKSClient := func(_ context.Context, _ string, _ ...awsconfig.OptionsFn) (EKSClient, error) {
				return &mockedEKSClient{
					clusters:                 eksClusters(),
					accessEntries:            accessEntries(),
					associatedAccessPolicies: associatedPolicies(),
				}, nil
			}

			var (
				errs []error
				mu   sync.Mutex
			)

			collectErr := func(err error) {
				mu.Lock()
				defer mu.Unlock()
				errs = append(errs, err)
			}
			a := &Fetcher{
				Config: Config{
					AccountID:    accountID,
					Regions:      regions,
					Integration:  accountID,
					GetEKSClient: getEKSClient,
				},
				lastResult: &Resources{},
			}

			var result Resources
			execFunc := a.pollAWSEKSClusters(context.Background(), &result, collectErr)
			require.NoError(t, execFunc())
			require.Empty(t, cmp.Diff(
				tt.want,
				&result,
				protocmp.Transform(),
				// Tags originate from a map so we must sort them before comparing.
				protocmp.SortRepeated(
					func(a, b *accessgraphv1alpha.AWSTag) bool {
						return a.Key < b.Key
					},
				),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSEKSClusterV1{}, "last_sync_time"),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1{}, "last_sync_time"),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSEKSClusterAccessEntryV1{}, "last_sync_time"),
			))
		})
	}
}

func eksClusters() []*ekstypes.Cluster {
	return []*ekstypes.Cluster{
		{
			Name:      aws.String("cluster1"),
			Arn:       aws.String("arn:us-west1:eks:cluster1"),
			CreatedAt: aws.Time(date),
			Status:    ekstypes.ClusterStatusActive,
			Tags: map[string]string{
				"tag1": "",
				"tag2": "val2",
			},
		},
	}
}

func accessEntries() []*ekstypes.AccessEntry {
	return []*ekstypes.AccessEntry{
		{
			PrincipalArn:   aws.String(principalARN),
			AccessEntryArn: aws.String(accessEntryARN),
			CreatedAt:      aws.Time(date),
			ModifiedAt:     aws.Time(date),
			ClusterName:    aws.String("cluster1"),
			Tags: map[string]string{
				"t1": "t2",
			},
			Type:             aws.String(string(ekstypes.AccessScopeTypeCluster)),
			Username:         aws.String("teleport"),
			KubernetesGroups: []string{"teleport"},
		},
	}
}

func associatedPolicies() []ekstypes.AssociatedAccessPolicy {
	return []ekstypes.AssociatedAccessPolicy{
		{
			AccessScope: &ekstypes.AccessScope{
				Namespaces: []string{"ns1"},
				Type:       ekstypes.AccessScopeTypeCluster,
			},
			ModifiedAt:   aws.Time(date),
			AssociatedAt: aws.Time(date),
			PolicyArn:    aws.String("policy_arn"),
		},
	}
}
