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
				InstanceID: "aurora-instance-1",
				Region:     "us-west-1",
			},
		},
		{
			name:                "RDS instance in cn-north-1",
			endpoint:            "aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				InstanceID: "aurora-instance-2",
				Region:     "cn-north-1",
			},
		},
		{
			name:                "RDS cluster",
			endpoint:            "my-cluster.cluster-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ClusterID: "my-cluster",
				Region:    "us-west-1",
			},
		},
		{
			name:                "RDS cluster custom endpoint",
			endpoint:            "my-custom.cluster-custom-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectDetails: &RDSEndpointDetails{
				ClusterCustomEndpointName: "my-custom",
				Region:                    "us-west-1",
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
			name:     "primary endpiont, TLS disabled",
			inputURI: "my-redis-cluster.xxxxxx.ng.0001.cac1.cache.amazonaws.com:6379",
			expectInfo: &RedisEndpointInfo{
				ID:           "my-redis-cluster",
				Region:       "ca-central-1",
				EndpointType: ElastiCachePrimaryEndpoint,
			},
		},
		{
			name:     "reader endpiont, TLS disabled",
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
				require.Equal(t, got, test.wantRegion)
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
			endpoint:                           "my-workgroup.1234567890.us-east-1.redshift-serverless.amazonaws.com:5439",
			expectIsRedshiftServerlessEndpoint: true,
			expectDetails: &RedshiftServerlessEndpointDetails{
				WorkgroupName: "my-workgroup",
				AccountID:     "1234567890",
				Region:        "us-east-1",
			},
		},
		{
			name:                               "vpc endpoint",
			endpoint:                           "my-vpc-endpoint-xxxyyyzzz.1234567890.us-east-1.redshift-serverless.amazonaws.com",
			expectIsRedshiftServerlessEndpoint: true,
			expectDetails: &RedshiftServerlessEndpointDetails{
				EndpointName: "my-vpc",
				AccountID:    "1234567890",
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
