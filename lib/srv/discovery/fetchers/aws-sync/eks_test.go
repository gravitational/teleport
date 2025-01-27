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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

var (
	date           = time.Date(2024, 03, 12, 0, 0, 0, 0, time.UTC)
	principalARN   = "arn:iam:teleport"
	accessEntryARN = "arn:iam:access_entry"
)

func TestPollAWSEKSClusters(t *testing.T) {
	const (
		accountID = "12345678"
	)
	var (
		regions = []string{"eu-west-1"}
	)
	cluster := &accessgraphv1alpha.AWSEKSClusterV1{
		Name:      "cluster1",
		Arn:       "arn:us-west1:eks:cluster1",
		CreatedAt: timestamppb.New(date),
		Status:    "ACTIVE",
		Tags: []*accessgraphv1alpha.AWSTag{
			{
				Key:   "tag1",
				Value: nil,
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
							Type:       eks.AccessScopeTypeCluster,
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
			mockedClients := &cloud.TestCloudClients{
				EKS: &mocks.EKSMock{
					Clusters:           eksClusters(),
					AccessEntries:      accessEntries(),
					AssociatedPolicies: associatedPolicies(),
				},
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
					CloudClients: mockedClients,
					Regions:      regions,
					Integration:  accountID,
				},
				lastResult: &Resources{},
			}
			result := &Resources{}
			execFunc := a.pollAWSEKSClusters(context.Background(), result, collectErr)
			require.NoError(t, execFunc())
			require.Empty(t, cmp.Diff(
				tt.want,
				result,
				protocmp.Transform(),
				// tags originate from a map so we must sort them before comparing.
				protocmp.SortRepeated(
					func(a, b *accessgraphv1alpha.AWSTag) bool {
						return a.Key < b.Key
					},
				),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSEKSClusterV1{}, "last_sync_time"),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1{}, "last_sync_time"),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSEKSClusterAccessEntryV1{}, "last_sync_time"),
			),
			)

		})
	}
}

func eksClusters() []*eks.Cluster {
	return []*eks.Cluster{
		{
			Name:      aws.String("cluster1"),
			Arn:       aws.String("arn:us-west1:eks:cluster1"),
			CreatedAt: aws.Time(date),
			Status:    aws.String(eks.AddonStatusActive),
			Tags: map[string]*string{
				"tag1": nil,
				"tag2": aws.String("val2"),
			},
		},
	}
}

func accessEntries() []*eks.AccessEntry {
	return []*eks.AccessEntry{
		{
			PrincipalArn:   aws.String(principalARN),
			AccessEntryArn: aws.String(accessEntryARN),
			CreatedAt:      aws.Time(date),
			ModifiedAt:     aws.Time(date),
			ClusterName:    aws.String("cluster1"),
			Tags: map[string]*string{
				"t1": aws.String("t2"),
			},
			Type:             aws.String(eks.AccessScopeTypeCluster),
			Username:         aws.String("teleport"),
			KubernetesGroups: []*string{aws.String("teleport")},
		},
	}
}

func associatedPolicies() []*eks.AssociatedAccessPolicy {
	return []*eks.AssociatedAccessPolicy{
		{
			AccessScope: &eks.AccessScope{
				Namespaces: []*string{aws.String("ns1")},
				Type:       aws.String(eks.AccessScopeTypeCluster),
			},
			ModifiedAt:   aws.Time(date),
			AssociatedAt: aws.Time(date),
			PolicyArn:    aws.String("policy_arn"),
		},
	}
}
