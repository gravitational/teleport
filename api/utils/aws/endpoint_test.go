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
		expectInstanceID    string
		expectRegion        string
		expectParseErrorIs  func(error) bool
	}{
		{
			name:                "standard",
			endpoint:            "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectInstanceID:    "aurora-instance-1",
			expectRegion:        "us-west-1",
		},
		{
			name:                "cn-north-1",
			endpoint:            "aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn",
			expectIsRDSEndpoint: true,
			expectInstanceID:    "aurora-instance-2",
			expectRegion:        "cn-north-1",
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

			instanceID, region, err := ParseRDSEndpoint(test.endpoint)
			if test.expectParseErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectParseErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectInstanceID, instanceID)
				require.Equal(t, test.expectRegion, region)
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
