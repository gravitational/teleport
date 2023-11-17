/*
Copyright 2023 Gravitational, Inc.

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

package fixtures

const (
	// AWSAccountID is a sample AWS Account ID. It must be 12 digits.
	AWSAccountID = "123456789012"
	// AWSRegion is a sample AWS region.
	AWSRegion = "us-east-1"

	// AWSRedshiftURI is a sample URI for a Redshift cluster.
	AWSRedshiftURI = "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438"
	// AWSRedshiftServerlessURI is a sample URI for a Redshift Serverless workgroup.
	AWSRedshiftServerlessURI = "my-workgroup.123456789012.us-east-1.redshift-serverless.amazonaws.com:5439"
	// AWSRDSInstanceURI is a sample URI for a RDS PostgreSQL instance.
	AWSRDSInstanceURI = "instance.abcdefghijklmnop.us-east-1.rds.amazonaws.com:5432"
	// AWSRDSProxyURI is a sample URI for a RDS Proxy for RDS PostgreSQL.
	AWSRDSProxyURI = "my-proxy.proxy-abcdefghijklmnop.us-east-1.rds.amazonaws.com:5432"
	// AWSElastiCacheClusterURI is a sample URI for an ElastiCache for Redis (cluster endpoint).
	AWSElastiCacheClusterURI = "clustercfg.my-redis-cluster.xxxxxx.use1.cache.amazonaws.com:6379"
	// AWSMemoryDBURI is a sample URI for a MemoryDB cluster.
	AWSMemoryDBURI = "clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379"
)
