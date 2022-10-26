/*
Copyright 2021 Gravitational, Inc.

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

package cloud

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/stretchr/testify/require"
)

// TestAWSMetadata tests fetching AWS metadata for RDS and Redshift databases.
func TestAWSMetadata(t *testing.T) {
	// Configure RDS API mock.
	rds := &RDSMock{
		DBInstances: []*rds.DBInstance{
			// Standalone RDS instance.
			{
				DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:1234567890:db:postgres-rds"),
				DBInstanceIdentifier:             aws.String("postgres-rds"),
				DbiResourceId:                    aws.String("db-xyz"),
				IAMDatabaseAuthenticationEnabled: aws.Bool(true),
			},
			// Instance that is a part of an Aurora cluster.
			{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:1234567890:db:postgres-aurora-1"),
				DBInstanceIdentifier: aws.String("postgres-aurora-1"),
				DBClusterIdentifier:  aws.String("postgres-aurora"),
			},
		},
		DBClusters: []*rds.DBCluster{
			// Aurora cluster.
			{
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:1234567890:cluster:postgres-aurora"),
				DBClusterIdentifier: aws.String("postgres-aurora"),
				DbClusterResourceId: aws.String("cluster-xyz"),
			},
		},
	}

	// Configure Redshift API mock.
	redshift := &RedshiftMock{
		Clusters: []*redshift.Cluster{
			{
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-west-1:1234567890:namespace:namespace-id"),
				ClusterIdentifier:   aws.String("redshift-cluster-1"),
			},
			{
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-2:0987654321:namespace:namespace-id"),
				ClusterIdentifier:   aws.String("redshift-cluster-2"),
			},
		},
	}

	// Configure ElastiCache API mock.
	elasticache := &ElastiCacheMock{
		ReplicationGroups: []*elasticache.ReplicationGroup{
			{
				ARN:                      aws.String("arn:aws:elasticache:us-west-1:123456789:replicationgroup:my-redis"),
				ReplicationGroupId:       aws.String("my-redis"),
				ClusterEnabled:           aws.Bool(true),
				TransitEncryptionEnabled: aws.Bool(true),
				UserGroupIds:             []*string{aws.String("my-user-group")},
			},
		},
	}

	// Configure MemoryDB API mock.
	memorydb := &MemoryDBMock{
		Clusters: []*memorydb.Cluster{
			{
				ARN:        aws.String("arn:aws:memorydb:us-west-1:123456789:cluster:my-cluster"),
				Name:       aws.String("my-cluster"),
				TLSEnabled: aws.Bool(true),
				ACLName:    aws.String("my-user-group"),
			},
		},
	}

	// Create metadata fetcher.
	metadata, err := NewMetadata(MetadataConfig{
		Clients: &cloud.TestCloudClients{
			RDS:         rds,
			Redshift:    redshift,
			ElastiCache: elasticache,
			MemoryDB:    memorydb,
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
				RDS: types.RDS{
					InstanceID: "postgres-rds",
				},
			},
			outAWS: types.AWS{
				Region:    "us-west-1",
				AccountID: "1234567890",
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
				RDS: types.RDS{
					InstanceID: "postgres-aurora",
				},
			},
			outAWS: types.AWS{
				Region:    "us-east-1",
				AccountID: "1234567890",
				RDS: types.RDS{
					ClusterID:  "postgres-aurora",
					ResourceID: "cluster-xyz",
				},
			},
		},
		{
			name: "RDS instance, part of Aurora cluster",
			inAWS: types.AWS{
				RDS: types.RDS{
					InstanceID: "postgres-aurora-1",
				},
			},
			outAWS: types.AWS{
				Region:    "us-east-1",
				AccountID: "1234567890",
				RDS: types.RDS{
					ClusterID:  "postgres-aurora",
					ResourceID: "cluster-xyz",
				},
			},
		},
		{
			name: "Redshift cluster 1",
			inAWS: types.AWS{
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-1",
				},
			},
			outAWS: types.AWS{
				AccountID: "1234567890",
				Region:    "us-west-1",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-1",
				},
			},
		},
		{
			name: "Redshift cluster 2",
			inAWS: types.AWS{
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-2",
				},
			},
			outAWS: types.AWS{
				AccountID: "0987654321",
				Region:    "us-east-2",
				Redshift: types.Redshift{
					ClusterID: "redshift-cluster-2",
				},
			},
		},
		{
			name: "ElastiCache",
			inAWS: types.AWS{
				ElastiCache: types.ElastiCache{
					ReplicationGroupID: "my-redis",
					EndpointType:       "configuration",
				},
			},
			outAWS: types.AWS{
				AccountID: "123456789",
				Region:    "us-west-1",
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
				MemoryDB: types.MemoryDB{
					ClusterName:  "my-cluster",
					EndpointType: "cluster",
				},
			},
			outAWS: types.AWS{
				AccountID: "123456789",
				Region:    "us-west-1",
				MemoryDB: types.MemoryDB{
					ClusterName:  "my-cluster",
					ACLName:      "my-user-group",
					TLSEnabled:   true,
					EndpointType: "cluster",
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
		})
	}
}

// TestAWSMetadataNoPermissions verifies that lack of AWS permissions does not
// cause an error.
func TestAWSMetadataNoPermissions(t *testing.T) {
	// Create unauthorized mocks.
	rds := &RDSMockUnauth{}
	redshift := &RedshiftMockUnauth{}

	// Create metadata fetcher.
	metadata, err := NewMetadata(MetadataConfig{
		Clients: &cloud.TestCloudClients{
			RDS:      rds,
			Redshift: redshift,
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
				RDS: types.RDS{
					InstanceID: "postgres-rds",
				},
			},
		},
		{
			name: "Redshift cluster",
			meta: types.AWS{
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
		})
	}
}
