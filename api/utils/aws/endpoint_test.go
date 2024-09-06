/*
Copyright 2022 Gravitational, Inc.

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

package aws

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestParseRDSEndpoint(t *testing.T) {
	tests := []struct {
		name                string
		endpoint            string
		expectIsRDSEndpoint bool
		expectDetails       *RDSEndpointDetails
		expectParseErrorIs  func(error) bool
	}{
		{
			name:                "RDS instance",
			endpoint:            "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				InstanceID:   "aurora-instance-1",
				Region:       "us-west-1",
				EndpointType: "instance",
			},
		},
		{
			name:                "RDS instance in cn-north-1",
			endpoint:            "aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				InstanceID:   "aurora-instance-2",
				Region:       "cn-north-1",
				EndpointType: "instance",
			},
		},
		{
			name:                "RDS cluster",
			endpoint:            "my-cluster.cluster-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ClusterID:    "my-cluster",
				Region:       "us-west-1",
				EndpointType: "primary",
			},
		},
		{
			name:                "RDS cluster reader",
			endpoint:            "my-cluster.cluster-ro-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ClusterID:    "my-cluster",
				Region:       "us-west-1",
				EndpointType: "reader",
			},
		},
		{
			name:                "RDS cluster custom endpoint",
			endpoint:            "my-custom.cluster-custom-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ClusterCustomEndpointName: "my-custom",
				Region:                    "us-west-1",
				EndpointType:              "custom",
			},
		},
		{
			name:                "RDS proxy",
			endpoint:            "my-proxy.proxy-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ProxyName: "my-proxy",
				Region:    "us-west-1",
			},
		},
		{
			name:                "RDS proxy custom endpoint",
			endpoint:            "my-custom.endpoint.proxy-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ProxyCustomEndpointName: "my-custom",
				Region:                  "us-west-1",
			},
		},
		{
			name:                "localhost:5432",
			endpoint:            "localhost",
			expectIsRDSEndpoint: false,
			expectParseErrorIs:  trace.IsBadParameter,
		},
		{
			name:                "Redshift endpoint fails",
			endpoint:            "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com",
			expectIsRDSEndpoint: false,
			expectParseErrorIs:  trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRDSEndpoint, IsRDSEndpoint(test.endpoint))

			actualDetails, err := ParseRDSEndpoint(test.endpoint)
			if test.expectParseErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectParseErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectDetails, actualDetails)
			}
		})
	}
}

func TestParseRedshiftEndpoint(t *testing.T) {
	tests := []struct {
		name                     string
		endpoint                 string
		expectIsRedshiftEndpoint bool
		expectClusterID          string
		expectRegion             string
		expectParseErrorIs       func(error) bool
	}{
		{
			name:                     "standard",
			endpoint:                 "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5432",
			expectClusterID:          "redshift-cluster-1",
			expectRegion:             "us-east-1",
			expectIsRedshiftEndpoint: true,
		},
		{
			name:                     "cn-north-1",
			endpoint:                 "redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-1.amazonaws.com.cn",
			expectClusterID:          "redshift-cluster-2",
			expectRegion:             "cn-north-1",
			expectIsRedshiftEndpoint: true,
		},
		{
			name:                     "localhost:5432",
			endpoint:                 "localhost",
			expectIsRedshiftEndpoint: false,
			expectParseErrorIs:       trace.IsBadParameter,
		},
		{
			name:                     "RDS endpoint fails",
			endpoint:                 "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com",
			expectIsRedshiftEndpoint: false,
			expectParseErrorIs:       trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRedshiftEndpoint, IsRedshiftEndpoint(test.endpoint))

			clusterID, region, err := ParseRedshiftEndpoint(test.endpoint)
			if test.expectParseErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectParseErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectClusterID, clusterID)
				require.Equal(t, test.expectRegion, region)
			}
		})
	}
}

func TestParseElastiCacheEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		inputURI    string
		expectInfo  *RedisEndpointInfo
		expectError bool
	}{
		{
			name:     "configuration endpoint, TLS enabled",
			inputURI: "clustercfg.my-redis-shards.xxxxxx.use1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-redis-shards",
				Region:                   "us-east-1",
				TransitEncryptionEnabled: true,
				EndpointType:             ElastiCacheConfigurationEndpoint,
			},
		},
		{
			name:     "primary endpoint, TLS enabled",
			inputURI: "master.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-redis-cluster",
				Region:                   "ca-central-1",
				TransitEncryptionEnabled: true,
				EndpointType:             ElastiCachePrimaryEndpoint,
			},
		},
		{
			name:     "reader endpoint, TLS enabled",
			inputURI: "replica.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-redis-cluster",
				Region:                   "ca-central-1",
				TransitEncryptionEnabled: true,
				EndpointType:             ElastiCacheReaderEndpoint,
			},
		},
		{
			name:     "node endpoint, TLS enabled",
			inputURI: "my-redis-shards-0002-001.my-redis-shards.xxxxxx.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-redis-shards",
				Region:                   "ca-central-1",
				TransitEncryptionEnabled: true,
				EndpointType:             ElastiCacheNodeEndpoint,
			},
		},
		{
			name:     "configuration endpoint, TLS disabled",
			inputURI: "my-redis-shards.xxxxxx.clustercfg.use1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:           "my-redis-shards",
				Region:       "us-east-1",
				EndpointType: ElastiCacheConfigurationEndpoint,
			},
		},
		{
			name:     "primary endpoint, TLS disabled",
			inputURI: "my-redis-cluster.xxxxxx.ng.0001.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:           "my-redis-cluster",
				Region:       "ca-central-1",
				EndpointType: ElastiCachePrimaryEndpoint,
			},
		},
		{
			name:     "reader endpoint, TLS disabled",
			inputURI: "my-redis-cluster-ro.xxxxxx.ng.0001.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:           "my-redis-cluster",
				Region:       "ca-central-1",
				EndpointType: ElastiCacheReaderEndpoint,
			},
		},
		{
			name:     "node endpoint, TLS disabled",
			inputURI: "my-redis-shards-0001-001.xxxxxx.0001.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:           "my-redis-shards",
				Region:       "ca-central-1",
				EndpointType: ElastiCacheNodeEndpoint,
			},
		},
		{
			name:     "CN endpoint",
			inputURI: "replica.my-redis-cluster.xxxxxx.cnn1.cache.amazonaws.com.cn:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-redis-cluster",
				Region:                   "cn-north-1",
				TransitEncryptionEnabled: true,
				EndpointType:             ElastiCacheReaderEndpoint,
			},
		},
		{
			name:     "endpoint with schema and parameters",
			inputURI: "redis://my-redis-cluster.xxxxxx.ng.0001.cac1.cache.amazonaws.com:6379?a=b&c=d",
			expectInfo: &RedisEndpointInfo{
				ID:           "my-redis-cluster",
				Region:       "ca-central-1",
				EndpointType: ElastiCachePrimaryEndpoint,
			},
		},
		{
			name:        "invalid suffix",
			inputURI:    "replica.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.ca:6379",
			expectError: true,
		},
		{
			name:        "invalid url",
			inputURI:    "://replica.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
			expectError: true,
		},
		{
			name:        "invalid format",
			inputURI:    "my-redis-cluster.cac1.cache.amazonaws.com:6379",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualInfo, err := ParseElastiCacheEndpoint(test.inputURI)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectInfo, actualInfo)
			}
		})
	}
}

func TestParseMemoryDBEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputURI    string
		expectInfo  *RedisEndpointInfo
		expectError bool
	}{
		{
			name:     "TLS enabled cluster endpoint",
			inputURI: "clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-memorydb",
				Region:                   "us-east-1",
				TransitEncryptionEnabled: true,
				EndpointType:             "cluster",
			},
		},
		{
			name:     "TLS disabled cluster endpoint",
			inputURI: "my-memorydb.xxxxxx.clustercfg.memorydb.us-east-1.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-memorydb",
				Region:                   "us-east-1",
				TransitEncryptionEnabled: false,
				EndpointType:             "cluster",
			},
		},
		{
			name:     "TLS enabled node endpoint",
			inputURI: "my-memorydb-0002-001.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-memorydb",
				Region:                   "us-east-1",
				TransitEncryptionEnabled: true,
				EndpointType:             "node",
			},
		},
		{
			name:     "TLS disabled node endpoint",
			inputURI: "my-memorydb-0002-001.xxxxx.0002.memorydb.us-east-1.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-memorydb",
				Region:                   "us-east-1",
				TransitEncryptionEnabled: false,
				EndpointType:             "node",
			},
		},
		{
			name:     "CN endpoint",
			inputURI: "clustercfg.my-memorydb.xxxxxx.memorydb.cn-north-1.amazonaws.com.cn:6379",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-memorydb",
				Region:                   "cn-north-1",
				TransitEncryptionEnabled: true,
				EndpointType:             "cluster",
			},
		},
		{
			name:     "endpoint with schema and parameters",
			inputURI: "redis://clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379?a=b&c=d",
			expectInfo: &RedisEndpointInfo{
				ID:                       "my-memorydb",
				Region:                   "us-east-1",
				TransitEncryptionEnabled: true,
				EndpointType:             "cluster",
			},
		},
		{
			name:        "invalid suffix",
			inputURI:    "clustercfg.my-memorydb.xxxxxx.memorydb.ca-central-1.amazonaws.ca:6379",
			expectError: true,
		},
		{
			name:        "invalid url",
			inputURI:    "://clustercfg.my-memorydb.xxxxxx.memorydb.ca-central-1.amazonaws.com:6379",
			expectError: true,
		},
		{
			name:        "invalid format",
			inputURI:    "unknown.format.memorydb.ca-central-1.amazonaws.com:6379",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualInfo, err := ParseMemoryDBEndpoint(test.inputURI)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectInfo, actualInfo)
			}
		})
	}
}

func TestCassandraEndpointRegion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputURI    string
		wantRegion  string
		expectError bool
	}{
		{
			name:        "us-east-1",
			inputURI:    "cassandra.us-east-1.amazonaws.com",
			wantRegion:  "us-east-1",
			expectError: false,
		},
		{
			name:        "cn-north-1.",
			inputURI:    "cassandra.cn-north-1.amazonaws.com.cn",
			wantRegion:  "cn-north-1",
			expectError: false,
		},
		{
			name:        "us-gov-east-1",
			inputURI:    "cassandra.us-gov-east-1.amazonaws.com",
			wantRegion:  "us-gov-east-1",
			expectError: false,
		},
		{
			name:        "invalid uri",
			inputURI:    "foo.cassandra.us-east-1.amazonaws.com",
			wantRegion:  "us-east-1",
			expectError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CassandraEndpointRegion(test.inputURI)
			if test.expectError {
				require.Error(t, err)
				require.False(t, IsKeyspacesEndpoint(test.inputURI))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantRegion, got)
				require.True(t, IsKeyspacesEndpoint(test.inputURI))
			}
		})
	}

}

func TestRedshiftServerlessEndpoint(t *testing.T) {
	tests := []struct {
		name                               string
		endpoint                           string
		expectIsRedshiftServerlessEndpoint bool
		expectDetails                      *RedshiftServerlessEndpointDetails
	}{
		{
			name:                               "workgroup endpoint",
			endpoint:                           "my-workgroup.123456789012.us-east-1.redshift-serverless.amazonaws.com:5439",
			expectIsRedshiftServerlessEndpoint: true,
			expectDetails: &RedshiftServerlessEndpointDetails{
				WorkgroupName: "my-workgroup",
				AccountID:     "123456789012",
				Region:        "us-east-1",
			},
		},
		{
			name:                               "vpc endpoint",
			endpoint:                           "my-vpc-endpoint-xxxyyyzzz.123456789012.us-east-1.redshift-serverless.amazonaws.com",
			expectIsRedshiftServerlessEndpoint: true,
			expectDetails: &RedshiftServerlessEndpointDetails{
				EndpointName: "my-vpc",
				AccountID:    "123456789012",
				Region:       "us-east-1",
			},
		},
		{
			name:                               "localhost:5432",
			endpoint:                           "localhost",
			expectIsRedshiftServerlessEndpoint: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRedshiftServerlessEndpoint, IsRedshiftServerlessEndpoint(test.endpoint))

			actualDetails, err := ParseRedshiftServerlessEndpoint(test.endpoint)
			if !test.expectIsRedshiftServerlessEndpoint {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectDetails, actualDetails)
			}
		})
	}
}

func TestDynamoDBURIForRegion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc          string
		region        string
		wantURI       string
		wantPartition string
	}{
		{
			desc:          "region is in correct AWS partition",
			region:        "us-east-1",
			wantURI:       "aws://dynamodb.us-east-1.amazonaws.com",
			wantPartition: ".amazonaws.com",
		},
		{
			desc:          "china north region is in correct AWS partition",
			region:        "cn-north-1",
			wantURI:       "aws://dynamodb.cn-north-1.amazonaws.com.cn",
			wantPartition: ".amazonaws.com.cn",
		},
		{
			desc:          "china northwest region is in correct AWS partition",
			region:        "cn-northwest-1",
			wantURI:       "aws://dynamodb.cn-northwest-1.amazonaws.com.cn",
			wantPartition: ".amazonaws.com.cn",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.wantURI, DynamoDBURIForRegion(tt.region))
			info, err := ParseDynamoDBEndpoint(tt.wantURI)
			require.NoError(t, err, "endpoint generated from region could not be parsed.")
			require.Equal(t, tt.region, info.Region)
			require.Equal(t, "dynamodb", info.Service)
			require.Equal(t, tt.wantPartition, info.Partition)
		})
	}
}

func TestParseDynamoDBEndpoint(t *testing.T) {
	t.Parallel()
	t.Run("parses valid endpoint", func(t *testing.T) {
		t.Parallel()
		for _, parts := range []struct {
			services  []string
			regions   []string
			partition string
		}{
			{
				services:  []string{DynamoDBServiceName, DynamoDBFipsServiceName, DynamoDBStreamsServiceName, DAXServiceName},
				regions:   []string{"us-east-1", "us-gov-east-1"},
				partition: AWSEndpointSuffix,
			},
			{
				services:  []string{DynamoDBServiceName, DynamoDBStreamsServiceName, DAXServiceName},
				regions:   []string{"cn-north-1", "cn-northwest-1"},
				partition: AWSCNEndpointSuffix,
			},
		} {
			parts := parts
			for _, svc := range parts.services {
				svc := svc
				for _, region := range parts.regions {
					region := region
					endpoint := fmt.Sprintf("%s.%s%s", svc, region, parts.partition)
					t.Run(endpoint, func(t *testing.T) {
						t.Parallel()
						info, err := ParseDynamoDBEndpoint(endpoint)
						require.NoError(t, err)
						wantInfo := DynamoDBEndpointInfo{
							Service:   svc,
							Region:    region,
							Partition: parts.partition,
						}
						require.NotNil(t, info)
						require.Equal(t, wantInfo, *info)
					})
				}
			}
		}
	})

	tests := []struct {
		desc     string
		services []string
		regions  []string
		endpoint string
		wantInfo *DynamoDBEndpointInfo
	}{
		{
			desc:     "empty uri",
			endpoint: "",
		},
		{
			desc:     "not AWS uri",
			endpoint: "localhost",
		},
		{
			desc:     "missing region",
			endpoint: "amazonaws.com",
		},
		{
			desc:     "missing china region",
			endpoint: "amazonaws.com.cn",
		},
		{
			desc:     "unrecognized service subdomain",
			endpoint: "foo.us-east-1.amazonaws.com",
		},
		{
			desc:     "unrecognized dynamodb service subdomain",
			endpoint: "foo.dynamodb.us-east-1.amazonaws.com",
		},
		{
			desc:     "unrecognized streams service subdomain",
			endpoint: "streams.foo.us-east-1.amazonaws.com",
		},
		{
			desc:     "mismatched us region and china partition",
			endpoint: "streams.dynamodb.us-east-1.amazonaws.com.cn",
		},
		{
			desc:     "mismatched china region and non-china partition",
			endpoint: "streams.dynamodb.cn-north-1.amazonaws.com",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run("detects invalid endpoint with "+tt.desc, func(t *testing.T) {
			t.Parallel()
			info, err := ParseDynamoDBEndpoint(tt.endpoint)
			require.Error(t, err, "endpoint %s should be invalid", tt.endpoint)
			require.Nil(t, info)
		})
	}
}

func TestParseOpensearchEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("fixed example", func(t *testing.T) {
		want := &OpenSearchEndpointInfo{
			Service:   OpenSearchServiceName,
			Region:    "eu-central-1",
			Partition: AWSEndpointSuffix,
		}

		endpoint := "https://search-my-opensearch-instance-xxxxxxxxxxxxxxxx.eu-central-1.es.amazonaws.com:443"

		out, err := ParseOpensearchEndpoint(endpoint)
		require.NoError(t, err)
		require.Equal(t, want, out)
	})

	t.Run("parses valid endpoint", func(t *testing.T) {
		t.Parallel()
		for _, parts := range []struct {
			services  []string
			regions   []string
			partition string
		}{
			{
				services:  []string{OpenSearchServiceName},
				regions:   []string{"us-east-1", "us-gov-east-1"},
				partition: AWSEndpointSuffix,
			},
			{
				services:  []string{OpenSearchServiceName},
				regions:   []string{"cn-north-1", "cn-northwest-1"},
				partition: AWSCNEndpointSuffix,
			},
		} {
			parts := parts
			for _, svc := range parts.services {
				svc := svc
				for _, region := range parts.regions {
					region := region
					endpoint := fmt.Sprintf("opensearch-instance-foo.%s.%s%s", region, svc, parts.partition)
					t.Run(endpoint, func(t *testing.T) {
						t.Parallel()
						info, err := ParseOpensearchEndpoint(endpoint)
						require.NoError(t, err)
						wantInfo := OpenSearchEndpointInfo{
							Service:   svc,
							Region:    region,
							Partition: parts.partition,
						}
						require.NotNil(t, info)
						require.Equal(t, wantInfo, *info)
					})
				}
			}
		}
	})

	tests := []struct {
		desc     string
		endpoint string
	}{
		{
			desc:     "empty uri",
			endpoint: "",
		},
		{
			desc:     "not AWS uri",
			endpoint: "localhost",
		},
		{
			desc:     "missing region",
			endpoint: "amazonaws.com",
		},
		{
			desc:     "missing china region",
			endpoint: "amazonaws.com.cn",
		},
		{
			desc:     "unrecognized service subdomain",
			endpoint: "foo.us-east-1.amazonaws.com",
		},
		{
			desc:     "unrecognized opensearch service subdomain",
			endpoint: "foo.opensearch.us-east-1.amazonaws.com",
		},
		{
			desc:     "unrecognized streams service subdomain",
			endpoint: "streams.foo.us-east-1.amazonaws.com",
		},
		{
			desc:     "mismatched us region and china partition",
			endpoint: "streams.opensearch.us-east-1.amazonaws.com.cn",
		},
		{
			desc:     "mismatched china region and non-china partition",
			endpoint: "streams.opensearch.cn-north-1.amazonaws.com",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run("detects invalid endpoint with "+tt.desc, func(t *testing.T) {
			t.Parallel()
			info, err := ParseOpensearchEndpoint(tt.endpoint)
			require.Error(t, err, "endpoint %s should be invalid", tt.endpoint)
			require.Nil(t, info)
		})
	}
}
