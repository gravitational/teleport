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

package cloud

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestAWSMetadata tests fetching AWS metadata for RDS and Redshift databases.
func TestAWSMetadata(t *testing.T) {
	// Configure RDS API mock.
	rds := &mocks.RDSMock{
		DBInstances: []*rds.DBInstance{
			// Standalone RDS instance.
			{
				DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:123456789012:db:postgres-rds"),
				DBInstanceIdentifier:             aws.String("postgres-rds"),
				DbiResourceId:                    aws.String("db-xyz"),
				IAMDatabaseAuthenticationEnabled: aws.Bool(true),
			},
			// Instance that is a part of an Aurora cluster.
			{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:postgres-aurora-1"),
				DBInstanceIdentifier: aws.String("postgres-aurora-1"),
				DBClusterIdentifier:  aws.String("postgres-aurora"),
			},
		},
		DBClusters: []*rds.DBCluster{
			// Aurora cluster.
			{
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:postgres-aurora"),
				DBClusterIdentifier: aws.String("postgres-aurora"),
				DbClusterResourceId: aws.String("cluster-xyz"),
			},
		},
		DBProxies: []*rds.DBProxy{
			{
				DBProxyArn:  aws.String("arn:aws:rds:us-east-1:123456789012:db-proxy:prx-resource-id"),
				DBProxyName: aws.String("rds-proxy"),
			},
		},
		DBProxyEndpoints: []*rds.DBProxyEndpoint{
			{
				DBProxyEndpointName: aws.String("rds-proxy-endpoint"),
				DBProxyName:         aws.String("rds-proxy"),
			},
		},
	}

	// Configure Redshift API mock.
	redshift := &mocks.RedshiftMock{
		Clusters: []*redshift.Cluster{
			{
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-west-1:123456789012:namespace:namespace-id"),
				ClusterIdentifier:   aws.String("redshift-cluster-1"),
			},
			{
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-2:210987654321:namespace:namespace-id"),
				ClusterIdentifier:   aws.String("redshift-cluster-2"),
			},
		},
	}

	// Configure ElastiCache API mock.
	elasticache := &mocks.ElastiCacheMock{
		ReplicationGroups: []*elasticache.ReplicationGroup{
			{
				ARN:                      aws.String("arn:aws:elasticache:us-west-1:123456789012:replicationgroup:my-redis"),
				ReplicationGroupId:       aws.String("my-redis"),
				ClusterEnabled:           aws.Bool(true),
				TransitEncryptionEnabled: aws.Bool(true),
				UserGroupIds:             []*string{aws.String("my-user-group")},
			},
		},
	}

	// Configure MemoryDB API mock.
	memorydb := &mocks.MemoryDBMock{
		Clusters: []*memorydb.Cluster{
			{
				ARN:        aws.String("arn:aws:memorydb:us-west-1:123456789012:cluster:my-cluster"),
				Name:       aws.String("my-cluster"),
				TLSEnabled: aws.Bool(true),
				ACLName:    aws.String("my-user-group"),
			},
		},
	}

	stsMock := &mocks.STSMock{}

	// Configure Redshift Serverless API mock.
	redshiftServerlessWorkgroup := mocks.RedshiftServerlessWorkgroup("my-workgroup", "us-west-1")
	redshiftServerlessEndpoint := mocks.RedshiftServerlessEndpointAccess(redshiftServerlessWorkgroup, "my-endpoint", "us-west-1")
	redshiftServerless := &mocks.RedshiftServerlessMock{
		Workgroups: []*redshiftserverless.Workgroup{redshiftServerlessWorkgroup},
		Endpoints:  []*redshiftserverless.EndpointAccess{redshiftServerlessEndpoint},
	}

	// Create metadata fetcher.
	metadata, err := NewMetadata(MetadataConfig{
		Clients: &cloud.TestCloudClients{
			RDS:                rds,
			Redshift:           redshift,
			ElastiCache:        elasticache,
			MemoryDB:           memorydb,
			RedshiftServerless: redshiftServerless,
			STS:                stsMock,
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name   string
		inAWS  types.AWS
		outAWS types.AWS
	}{
		{
			name: "RDS instance",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					InstanceID: "postgres-rds",
				},
			},
			outAWS: types.AWS{
				Region:        "us-west-1",
				AccountID:     "123456789012",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					InstanceID: "postgres-rds",
					ResourceID: "db-xyz",
					IAMAuth:    true,
				},
			},
		},
		{
			name: "Aurora cluster",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					InstanceID: "postgres-aurora",
				},
			},
			outAWS: types.AWS{
				Region:        "us-east-1",
				AccountID:     "123456789012",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					ClusterID:  "postgres-aurora",
					ResourceID: "cluster-xyz",
				},
			},
		},
		{
			name: "RDS instance, part of Aurora cluster",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					InstanceID: "postgres-aurora-1",
				},
			},
			outAWS: types.AWS{
				Region:        "us-east-1",
				AccountID:     "123456789012",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					ClusterID:  "postgres-aurora",
					ResourceID: "cluster-xyz",
				},
			},
		},
		{
			name: "Redshift cluster 1",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-1",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-1",
				},
			},
		},
		{
			name: "Redshift cluster 2",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-2",
				},
			},
			outAWS: types.AWS{
				AccountID:     "210987654321",
				Region:        "us-east-2",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-2",
				},
			},
		},
		{
			name: "ElastiCache",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				ElastiCache: types.ElastiCache{
					ReplicationGroupID: "my-redis",
					EndpointType:       "configuration",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				ElastiCache: types.ElastiCache{
					ReplicationGroupID:       "my-redis",
					UserGroupIDs:             []string{"my-user-group"},
					TransitEncryptionEnabled: true,
					EndpointType:             "configuration",
				},
			},
		},
		{
			name: "MemoryDB",
			inAWS: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				MemoryDB: types.MemoryDB{
					ClusterName:  "my-cluster",
					EndpointType: "cluster",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				MemoryDB: types.MemoryDB{
					ClusterName:  "my-cluster",
					ACLName:      "my-user-group",
					TLSEnabled:   true,
					EndpointType: "cluster",
				},
			},
		},
		{
			name: "RDS Proxy",
			inAWS: types.AWS{
				Region:        "us-east-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDSProxy: types.RDSProxy{
					Name: "rds-proxy",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-east-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDSProxy: types.RDSProxy{
					Name:       "rds-proxy",
					ResourceID: "prx-resource-id",
				},
			},
		},
		{
			name: "RDS Proxy custom endpoint",
			inAWS: types.AWS{
				Region:        "us-east-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDSProxy: types.RDSProxy{
					CustomEndpointName: "rds-proxy-endpoint",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-east-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDSProxy: types.RDSProxy{
					Name:               "rds-proxy",
					CustomEndpointName: "rds-proxy-endpoint",
					ResourceID:         "prx-resource-id",
				},
			},
		},
		{
			name: "Redshift Serverless workgroup",
			inAWS: types.AWS{
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RedshiftServerless: types.RedshiftServerless{
					WorkgroupName: "my-workgroup",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RedshiftServerless: types.RedshiftServerless{
					WorkgroupName: "my-workgroup",
					WorkgroupID:   "some-uuid-for-my-workgroup",
				},
			},
		},
		{
			name: "Redshift Serverless VPC endpoint",
			inAWS: types.AWS{
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RedshiftServerless: types.RedshiftServerless{
					EndpointName: "my-endpoint",
				},
			},
			outAWS: types.AWS{
				AccountID:     "123456789012",
				Region:        "us-west-1",
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RedshiftServerless: types.RedshiftServerless{
					WorkgroupName: "my-workgroup",
					EndpointName:  "my-endpoint",
					WorkgroupID:   "some-uuid-for-my-workgroup",
				},
			},
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: "test",
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost",
				AWS:      test.inAWS,
			})
			require.NoError(t, err)

			err = metadata.Update(ctx, database)
			require.NoError(t, err)
			require.Equal(t, test.outAWS, database.GetAWS())
			require.Equal(t, []string{test.inAWS.AssumeRoleARN}, stsMock.GetAssumedRoleARNs())
			require.Equal(t, []string{test.inAWS.ExternalID}, stsMock.GetAssumedRoleExternalIDs())
			stsMock.ResetAssumeRoleHistory()
		})
	}
}

// TestAWSMetadataNoPermissions verifies that lack of AWS permissions does not
// cause an error.
func TestAWSMetadataNoPermissions(t *testing.T) {
	// Create unauthorized mocks.
	rds := &mocks.RDSMockUnauth{}
	redshift := &mocks.RedshiftMockUnauth{}

	stsMock := &mocks.STSMock{}

	// Create metadata fetcher.
	metadata, err := NewMetadata(MetadataConfig{
		Clients: &cloud.TestCloudClients{
			RDS:      rds,
			Redshift: redshift,
			STS:      stsMock,
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		meta types.AWS
	}{
		{
			name: "RDS instance",
			meta: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDS: types.RDS{
					InstanceID: "postgres-rds",
				},
			},
		},
		{
			name: "RDS proxy",
			meta: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDSProxy: types.RDSProxy{
					Name: "rds-proxy",
				},
			},
		},
		{
			name: "RDS proxy endpoint",
			meta: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				RDSProxy: types.RDSProxy{
					CustomEndpointName: "rds-proxy-endpoint",
				},
			},
		},
		{
			name: "Redshift cluster",
			meta: types.AWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				ExternalID:    "externalID123",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-1",
				},
			},
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: "test",
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost",
				AWS:      test.meta,
			})
			require.NoError(t, err)

			// Verify there's no error and metadata stayed the same.
			err = metadata.Update(ctx, database)
			require.NoError(t, err)
			require.Equal(t, test.meta, database.GetAWS())
			require.Equal(t, []string{test.meta.AssumeRoleARN}, stsMock.GetAssumedRoleARNs())
			require.Equal(t, []string{test.meta.ExternalID}, stsMock.GetAssumedRoleExternalIDs())
			stsMock.ResetAssumeRoleHistory()
		})
	}
}
