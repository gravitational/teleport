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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestPollAWSRDS(t *testing.T) {
	const (
		accountID = "12345678"
	)
	var (
		regions = []string{"eu-west-1"}
	)

	tests := []struct {
		name string
		want *Resources
	}{
		{
			name: "poll rds databases",
			want: &Resources{
				RDSDatabases: []*accessgraphv1alpha.AWSRDSDatabaseV1{
					{
						Arn:    "arn:us-west1:rds:instance1",
						Status: rds.DBProxyStatusAvailable,
						Name:   "db1",
						EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
							Engine:  rds.EngineFamilyMysql,
							Version: "v1.1",
						},
						CreatedAt: timestamppb.New(date),
						Tags: []*accessgraphv1alpha.AWSTag{
							{
								Key:   "tag",
								Value: wrapperspb.String("val"),
							},
						},
						Region:    "eu-west-1",
						IsCluster: false,
						AccountId: "12345678",
					},
					{
						Arn:    "arn:us-west1:rds:cluster1",
						Status: rds.DBProxyStatusAvailable,
						Name:   "cluster1",
						EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
							Engine:  rds.EngineFamilyMysql,
							Version: "v1.1",
						},
						CreatedAt: timestamppb.New(date),
						Tags: []*accessgraphv1alpha.AWSTag{
							{
								Key:   "tag",
								Value: wrapperspb.String("val"),
							},
						},
						Region:    "eu-west-1",
						IsCluster: true,
						AccountId: "12345678",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedClients := &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBInstances: dbInstances(),
					DBClusters:  dbClusters(),
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
			a := &awsFetcher{
				Config: Config{
					AccountID:    accountID,
					CloudClients: mockedClients,
					Regions:      regions,
					Integration:  accountID,
				},
			}
			result := &Resources{}
			execFunc := a.pollAWSRDSDatabases(context.Background(), result, collectErr)
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
			),
			)

		})
	}
}

func dbInstances() []*rds.DBInstance {
	return []*rds.DBInstance{
		{
			DBInstanceIdentifier: aws.String("db1"),
			DBInstanceArn:        aws.String("arn:us-west1:rds:instance1"),
			InstanceCreateTime:   aws.Time(date),
			Engine:               aws.String(rds.EngineFamilyMysql),
			DBInstanceStatus:     aws.String(rds.DBProxyStatusAvailable),
			EngineVersion:        aws.String("v1.1"),
			TagList: []*rds.Tag{
				{
					Key:   aws.String("tag"),
					Value: aws.String("val"),
				},
			},
		},
	}
}

func dbClusters() []*rds.DBCluster {
	return []*rds.DBCluster{
		{
			DBClusterIdentifier: aws.String("cluster1"),
			DBClusterArn:        aws.String("arn:us-west1:rds:cluster1"),
			ClusterCreateTime:   aws.Time(date),
			Engine:              aws.String(rds.EngineFamilyMysql),
			Status:              aws.String(rds.DBProxyStatusAvailable),
			EngineVersion:       aws.String("v1.1"),
			TagList: []*rds.Tag{
				{
					Key:   aws.String("tag"),
					Value: aws.String("val"),
				},
			},
		},
	}
}
