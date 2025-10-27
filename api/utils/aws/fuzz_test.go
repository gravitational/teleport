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

	"github.com/stretchr/testify/require"
)

func FuzzParseRDSEndpoint(f *testing.F) {
	f.Add("")
	f.Add(":123")
	f.Add("foo:123")
	f.Add("foo" + AWSCNEndpointSuffix)
	f.Add("a.cluster-custom-b.c." + RDSServiceName + AWSEndpointSuffix)
	f.Add("a.cluster-b.c." + RDSServiceName + AWSEndpointSuffix)
	f.Add("a.proxy-b.c." + RDSServiceName + AWSEndpointSuffix)
	f.Add("a.b.c." + RDSServiceName + AWSEndpointSuffix)
	f.Add("a.endpoint.proxy-c.d." + RDSServiceName + AWSEndpointSuffix)
	f.Add("a.cluster-custom-b." + RDSServiceName + ".c" + AWSCNEndpointSuffix)
	f.Add("a.cluster-b." + RDSServiceName + ".c" + AWSCNEndpointSuffix)
	f.Add("a.proxy-b." + RDSServiceName + ".c" + AWSCNEndpointSuffix)
	f.Add("a.b." + RDSServiceName + ".c" + AWSCNEndpointSuffix)
	f.Add("a.endpoint.proxy-c." + RDSServiceName + ".d" + AWSCNEndpointSuffix)

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseRDSEndpoint(endpoint)
		})
	})
}

func FuzzParseRedshiftEndpoint(f *testing.F) {
	f.Add("")
	f.Add(":123")
	f.Add("foo:123")
	f.Add("foo" + AWSCNEndpointSuffix)
	f.Add("redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com")
	f.Add("redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-1.amazonaws.com.cn")

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _, _ = ParseRedshiftEndpoint(endpoint)
		})
	})
}

func FuzzParseElastiCacheEndpoint(f *testing.F) {
	f.Add("")
	f.Add(":123")
	f.Add("foo:123")
	f.Add("foo" + AWSEndpointSuffix)
	f.Add("foo" + AWSCNEndpointSuffix)
	f.Add("clustercfg.b.c.usnw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("clustercfg.my-redis-shards.xxxxxx.use1.cache.foo:6379")
	f.Add("a.b.clustercfg.usnw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("my-redis-shards.xxxxxx.clustercfg.use1.cache.foo:6379")
	f.Add("a.b.0001.usnw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("my-redis-cluster-001.xxxxxx.0001.use0.cache.foo:6379")
	f.Add("my-redis-shards-0001-001.xxxxxx.0001.use0.cache.foo:6379")
	f.Add("master.b.c.usnw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("master.my-redis-cluster.xxxxxx.use1.cache.foo:6379")
	f.Add("replica.b.c.usnw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("replica.my-redis-cluster.xxxxxx.use1.cache.foo:6379")
	f.Add("a.b.c.usnw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("a.b.c.usne1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("a.b.c.usse1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("a.b.c.ussw1." + ElastiCacheServiceName + AWSEndpointSuffix)
	f.Add("my-redis-cluster.xxxxxx.ng.0001.use1.cache.foo:6379")
	f.Add("my-redis-cluster-ro.xxxxxx.ng.0001.use1.cache.foo:6379")

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseElastiCacheEndpoint(endpoint)
		})
	})
}

func FuzzParseElastiCacheServerlessEndpoint(f *testing.F) {
	f.Add("")
	f.Add(":123")
	f.Add("foo:123")
	f.Add("cache.b.c.usnw1.amazonaws.com")
	f.Add("a.b.c.d.amazonaws.com:6379")
	f.Add("a.serverless.c.cac1.cache.amazonaws.com:6379")
	f.Add("example-cache-abc123.serverless.cac1.cache.amazonaws.com:6379")
	f.Add("redis://example-cache-abc123.serverless.cac1.cache.amazonaws.com:6379")
	f.Add("://://example-cache-abc123.serverless.cac1.cache.amazonaws.com:6379")

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseElastiCacheServerlessEndpoint(endpoint)
		})
	})
}

func FuzzParseDynamoDBEndpoint(f *testing.F) {
	f.Add("")
	f.Add(":123")
	f.Add("foo:123")
	f.Add(DynamoDBServiceName + ".foo" + AWSEndpointSuffix)
	f.Add(DynamoDBServiceName + ".foo" + AWSEndpointSuffix + ":1234")
	f.Add(DynamoDBFipsServiceName + ".foo" + AWSCNEndpointSuffix)

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseDynamoDBEndpoint(endpoint)
		})
	})
}

func FuzzParseOpensearchEndpoint(f *testing.F) {
	f.Add("")
	f.Add(":123")
	f.Add("foo:123")
	f.Add("a.b." + OpenSearchServiceName + AWSEndpointSuffix)
	f.Add("a.b." + OpenSearchServiceName + AWSEndpointSuffix + ":1234")
	f.Add("a.b." + OpenSearchServiceName + AWSCNEndpointSuffix)

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseOpensearchEndpoint(endpoint)
		})
	})
}
