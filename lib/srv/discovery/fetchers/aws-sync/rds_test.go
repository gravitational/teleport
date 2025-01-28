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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/gravitational/teleport/api/types"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestPollAWSRDS(t *testing.T) {
	const (
		accountID = "12345678"
	)
	var (
		regions = []string{"eu-west-1"}
	)

	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "integration-test"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:sts::123456789012:role/TestRole",
		},
	)
	require.NoError(t, err)

	resourcesFixture := Resources{
		RDSDatabases: []*accessgraphv1alpha.AWSRDSDatabaseV1{
			{
				Arn:    "arn:us-west1:rds:instance1",
				Status: string(rdstypes.DBProxyStatusAvailable),
				Name:   "db1",
				EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
					Engine:  string(rdstypes.EngineFamilyMysql),
					Version: "v1.1",
				},
				CreatedAt: timestamppb.New(date),
				Tags: []*accessgraphv1alpha.AWSTag{
					{
						Key:   "tag",
						Value: wrapperspb.String("val"),
					},
				},
				Region:     "eu-west-1",
				IsCluster:  false,
				AccountId:  "12345678",
				ResourceId: "db1",
			},
			{
				Arn:    "arn:us-west1:rds:cluster1",
				Status: string(rdstypes.DBProxyStatusAvailable),
				Name:   "cluster1",
				EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
					Engine:  string(rdstypes.EngineFamilyMysql),
					Version: "v1.1",
				},
				CreatedAt: timestamppb.New(date),
				Tags: []*accessgraphv1alpha.AWSTag{
					{
						Key:   "tag",
						Value: wrapperspb.String("val"),
					},
				},
				Region:     "eu-west-1",
				IsCluster:  true,
				AccountId:  "12345678",
				ResourceId: "cluster1",
			},
		},
	}

	tests := []struct {
		name             string
		fetcherConfigOpt func(*Fetcher)
		want             *Resources
		checkError       func(*testing.T, error)
	}{
		{
			name: "poll rds databases",
			want: &resourcesFixture,
			fetcherConfigOpt: func(a *Fetcher) {
				a.awsClients = fakeAWSClients{
					rdsClient: &mocks.RDSClient{
						DBInstances: dbInstances(),
						DBClusters:  dbClusters(),
					},
				}
			},
			checkError: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "reuse last synced databases on failure",
			want: &resourcesFixture,
			fetcherConfigOpt: func(a *Fetcher) {
				a.awsClients = fakeAWSClients{
					rdsClient: &mocks.RDSClient{Unauth: true},
				}
				a.lastResult = &resourcesFixture
			},
			checkError: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "failed to fetch databases")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Fetcher{
				Config: Config{
					AccountID: accountID,
					AWSConfigProvider: &mocks.AWSConfigProvider{
						OIDCIntegrationClient: &mocks.FakeOIDCIntegrationClient{
							Integration: awsOIDCIntegration,
							Token:       "fake-oidc-token",
						},
					},
					Regions:     regions,
					Integration: awsOIDCIntegration.GetName(),
				},
			}
			if tt.fetcherConfigOpt != nil {
				tt.fetcherConfigOpt(a)
			}
			result := &Resources{}
			collectErr := func(err error) {
				tt.checkError(t, err)
			}
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
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSRDSDatabaseV1{}, "last_sync_time"),
			),
			)

		})
	}
}

func dbInstances() []rdstypes.DBInstance {
	return []rdstypes.DBInstance{
		{
			DBInstanceIdentifier: aws.String("db1"),
			DBInstanceArn:        aws.String("arn:us-west1:rds:instance1"),
			InstanceCreateTime:   aws.Time(date),
			Engine:               aws.String(string(rdstypes.EngineFamilyMysql)),
			DBInstanceStatus:     aws.String(string(rdstypes.DBProxyStatusAvailable)),
			EngineVersion:        aws.String("v1.1"),
			TagList: []rdstypes.Tag{
				{
					Key:   aws.String("tag"),
					Value: aws.String("val"),
				},
			},
			DbiResourceId: aws.String("db1"),
		},
	}
}

func dbClusters() []rdstypes.DBCluster {
	return []rdstypes.DBCluster{
		{
			DBClusterIdentifier: aws.String("cluster1"),
			DBClusterArn:        aws.String("arn:us-west1:rds:cluster1"),
			ClusterCreateTime:   aws.Time(date),
			Engine:              aws.String(string(rdstypes.EngineFamilyMysql)),
			Status:              aws.String(string(rdstypes.DBProxyStatusAvailable)),
			EngineVersion:       aws.String("v1.1"),
			TagList: []rdstypes.Tag{
				{
					Key:   aws.String("tag"),
					Value: aws.String("val"),
				},
			},
			DbClusterResourceId: aws.String("cluster1"),
		},
	}
}

type fakeAWSClients struct {
	iamClient iamClient
	rdsClient rdsClient
	s3Client  s3Client
}

func (f fakeAWSClients) getIAMClient(cfg aws.Config, optFns ...func(*iam.Options)) iamClient {
	return f.iamClient
}

func (f fakeAWSClients) getRDSClient(cfg aws.Config, optFns ...func(*rds.Options)) rdsClient {
	return f.rdsClient
}

func (f fakeAWSClients) getS3Client(cfg aws.Config, optFns ...func(*s3.Options)) s3Client {
	return f.s3Client
}
