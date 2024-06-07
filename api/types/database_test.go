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

package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	gcputils "github.com/gravitational/teleport/api/utils/gcp"
)

// TestDatabaseRDSEndpoint verifies AWS info is correctly populated
// based on the RDS endpoint.
func TestDatabaseRDSEndpoint(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name                 string
		labels               map[string]string
		spec                 DatabaseSpecV3
		errorCheck           require.ErrorAssertionFunc
		expectedAWS          AWS
		expectedEndpointType string
	}{
		{
			name: "aurora instance",
			spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			},
			errorCheck: require.NoError,
			expectedAWS: AWS{
				Region: "us-west-1",
				RDS: RDS{
					InstanceID: "aurora-instance-1",
				},
			},
			expectedEndpointType: "instance",
		},
		{
			name: "invalid account id",
			spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "marcotest-db001.abcdefghijklmnop.us-east-1.rds.amazonaws.com:5432",
				AWS: AWS{
					AccountID: "invalid",
				},
			},
			errorCheck: isBadParamErrFn,
		},
		{
			name: "valid account id",
			spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "marcotest-db001.cluster-ro-abcdefghijklmnop.us-east-1.rds.amazonaws.com:5432",
				AWS: AWS{
					AccountID: "123456789012",
				},
			},
			errorCheck: require.NoError,
			expectedAWS: AWS{
				Region: "us-east-1",
				RDS: RDS{
					ClusterID: "marcotest-db001",
				},
				AccountID: "123456789012",
			},
			expectedEndpointType: "reader",
		},
		{
			name: "discovered instance",
			labels: map[string]string{
				"account-id":                        "123456789012",
				"endpoint-type":                     "primary",
				"engine":                            "aurora-postgresql",
				"engine-version":                    "15.2",
				"region":                            "us-west-1",
				"teleport.dev/cloud":                "AWS",
				"teleport.dev/origin":               "cloud",
				"teleport.internal/discovered-name": "rds",
			},
			spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "discovered.rds.com:5432",
				AWS: AWS{
					Region: "us-west-1",
					RDS: RDS{
						InstanceID: "aurora-instance-1",
						IAMAuth:    true,
					},
				},
			},
			errorCheck: require.NoError,
			expectedAWS: AWS{
				Region: "us-west-1",
				RDS: RDS{
					InstanceID: "aurora-instance-1",
					IAMAuth:    true,
				},
			},
			expectedEndpointType: "primary",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			database, err := NewDatabaseV3(
				Metadata{
					Labels: tt.labels,
					Name:   "rds",
				},
				tt.spec,
			)
			tt.errorCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expectedAWS, database.GetAWS())
			require.Equal(t, tt.expectedEndpointType, database.GetEndpointType())
		})
	}
}

// TestDatabaseRDSProxyEndpoint verifies AWS info is correctly populated based
// on the RDS Proxy endpoint.
func TestDatabaseRDSProxyEndpoint(t *testing.T) {
	database, err := NewDatabaseV3(Metadata{
		Name: "rdsproxy",
	}, DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "my-proxy.proxy-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
	})
	require.NoError(t, err)
	require.Equal(t, AWS{
		Region: "us-west-1",
		RDSProxy: RDSProxy{
			Name: "my-proxy",
		},
	}, database.GetAWS())
}

// TestDatabaseRedshiftEndpoint verifies AWS info is correctly populated
// based on the Redshift endpoint.
func TestDatabaseRedshiftEndpoint(t *testing.T) {
	database, err := NewDatabaseV3(Metadata{
		Name: "redshift",
	}, DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
	})
	require.NoError(t, err)
	require.Equal(t, AWS{
		Region: "us-east-1",
		Redshift: Redshift{
			ClusterID: "redshift-cluster-1",
		},
	}, database.GetAWS())
}

// TestDatabaseStatus verifies database resource status field usage.
func TestDatabaseStatus(t *testing.T) {
	database, err := NewDatabaseV3(Metadata{
		Name: "test",
	}, DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	caCert := "test"
	database.SetStatusCA(caCert)
	require.Equal(t, caCert, database.GetCA())

	awsMeta := AWS{AccountID: "account-id"}
	database.SetStatusAWS(awsMeta)
	require.Equal(t, awsMeta, database.GetAWS())
}

func TestDatabaseElastiCacheEndpoint(t *testing.T) {
	t.Run("valid URI", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "elasticache",
		}, DatabaseSpecV3{
			Protocol: "redis",
			URI:      "clustercfg.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
		})

		require.NoError(t, err)
		require.Equal(t, AWS{
			Region: "ca-central-1",
			ElastiCache: ElastiCache{
				ReplicationGroupID:       "my-redis-cluster",
				TransitEncryptionEnabled: true,
				EndpointType:             "configuration",
			},
		}, database.GetAWS())
		require.True(t, database.IsElastiCache())
		require.True(t, database.IsAWSHosted())
		require.True(t, database.IsCloudHosted())
	})

	t.Run("invalid URI", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "elasticache",
		}, DatabaseSpecV3{
			Protocol: "redis",
			URI:      "some.endpoint.cache.amazonaws.com:6379",
			AWS: AWS{
				Region: "us-east-5",
				ElastiCache: ElastiCache{
					ReplicationGroupID: "some-id",
				},
			},
		})

		// A warning is logged, no error is returned, and AWS metadata is not
		// updated.
		require.NoError(t, err)
		require.Equal(t, AWS{
			Region: "us-east-5",
			ElastiCache: ElastiCache{
				ReplicationGroupID: "some-id",
			},
		}, database.GetAWS())
	})
}

func TestDatabaseMemoryDBEndpoint(t *testing.T) {
	t.Run("valid URI", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "memorydb",
		}, DatabaseSpecV3{
			Protocol: "redis",
			URI:      "clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
		})

		require.NoError(t, err)
		require.Equal(t, AWS{
			Region: "us-east-1",
			MemoryDB: MemoryDB{
				ClusterName:  "my-memorydb",
				TLSEnabled:   true,
				EndpointType: "cluster",
			},
		}, database.GetAWS())
		require.True(t, database.IsMemoryDB())
		require.True(t, database.IsAWSHosted())
		require.True(t, database.IsCloudHosted())
	})

	t.Run("invalid URI", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "memorydb",
		}, DatabaseSpecV3{
			Protocol: "redis",
			URI:      "some.endpoint.memorydb.amazonaws.com:6379",
			AWS: AWS{
				Region: "us-east-5",
				MemoryDB: MemoryDB{
					ClusterName: "clustername",
				},
			},
		})

		// A warning is logged, no error is returned, and AWS metadata is not
		// updated.
		require.NoError(t, err)
		require.Equal(t, AWS{
			Region: "us-east-5",
			MemoryDB: MemoryDB{
				ClusterName: "clustername",
			},
		}, database.GetAWS())
	})
}

func TestDatabaseAzureEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        DatabaseSpecV3
		expectError bool
		expectAzure Azure
	}{
		{
			name: "valid MySQL",
			spec: DatabaseSpecV3{
				Protocol: "mysql",
				URI:      "example-mysql.mysql.database.azure.com:3306",
			},
			expectAzure: Azure{
				Name: "example-mysql",
			},
		},
		{
			name: "valid PostgresSQL",
			spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "example-postgres.postgres.database.azure.com:5432",
			},
			expectAzure: Azure{
				Name: "example-postgres",
			},
		},
		{
			name: "invalid database endpoint",
			spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "invalid.database.azure.com:5432",
			},
			expectError: true,
		},
		{
			name: "valid Redis",
			spec: DatabaseSpecV3{
				Protocol: "redis",
				URI:      "example-redis.redis.cache.windows.net:6380",
				Azure: Azure{
					ResourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/Redis/example-redis",
				},
			},
			expectAzure: Azure{
				Name:       "example-redis",
				ResourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/Redis/example-redis",
			},
		},
		{
			name: "valid Redis Enterprise",
			spec: DatabaseSpecV3{
				Protocol: "redis",
				URI:      "rediss://example-redis-enterprise.region.redisenterprise.cache.azure.net?mode=cluster",
				Azure: Azure{
					ResourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-redis-enterprise",
				},
			},
			expectAzure: Azure{
				Name:       "example-redis-enterprise",
				ResourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-redis-enterprise",
			},
		},
		{
			name: "invalid Redis (missing resource ID)",
			spec: DatabaseSpecV3{
				Protocol: "redis",
				URI:      "rediss://example-redis-enterprise.region.redisenterprise.cache.azure.net?mode=cluster",
			},
			expectError: true,
		},
		{
			name: "invalid Redis (unknown format)",
			spec: DatabaseSpecV3{
				Protocol: "redis",
				URI:      "rediss://bad-format.redisenterprise.cache.azure.net?mode=cluster",
				Azure: Azure{
					ResourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/bad-format",
				},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, err := NewDatabaseV3(Metadata{
				Name: "test",
			}, test.spec)

			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectAzure, database.GetAzure())
			}
		})
	}
}

func TestMySQLVersionValidation(t *testing.T) {
	t.Parallel()

	t.Run("correct config", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "mysql",
			URI:      "localhost:5432",
			MySQL: MySQLOptions{
				ServerVersion: "8.0.18",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "8.0.18", database.GetMySQLServerVersion())
	})

	t.Run("incorrect config - wrong protocol", func(t *testing.T) {
		_, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "Postgres",
			URI:      "localhost:5432",
			MySQL: MySQLOptions{
				ServerVersion: "8.0.18",
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "ServerVersion")
	})
}

func TestMySQLServerVersion(t *testing.T) {
	t.Parallel()

	database, err := NewDatabaseV3(Metadata{
		Name: "test",
	}, DatabaseSpecV3{
		Protocol: "mysql",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	require.Equal(t, "", database.GetMySQLServerVersion())

	database.SetMySQLServerVersion("8.0.1")
	require.Equal(t, "8.0.1", database.GetMySQLServerVersion())
}

func TestCassandraAWSEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("aws cassandra url from region", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "cassandra",
			AWS: AWS{
				Region:    "us-west-1",
				AccountID: "123456789012",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "cassandra.us-west-1.amazonaws.com:9142", database.GetURI())
	})

	t.Run("aws cassandra custom uri", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "cassandra",
			URI:      "cassandra.us-west-1.amazonaws.com:9142",
			AWS: AWS{
				AccountID: "123456789012",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "cassandra.us-west-1.amazonaws.com:9142", database.GetURI())
		require.Equal(t, "us-west-1", database.GetAWS().Region)
	})

	t.Run("aws cassandra custom fips uri", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "cassandra",
			URI:      "cassandra-fips.us-west-2.amazonaws.com:9142",
			AWS: AWS{
				AccountID: "123456789012",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "cassandra-fips.us-west-2.amazonaws.com:9142", database.GetURI())
		require.Equal(t, "us-west-2", database.GetAWS().Region)
	})

	t.Run("aws cassandra missing AccountID", func(t *testing.T) {
		_, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "cassandra",
			URI:      "cassandra.us-west-1.amazonaws.com:9142",
			AWS: AWS{
				AccountID: "",
			},
		})
		require.Error(t, err)
	})
}

func TestDatabaseFromRedshiftServerlessEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("workgroup", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "postgres",
			URI:      "my-workgroup.123456789012.us-east-1.redshift-serverless.amazonaws.com:5439",
		})
		require.NoError(t, err)
		require.Equal(t, AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			RedshiftServerless: RedshiftServerless{
				WorkgroupName: "my-workgroup",
			},
		}, database.GetAWS())
	})

	t.Run("vpc endpoint", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "postgres",
			URI:      "my-vpc-endpoint-xxxyyyzzz.123456789012.us-east-1.redshift-serverless.amazonaws.com:5439",
			AWS: AWS{
				RedshiftServerless: RedshiftServerless{
					WorkgroupName: "my-workgroup",
				},
			},
		})
		require.NoError(t, err)
		require.Equal(t, AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			RedshiftServerless: RedshiftServerless{
				WorkgroupName: "my-workgroup",
				EndpointName:  "my-vpc",
			},
		}, database.GetAWS())
	})
}

func TestDatabaseSelfHosted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inputURI string
	}{
		{
			name:     "localhost",
			inputURI: "localhost:5432",
		},
		{
			name:     "ec2 hostname",
			inputURI: "ec2-11-22-33-44.us-east-2.compute.amazonaws.com:5432",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, err := NewDatabaseV3(Metadata{
				Name: "self-hosted-localhost",
			}, DatabaseSpecV3{
				Protocol: "postgres",
				URI:      test.inputURI,
			})
			require.NoError(t, err)
			require.Equal(t, DatabaseTypeSelfHosted, database.GetType())
			require.False(t, database.IsCloudHosted())
		})
	}
}

func TestDynamoDBConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		uri        string
		region     string
		account    string
		roleARN    string
		externalID string
		wantSpec   DatabaseSpecV3
		wantErrMsg string
	}{
		{
			desc:    "account and region and empty URI is correct",
			region:  "us-west-1",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				URI: "aws://dynamodb.us-west-1.amazonaws.com",
				AWS: AWS{
					Region:    "us-west-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:       "account and region and assume role is correct",
			region:     "us-west-1",
			account:    "123456789012",
			roleARN:    "arn:aws:iam::123456789012:role/DBDiscoverer",
			externalID: "externalid123",
			wantSpec: DatabaseSpecV3{
				URI: "aws://dynamodb.us-west-1.amazonaws.com",
				AWS: AWS{
					Region:        "us-west-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalid123",
				},
			},
		},
		{
			desc:    "account and AWS URI and empty region is correct",
			uri:     "dynamodb.us-west-1.amazonaws.com",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				URI: "dynamodb.us-west-1.amazonaws.com",
				AWS: AWS{
					Region:    "us-west-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:    "account and AWS streams dynamodb URI and empty region is correct",
			uri:     "streams.dynamodb.us-west-1.amazonaws.com",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				URI: "streams.dynamodb.us-west-1.amazonaws.com",
				AWS: AWS{
					Region:    "us-west-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:    "account and AWS dax URI and empty region is correct",
			uri:     "dax.us-west-1.amazonaws.com",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				URI: "dax.us-west-1.amazonaws.com",
				AWS: AWS{
					Region:    "us-west-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:    "account and region and matching AWS URI region is correct",
			uri:     "dynamodb.us-west-1.amazonaws.com",
			region:  "us-west-1",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				URI: "dynamodb.us-west-1.amazonaws.com",
				AWS: AWS{
					Region:    "us-west-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:    "account and region and custom URI is correct",
			uri:     "localhost:8080",
			region:  "us-west-1",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				URI: "localhost:8080",
				AWS: AWS{
					Region:    "us-west-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:       "configured external ID but not assume role is ok",
			uri:        "localhost:8080",
			region:     "us-west-1",
			account:    "123456789012",
			externalID: "externalid123",
			wantSpec: DatabaseSpecV3{
				URI: "localhost:8080",
				AWS: AWS{
					Region:     "us-west-1",
					AccountID:  "123456789012",
					ExternalID: "externalid123",
				},
			},
		},
		{
			desc:       "region and different AWS URI region is an error",
			uri:        "dynamodb.us-west-2.amazonaws.com",
			region:     "us-west-1",
			account:    "123456789012",
			wantErrMsg: "does not match the configured URI",
		},
		{
			desc:       "invalid AWS URI is an error",
			uri:        "a.streams.dynamodb.us-west-1.amazonaws.com",
			region:     "us-west-1",
			account:    "123456789012",
			wantErrMsg: "invalid DynamoDB endpoint",
		},
		{
			desc:       "custom URI and missing region is an error",
			uri:        "localhost:8080",
			account:    "123456789012",
			wantErrMsg: "region is empty",
		},
		{
			desc:       "missing URI and missing region is an error",
			account:    "123456789012",
			wantErrMsg: "URI is empty",
		},
		{
			desc:       "invalid AWS account ID is an error",
			uri:        "localhost:8080",
			region:     "us-west-1",
			account:    "12345",
			wantErrMsg: "must be 12-digit",
		},
		{
			region:     "us-west-1",
			desc:       "missing account id",
			wantErrMsg: "account ID is empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			database, err := NewDatabaseV3(Metadata{
				Name: "test",
			}, DatabaseSpecV3{
				Protocol: "dynamodb",
				URI:      tt.uri,
				AWS: AWS{
					Region:        tt.region,
					AccountID:     tt.account,
					AssumeRoleARN: tt.roleARN,
					ExternalID:    tt.externalID,
				},
			})
			if tt.wantErrMsg != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}
			require.NoError(t, err)
			diff := cmp.Diff(tt.wantSpec, database.Spec, cmpopts.IgnoreFields(DatabaseSpecV3{}, "Protocol"))
			require.Empty(t, diff)
		})
	}
}

func TestOpenSearchConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		uri        string
		region     string
		account    string
		wantSpec   DatabaseSpecV3
		wantErrMsg string
	}{
		{
			desc:       "missing account is an error",
			uri:        "my-opensearch-instance-xxxxxx.us-west-2.amazonaws.com",
			region:     "us-west-2",
			account:    "",
			wantErrMsg: "database \"test\" AWS account ID is empty",
		},
		{
			desc:       "custom URI without region is an error",
			uri:        "localhost:8080",
			region:     "",
			account:    "123456789012",
			wantErrMsg: "database \"test\" AWS region is missing and cannot be derived from the URI \"localhost:8080\"",
		},
		{
			desc:    "custom URI with region is correct",
			uri:     "localhost:8080",
			region:  "eu-central-1",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				Protocol: "opensearch",
				URI:      "localhost:8080",
				AWS: AWS{
					Region:    "eu-central-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:       "AWS URI for wrong service",
			uri:        "my-opensearch-instance-xxxxxx.eu-central-1.foobar.amazonaws.com",
			region:     "eu-central-1",
			account:    "123456789012",
			wantErrMsg: "invalid OpenSearch endpoint \"my-opensearch-instance-xxxxxx.eu-central-1.foobar.amazonaws.com\", invalid service \"foobar\"",
		},
		{
			desc:    "region is optional if it can be derived from URI",
			uri:     "my-opensearch-instance-xxxxxx.eu-central-1.es.amazonaws.com",
			region:  "",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				Protocol: "opensearch",
				URI:      "my-opensearch-instance-xxxxxx.eu-central-1.es.amazonaws.com",
				AWS: AWS{
					Region:    "eu-central-1",
					AccountID: "123456789012",
				},
			},
		},
		{
			desc:       "URI-derived region must match explicit region",
			uri:        "my-opensearch-instance-xxxxxx.eu-central-1.es.amazonaws.com",
			region:     "eu-central-2",
			account:    "123456789012",
			wantErrMsg: "database \"test\" AWS region \"eu-central-2\" does not match the configured URI region \"eu-central-1\"",
		},

		{
			desc:    "no error when full data provided and matches",
			uri:     "my-opensearch-instance-xxxxxx.eu-central-1.es.amazonaws.com",
			region:  "eu-central-1",
			account: "123456789012",
			wantSpec: DatabaseSpecV3{
				Protocol: "opensearch",
				URI:      "my-opensearch-instance-xxxxxx.eu-central-1.es.amazonaws.com",
				AWS: AWS{
					Region:    "eu-central-1",
					AccountID: "123456789012",
				},
			},
		},

		{
			desc:       "invalid AWS account ID is an error",
			uri:        "localhost:8080",
			region:     "us-west-1",
			account:    "12345",
			wantErrMsg: "must be 12-digit",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			database, err := NewDatabaseV3(Metadata{
				Name: "test",
			}, DatabaseSpecV3{
				Protocol: "opensearch",
				URI:      tt.uri,
				AWS: AWS{
					Region:    tt.region,
					AccountID: tt.account,
				},
			})

			if tt.wantErrMsg != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}

			require.NoError(t, err)
			require.True(t, database.IsOpenSearch())
			require.Equal(t, tt.wantSpec, database.Spec)
		})
	}
}

func TestAWSIsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  AWS
		assert require.BoolAssertionFunc
	}{
		{
			name:   "true",
			input:  AWS{},
			assert: require.True,
		},
		{
			name: "true with unrecognized bytes",
			input: AWS{
				XXX_unrecognized: []byte{66, 0},
			},
			assert: require.True,
		},
		{
			name: "true with nested unrecognized bytes",
			input: AWS{
				MemoryDB: MemoryDB{
					XXX_unrecognized: []byte{99, 0},
				},
			},
			assert: require.True,
		},
		{
			name: "false",
			input: AWS{
				Region: "us-west-1",
			},
			assert: require.False,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.assert(t, test.input.IsEmpty())
		})
	}
}

func TestValidateDatabaseName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		dbName            string
		expectErrContains string
	}{
		{
			name:   "valid long name and uppercase chars",
			dbName: strings.Repeat("aA", 100),
		},
		{
			name:              "invalid trailing hyphen",
			dbName:            "invalid-database-name-",
			expectErrContains: `"invalid-database-name-" does not match regex`,
		},
		{
			name:              "invalid first character",
			dbName:            "1-invalid-database-name",
			expectErrContains: `"1-invalid-database-name" does not match regex`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDatabaseName(test.dbName)
			if test.expectErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.expectErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestIAMPolicyStatusJSON(t *testing.T) {
	t.Parallel()

	status := IAMPolicyStatus_IAM_POLICY_STATUS_SUCCESS

	marshaled, err := status.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `"IAM_POLICY_STATUS_SUCCESS"`, string(marshaled))

	data, err := json.Marshal("IAM_POLICY_STATUS_FAILED")
	require.NoError(t, err)
	require.NoError(t, status.UnmarshalJSON(data))
	require.Equal(t, IAMPolicyStatus_IAM_POLICY_STATUS_FAILED, status)
}

func TestDatabaseSpanner(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		spec       DatabaseSpecV3
		errorCheck require.ErrorAssertionFunc
	}{
		"valid with uri": {
			spec: DatabaseSpecV3{
				Protocol: "spanner",
				URI:      gcputils.SpannerEndpoint,
				GCP: GCPCloudSQL{
					ProjectID:  "project-id",
					InstanceID: "instance-id",
				},
			},
			errorCheck: require.NoError,
		},
		"valid without uri": {
			spec: DatabaseSpecV3{
				Protocol: "spanner",
				GCP: GCPCloudSQL{
					ProjectID:  "project-id",
					InstanceID: "instance-id",
				},
			},
			errorCheck: require.NoError,
		},
		"invalid missing project id": {
			spec: DatabaseSpecV3{
				Protocol: "spanner",
				GCP: GCPCloudSQL{
					InstanceID: "instance-id",
				},
			},
			errorCheck: require.Error,
		},
		"invalid missing instance id": {
			spec: DatabaseSpecV3{
				Protocol: "spanner",
				GCP: GCPCloudSQL{
					ProjectID: "project-id",
				},
			},
			errorCheck: require.Error,
		},
		"invalid missing project and instance id for spanner protocol": {
			spec: DatabaseSpecV3{
				Protocol: "spanner",
			},
			errorCheck: require.Error,
		},
		"invalid missing project and instance id for spanner endpoint": {
			spec: DatabaseSpecV3{
				URI: gcputils.SpannerEndpoint,
			},
			errorCheck: require.Error,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			db, err := NewDatabaseV3(
				Metadata{
					Name: "my-spanner",
				},
				test.spec,
			)
			test.errorCheck(t, err)
			if err != nil {
				return
			}

			require.True(t, db.IsGCPHosted())
			require.Equal(t, DatabaseTypeSpanner, db.GetType())
			require.Equal(t, gcputils.SpannerEndpoint, db.GetURI())
		})
	}
}
