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
	"net"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// IsAWSEndpoint returns true if the input URI is an AWS endpoint.
func IsAWSEndpoint(uri string) bool {
	// Note that AWSCNEndpointSuffix contains AWSEndpointSuffix so there is no
	// need to search for AWSCNEndpointSuffix explicitly.
	return strings.Contains(uri, AWSEndpointSuffix)
}

// IsRDSEndpoint returns true if the input URI is an RDS endpoint.
//
// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html
func IsRDSEndpoint(uri string) bool {
	return strings.Contains(uri, RDSEndpointSubdomain) &&
		IsAWSEndpoint(uri)
}

// IsRedshiftEndpoint returns true if the input URI is an Redshift endpoint.
//
// https://docs.aws.amazon.com/redshift/latest/mgmt/connecting-from-psql.html
func IsRedshiftEndpoint(uri string) bool {
	return strings.Contains(uri, RedshiftEndpointSubdomain) &&
		IsAWSEndpoint(uri)
}

// IsElastiCacheEndpoint returns true if the input URI is an ElastiCache
// endpoint.
func IsElastiCacheEndpoint(uri string) bool {
	return strings.Contains(uri, ElastiCacheSubdomain) &&
		IsAWSEndpoint(uri)
}

// ParseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint.
func ParseRDSEndpoint(endpoint string) (id, region string, err error) {
	endpoint, err = trimEndpointPort(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		return parseRDSCNEndpoint(endpoint)
	}
	return parseRDSEndpoint(endpoint)
}

// parseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint for standard regions.
//
// RDS/Aurora endpoints look like this:
// aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com
func parseRDSEndpoint(endpoint string) (id, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSEndpointSuffix) || len(parts) != 6 || parts[3] != RDSServiceName {
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}
	return parts[0], parts[2], nil
}

// parseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint for AWS China regions.
//
// RDS/Aurora endpoints look like this for AWS China regions:
// aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn
func parseRDSCNEndpoint(endpoint string) (id, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSCNEndpointSuffix) || len(parts) != 7 || parts[2] != RDSServiceName {
		return "", "", trace.BadParameter("failed to parse %v as RDS CN endpoint", endpoint)
	}
	return parts[0], parts[3], nil
}

// ParseRedshiftEndpoint extracts cluster ID and region from the provided
// Redshift endpoint.
func ParseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	endpoint, err = trimEndpointPort(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		return parseRedshiftCNEndpoint(endpoint)
	}
	return parseRedshiftEndpoint(endpoint)
}

// parseRedshiftEndpoint extracts cluster ID and region from the provided
// Redshift endpoint for standard regions.
//
// Redshift endpoints look like this:
// redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com
func parseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSEndpointSuffix) || len(parts) != 6 || parts[3] != RedshiftServiceName {
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}
	return parts[0], parts[2], nil
}

// parseRedshiftCNEndpoint extracts cluster ID and region from the provided
// Redshift endpoint for AWS China regions.
//
// Redshift endpoints look like this for AWS China regions:
// redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-1.amazonaws.com.cn
func parseRedshiftCNEndpoint(endpoint string) (clusterID, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSCNEndpointSuffix) || len(parts) != 7 || parts[2] != RedshiftServiceName {
		return "", "", trace.BadParameter("failed to parse %v as Redshift CN endpoint", endpoint)
	}
	return parts[0], parts[3], nil
}

// TODO
type RedisEndpointInfo struct {
	ClusterName    string
	Region         string
	TLSEnabled     bool
	ClusterEnabled bool
}

// TODO
//
// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/GettingStarted.ConnectToCacheNode.html
func ParseElastiCacheRedisEndpoint(endpoint string) (*RedisEndpointInfo, error) {
	endpoint, err := trimEndpointPort(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var endpointWithoutSuffix string
	switch {
	case strings.HasPrefix(endpoint, AWSEndpointSuffix):
		endpointWithoutSuffix = strings.TrimSuffix(endpoint, AWSEndpointSuffix)

	case strings.HasSuffix(endpoint, AWSCNEndpointSuffix):
		// Endpoints for CN regions look like this:
		// my-redis-cluster.xxxxxx.0001.cnn1.cache.amazonaws.com.cn:6379
		endpointWithoutSuffix = strings.TrimSuffix(endpoint, AWSCNEndpointSuffix)

	default:
		return nil, trace.BadParameter("failed to parse %v as ElastiCache Redis endpoint", endpoint)
	}

	// Parts should end with service name without partition suffix.
	parts := strings.Split(endpointWithoutSuffix, ".")

	// For <part>.<part>.<part>.<short-region>.cache.
	//
	// Note that ElastiCache uses short region codes (e.g. "use1" for "us-east-1").
	//
	// For Redis with Cluster mode enabled, there is a single "configuration"
	// endpoint. For Redis with Cluster mode disabled, users can connect
	// through either "primary", "reader", or "node" endpoints.
	if len(parts) == 5 && parts[4] == ElastiCacheServiceName {
		region, ok := ShortRegionToRegion(parts[3])
		if !ok {
			return nil, trace.BadParameter("failed to parse %v as ElastiCache Redis endpoint: %v is not a valid region", endpoint, parts[3])
		}

		// Configuration endpoint for Redis with TLS enabled looks like:
		// clustercfg.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		if parts[0] == "clustercfg" {
			return &RedisEndpointInfo{
				ClusterName:    parts[1],
				Region:         region,
				TLSEnabled:     true,
				ClusterEnabled: true,
			}, nil
		}

		// Configuration endpoint for Redis with TLS disabled looks like:
		// my-redis-cluster.xxxxxx.clustercfg.use1.cache.<suffix>:6379
		if parts[2] == "clustercfg" {
			return &RedisEndpointInfo{
				ClusterName:    parts[0],
				Region:         region,
				TLSEnabled:     false,
				ClusterEnabled: true,
			}, nil
		}

		// Node endpoint for Redis with TLS disabled looks like:
		// my-redis-cluster-001.xxxxxx.0001.use0.cache.<suffix>:6379
		if isElasticCacheShardID(parts[2]) {
			return &RedisEndpointInfo{
				ClusterName:    trimElastiCacheNodeID(parts[0]),
				Region:         region,
				TLSEnabled:     false,
				ClusterEnabled: false,
			}, nil
		}

		// Node, primary, reader endpoints for Redis with TLS enabled look like:
		// my-redis-cluster-001.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		// master.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		// replica.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		return &RedisEndpointInfo{
			ClusterName:    parts[1],
			Region:         region,
			TLSEnabled:     true,
			ClusterEnabled: false,
		}, nil
	}

	// Primary and reader endpoints for Redis with TLS disabled look like:
	// my-redis.xxxxxx.ng.0001.use1.cache.<suffix>:6379
	// my-redis-ro.xxxxxx.ng.0001.use1.cache.<suffix>:6379
	if len(parts) == 6 && parts[5] == ElastiCacheServiceName && isElasticCacheShardID(parts[3]) {
		region, ok := ShortRegionToRegion(parts[4])
		if !ok {
			return nil, trace.BadParameter("failed to parse %v as ElastiCache Redis endpoint: %v is not a valid region", endpoint, parts[4])
		}

		// Remove "-ro" if exist.
		clusterName := strings.TrimSuffix(parts[0], "-ro")

		return &RedisEndpointInfo{
			ClusterName:    clusterName,
			Region:         region,
			TLSEnabled:     false,
			ClusterEnabled: false,
		}, nil
	}

	return nil, trace.BadParameter("failed to parse %v as ElastiCache Redis endpoint", endpoint)
}

// TODO
func isElasticCacheShardID(part string) bool {
	if len(part) != 4 {
		return false
	}
	_, err := strconv.Atoi(part)
	return err == nil
}

// TODO
func isElasticCacheNodeID(part string) bool {
	if len(part) != 3 {
		return false
	}
	_, err := strconv.Atoi(part)
	return err == nil
}

// TODO
func trimElastiCacheNodeID(name string) string {
	parts := strings.Split(name, "-")
	lastPart := parts[len(parts)-1]
	if isElasticCacheNodeID(lastPart) {
		return strings.TrimSuffix(name, "-"+lastPart)
	}
	return name
}

// TODO
func trimEndpointPort(endpoint string) (string, error) {
	if !strings.ContainsRune(endpoint, ':') {
		return endpoint, nil
	}

	endpoint, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return endpoint, nil
}

const (
	// AWSEndpointSuffix is the endpoint suffix for AWS Standard and AWS US
	// GovCloud regions.
	//
	// https://docs.aws.amazon.com/general/latest/gr/rande.html#regional-endpoints
	// https://docs.aws.amazon.com/govcloud-us/latest/UserGuide/using-govcloud-endpoints.html
	AWSEndpointSuffix = ".amazonaws.com"

	// AWSCNEndpointSuffix is the endpoint suffix for AWS China regions.
	//
	// https://docs.amazonaws.cn/en_us/aws/latest/userguide/endpoints-arns.html
	AWSCNEndpointSuffix = ".amazonaws.com.cn"

	// RDSServiceName is the service name for AWS RDS.
	RDSServiceName = "rds"

	// RedshiftServiceName is the service name for AWS Redshift.
	RedshiftServiceName = "redshift"

	// ElastiCacheServiceName is the service name for AWS ElastiCache.
	ElastiCacheServiceName = "cache"

	// RDSEndpointSubdomain is the RDS/Aurora subdomain.
	RDSEndpointSubdomain = "." + RDSServiceName + "."

	// RedshiftEndpointSubdomain is the Redshift endpoint subdomain.
	RedshiftEndpointSubdomain = "." + RedshiftServiceName + "."

	// ElastiCacheSubdomain is the ElastiCache endpoint subdomain.
	ElastiCacheSubdomain = "." + ElastiCacheServiceName + "."
)
