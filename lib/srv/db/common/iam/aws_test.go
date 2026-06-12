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

package iam

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

func TestGetAWSPolicyDocument(t *testing.T) {
	redshift, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-redshift",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
	})
	require.NoError(t, err)

	rds, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-rds",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "instance.abcdefghijklmnop.us-east-1.rds.amazonaws.com:5438",
		AWS: types.AWS{
			AccountID: "123456789012",
			RDS: types.RDS{
				ResourceID: "abcdef",
			},
		},
	})
	require.NoError(t, err)

	rdsProxy, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-rds-proxy",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "my-proxy.proxy-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
		AWS: types.AWS{
			AccountID: "123456789012",
			RDSProxy: types.RDSProxy{
				ResourceID: "qwerty",
			},
		},
	})
	require.NoError(t, err)

	elasticache, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-elasticache",
	}, types.DatabaseSpecV3{
		Protocol: "redis",
		URI:      "clustercfg.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			ElastiCache: types.ElastiCache{
				ReplicationGroupID: "some-group",
			},
		},
	})
	require.NoError(t, err)

	memorydb, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-memorydb",
	}, types.DatabaseSpecV3{
		Protocol: "redis",
		URI:      "clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			MemoryDB: types.MemoryDB{
				ClusterName:  "my-memorydb",
				TLSEnabled:   true,
				EndpointType: "cluster",
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		inputDatabase        types.Database
		expectPolicyDocument string
		expectPlaceholders   Placeholders
		expectError          bool
	}{
		{
			inputDatabase: redshift,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "redshift:GetClusterCredentials",
            "Resource": [
                "arn:aws:redshift:us-east-1:{account_id}:dbuser:redshift-cluster-1/*",
                "arn:aws:redshift:us-east-1:{account_id}:dbname:redshift-cluster-1/*",
                "arn:aws:redshift:us-east-1:{account_id}:dbgroup:redshift-cluster-1/*"
            ]
        }
    ]
}`,
			expectPlaceholders: Placeholders{"{account_id}"},
		},
		{
			inputDatabase: rds,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds-db:connect",
            "Resource": "arn:aws:rds-db:us-east-1:123456789012:dbuser:abcdef/*"
        }
    ]
}`,
		},
		{
			inputDatabase: rdsProxy,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds-db:connect",
            "Resource": "arn:aws:rds-db:us-west-1:123456789012:dbuser:qwerty/*"
        }
    ]
}`,
		},
		{
			inputDatabase: elasticache,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "elasticache:Connect",
            "Resource": [
                "arn:aws:elasticache:ca-central-1:123456789012:replicationgroup:some-group",
                "arn:aws:elasticache:ca-central-1:123456789012:user:*"
            ]
        }
    ]
}`,
		},
		{
			inputDatabase: memorydb,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "memorydb:Connect",
            "Resource": [
                "arn:aws:memorydb:us-east-1:123456789012:cluster/my-memorydb",
                "arn:aws:memorydb:us-east-1:123456789012:user/*"
            ]
        }
    ]
}`,
		},
	}

	for _, test := range tests {
		t.Run(test.inputDatabase.GetName(), func(t *testing.T) {
			policyDoc, placeholders, err := GetAWSPolicyDocument(test.inputDatabase)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectPlaceholders, placeholders)
			}

			readablePolicyDoc, err := GetReadableAWSPolicyDocument(test.inputDatabase)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectPolicyDocument, readablePolicyDoc)

				readablePolicyDocParsed, err := awslib.ParsePolicyDocument(readablePolicyDoc)
				require.NoError(t, err)
				require.Equal(t, policyDoc, readablePolicyDocParsed)
			}
		})
	}
}

func TestGetAWSPolicyDocumentForAssumedRole(t *testing.T) {
	redshift, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-redshift",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
		},
	})
	require.NoError(t, err)
	redshiftServerless, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-redshift-serverless",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "my-workgroup.123456789012.us-east-1.redshift-serverless.amazonaws.com:5439",
	})
	require.NoError(t, err)

	tests := []struct {
		inputDatabase        types.Database
		expectPolicyDocument string
		expectPlaceholders   Placeholders
	}{
		{
			inputDatabase: redshift,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "redshift:GetClusterCredentialsWithIAM",
            "Resource": "arn:aws:redshift:us-east-1:123456789012:dbname:redshift-cluster-1/*"
        }
    ]
}`,
		},
		{
			inputDatabase: redshiftServerless,
			expectPolicyDocument: `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "redshift-serverless:GetCredentials",
            "Resource": "arn:aws:redshift-serverless:us-east-1:123456789012:workgroup/{workgroup_id}"
        }
    ]
}`,
			expectPlaceholders: Placeholders{"{workgroup_id}"},
		},
	}

	for _, test := range tests {
		t.Run(test.inputDatabase.GetName(), func(t *testing.T) {
			policyDoc, placeholders, err := GetAWSPolicyDocumentForAssumedRole(test.inputDatabase)
			require.NoError(t, err)
			require.Equal(t, test.expectPlaceholders, placeholders)

			readablePolicyDoc, err := GetReadableAWSPolicyDocumentForAssumedRole(test.inputDatabase)
			require.NoError(t, err)
			require.Equal(t, test.expectPolicyDocument, readablePolicyDoc)

			readablePolicyDocParsed, err := awslib.ParsePolicyDocument(readablePolicyDoc)
			require.NoError(t, err)
			require.Equal(t, policyDoc, readablePolicyDocParsed)
		})
	}
}
