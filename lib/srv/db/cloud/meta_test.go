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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	opensearch "github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestAWSMetadata tests fetching AWS metadata for RDS and Redshift databases.
func TestAWSMetadata(t *testing.T) {
	// Configure RDS API mock.
	rdsClt := &mocks.RDSClient{
		DBInstances: []rdstypes.DBInstance{
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
		DBClusters: []rdstypes.DBCluster{
			// Aurora cluster.
			{
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:postgres-aurora"),
				DBClusterIdentifier: aws.String("postgres-aurora"),
				DbClusterResourceId: aws.String("cluster-xyz"),
			},
		},
		DBProxies: []rdstypes.DBProxy{
			{
				DBProxyArn:  aws.String("arn:aws:rds:us-east-1:123456789012:db-proxy:prx-resource-id"),
				DBProxyName: aws.String("rds-proxy"),
			},
		},
		DBProxyEndpoints: []rdstypes.DBProxyEndpoint{
			{
				DBProxyEndpointName: aws.String("rds-proxy-endpoint"),
				DBProxyName:         aws.String("rds-proxy"),
				Endpoint:            aws.String("localhost"),
			},
		},
	}

	// Configure Redshift API mock.
	redshiftClt := &mocks.RedshiftClient{
		Clusters: []redshifttypes.Cluster{
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
	ecClient := &mocks.ElastiCacheClient{
		ReplicationGroups: []ectypes.ReplicationGroup{
			{
				ARN:                      aws.String("arn:aws:elasticache:us-west-1:123456789012:replicationgroup:my-redis"),
				ReplicationGroupId:       aws.String("my-redis"),
				ClusterEnabled:           aws.Bool(true),
				TransitEncryptionEnabled: aws.Bool(true),
				UserGroupIds:             []string{"my-user-group"},
			},
		},
	}

	// Configure MemoryDB API mock.
	mdbClient := &mocks.MemoryDBClient{
		Clusters: []memorydbtypes.Cluster{
			{
				ARN:        aws.String("arn:aws:memorydb:us-west-1:123456789012:cluster:my-cluster"),
				Name:       aws.String("my-cluster"),
				TLSEnabled: aws.Bool(true),
				ACLName:    aws.String("my-user-group"),
			},
		},
	}

	fakeSTS := &mocks.STSClient{}

	// Configure Redshift Serverless API mock.
	redshiftServerlessWorkgroup := mocks.RedshiftServerlessWorkgroup("my-workgroup", "us-west-1")
	redshiftServerlessEndpoint := mocks.RedshiftServerlessEndpointAccess(redshiftServerlessWorkgroup, "my-endpoint", "us-west-1")
	redshiftServerless := &mocks.RedshiftServerlessClient{
		Workgroups: []rsstypes.Workgroup{*redshiftServerlessWorkgroup},
		Endpoints:  []rsstypes.EndpointAccess{*redshiftServerlessEndpoint},
	}

	// Create metadata fetcher.
	metadata, err := NewMetadata(MetadataConfig{
		Clients: &cloud.TestCloudClients{
			STS: &fakeSTS.STSClientV1,
		},
		AWSConfigProvider: &mocks.AWSConfigProvider{
			STSClient: fakeSTS,
		},
		awsClients: fakeAWSClients{
			mdbClient:      mdbClient,
			ecClient:       ecClient,
			rdsClient:      rdsClt,
			redshiftClient: redshiftClt,
			rssClient:      redshiftServerless,
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
			require.Equal(t, []string{test.inAWS.AssumeRoleARN}, fakeSTS.GetAssumedRoleARNs())
			require.Equal(t, []string{test.inAWS.ExternalID}, fakeSTS.GetAssumedRoleExternalIDs())
			fakeSTS.ResetAssumeRoleHistory()
		})
	}
}

// TestAWSMetadataNoPermissions verifies that lack of AWS permissions does not
// cause an error.
func TestAWSMetadataNoPermissions(t *testing.T) {
	fakeSTS := &mocks.STSClient{}

	// Create metadata fetcher.
	metadata, err := NewMetadata(MetadataConfig{
		Clients: &cloud.TestCloudClients{
			STS: &fakeSTS.STSClientV1,
		},
		AWSConfigProvider: &mocks.AWSConfigProvider{
			STSClient: fakeSTS,
		},
		awsClients: fakeAWSClients{
			ecClient:       &mocks.ElastiCacheClient{Unauth: true},
			mdbClient:      &mocks.MemoryDBClient{Unauth: true},
			rdsClient:      &mocks.RDSClient{Unauth: true},
			redshiftClient: &mocks.RedshiftClient{Unauth: true},
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
			require.Equal(t, []string{test.meta.AssumeRoleARN}, fakeSTS.GetAssumedRoleARNs())
			require.Equal(t, []string{test.meta.ExternalID}, fakeSTS.GetAssumedRoleExternalIDs())
			fakeSTS.ResetAssumeRoleHistory()
		})
	}
}

type fakeAWSClients struct {
	ecClient         elasticacheClient
	iamClient        iamClient
	mdbClient        memoryDBClient
	openSearchClient openSearchClient
	rdsClient        rdsClient
	redshiftClient   redshiftClient
	rssClient        rssClient
	stsClient        stsClient
}

func (f fakeAWSClients) getElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) elasticacheClient {
	return f.ecClient
}

func (f fakeAWSClients) getIAMClient(aws.Config, ...func(*iam.Options)) iamClient {
	return f.iamClient
}

func (f fakeAWSClients) getMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) memoryDBClient {
	return f.mdbClient
}

func (f fakeAWSClients) getOpenSearchClient(cfg aws.Config, optFns ...func(*opensearch.Options)) openSearchClient {
	return f.openSearchClient
}

func (f fakeAWSClients) getRDSClient(aws.Config, ...func(*rds.Options)) rdsClient {
	return f.rdsClient
}

func (f fakeAWSClients) getRedshiftClient(aws.Config, ...func(*redshift.Options)) redshiftClient {
	return f.redshiftClient
}

func (f fakeAWSClients) getRedshiftServerlessClient(aws.Config, ...func(*rss.Options)) rssClient {
	return f.rssClient
}

func (f fakeAWSClients) getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient {
	return f.stsClient
}
