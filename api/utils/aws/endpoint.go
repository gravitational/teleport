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
	"net"
	"net/url"
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
	return isAWSServiceEndpoint(uri, RDSServiceName)
}

// IsRedshiftEndpoint returns true if the input URI is an Redshift endpoint.
//
// https://docs.aws.amazon.com/redshift/latest/mgmt/connecting-from-psql.html
func IsRedshiftEndpoint(uri string) bool {
	return isAWSServiceEndpoint(uri, RedshiftServiceName)
}

// IsRedshiftServerlessEndpoint returns true if the input URI is an Redshift
// Serverless endpoint.
//
// https://docs.aws.amazon.com/redshift/latest/mgmt/serverless-connecting.html
func IsRedshiftServerlessEndpoint(uri string) bool {
	return isAWSServiceEndpoint(uri, RedshiftServerlessServiceName)
}

// IsElastiCacheEndpoint returns true if the input URI is an ElastiCache
// endpoint.
func IsElastiCacheEndpoint(uri string) bool {
	return isAWSServiceEndpoint(uri, ElastiCacheServiceName)
}

// IsMemoryDBEndpoint returns true if the input URI is an MemoryDB
// endpoint.
func IsMemoryDBEndpoint(uri string) bool {
	return isAWSServiceEndpoint(uri, MemoryDBSServiceName)
}

// IsKeyspacesEndpoint returns true if input URI is an AWS Keyspaces endpoint.
// https://docs.aws.amazon.com/keyspaces/latest/devguide/programmatic.endpoints.html
func IsKeyspacesEndpoint(uri string) bool {
	hasCassandraPrefix := strings.HasPrefix(uri, "cassandra.") || strings.HasPrefix(uri, "cassandra-fips.")
	return hasCassandraPrefix && IsAWSEndpoint(uri)
}

// IsOpenSearchEndpoint returns true if input URI is an OpenSearch endpoint.
func IsOpenSearchEndpoint(uri string) bool {
	return isAWSServiceEndpoint(uri, OpenSearchServiceName)
}

// RDSEndpointDetails contains information about an RDS endpoint.
type RDSEndpointDetails struct {
	// InstanceID is the identifier of an RDS instance.
	InstanceID string
	// ClusterID is the identifier of an RDS Aurora cluster.
	ClusterID string
	// ClusterCustomEndpointName is the identifier of an Aurora cluster custom endpoint.
	ClusterCustomEndpointName string
	// ProxyName is the identifier of an RDS proxy.
	ProxyName string
	// ProxyCustomEndpointName is the identifier of an RDS proxy custom endpoint.
	ProxyCustomEndpointName string
	// Region is the AWS region the database resides in.
	Region string
	// EndpointType specifies the type of the endpoint, if available.
	//
	// Note that the endpoint type of RDS Proxies are determined by their
	// targets, so the endpoint type will be empty for RDS Proxies here as it
	// cannot be decided by the endpoint URL itself.
	EndpointType string
}

// IsProxy returns true if the RDS endpoint is an RDS Proxy.
func (d RDSEndpointDetails) IsProxy() bool {
	return d.ProxyName != "" || d.ProxyCustomEndpointName != ""
}

// ParseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint.
func ParseRDSEndpoint(endpoint string) (d *RDSEndpointDetails, err error) {
	if strings.ContainsRune(endpoint, ':') {
		endpoint, _, err = net.SplitHostPort(endpoint)
		if err != nil {
			return nil, trace.Wrap(err)
		}
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
func parseRDSEndpoint(endpoint string) (*RDSEndpointDetails, error) {
	parts := strings.Split(endpoint, ".")
	hasCorrectLen := len(parts) == 6 || len(parts) == 7
	serviceNameIndex := len(parts) - 3
	regionIndex := len(parts) - 4
	suffixStart := regionIndex

	if !strings.HasSuffix(endpoint, AWSEndpointSuffix) || !hasCorrectLen || parts[serviceNameIndex] != RDSServiceName {
		return nil, trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}

	details, err := parseRDSWithoutSuffixes(endpoint, parts[:suffixStart], parts[regionIndex])
	return details, trace.Wrap(err)
}

// parseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint for AWS China regions.
//
// RDS/Aurora endpoints look like this for AWS China regions:
// aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn
func parseRDSCNEndpoint(endpoint string) (*RDSEndpointDetails, error) {
	parts := strings.Split(endpoint, ".")
	hasCorrectLen := len(parts) == 7 || len(parts) == 8
	regionIndex := len(parts) - 4
	serviceNameIndex := len(parts) - 5
	suffixStart := serviceNameIndex

	if !strings.HasSuffix(endpoint, AWSCNEndpointSuffix) || !hasCorrectLen || parts[serviceNameIndex] != RDSServiceName {
		return nil, trace.BadParameter("failed to parse %v as RDS CN endpoint", endpoint)
	}

	details, err := parseRDSWithoutSuffixes(endpoint, parts[:suffixStart], parts[regionIndex])
	return details, trace.Wrap(err)
}

// parseRDSWithoutSuffixes extracts identifiers from provided parts and returns
// RDSEndpointDetails. It is expected that the provided parts has either:
// - two parts (e.g. aurora-instance-1.abcdefghijklmnop)
// - or three parts (e.g. my-proxy-custom.endpoint.proxy-abcdefghijklmnop)
// as region/service/partition suffixes are removed by the caller.
func parseRDSWithoutSuffixes(endpoint string, parts []string, region string) (*RDSEndpointDetails, error) {
	// RDS/Aurora instance endpoints look like this:
	// aurora-instance-1.abcdefghijklmnop.<suffixes>
	//
	// Aurora cluster endpoints look like this:
	// my-cluster.cluster-abcdefghijklmnop.<suffixes>
	// my-cluster.cluster-ro-abcdefghijklmnop.<suffixes>
	// my-custom.cluster-custom-abcdefghijklmnop.<suffixes>
	//
	// RDS Proxy "default" endpoints look like this:
	// my-proxy.proxy-abcdefghijklmnop.<suffixes>
	//
	// RDS Proxy custom endpoints look like this:
	// my-proxy-custom.endpoint.proxy-abcdefghijklmnop.<suffixes>
	//
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/rds-proxy-setup.html#rds-proxy-connecting
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/rds-proxy-endpoints.html
	switch len(parts) {
	case 2:
		switch {
		case strings.HasPrefix(parts[1], "cluster-custom-"):
			// Note that we are not able to get the cluster ID from the cluster
			// custom endpoints. The cluster ID must be provided separately in
			// addition to the endpoints.
			return &RDSEndpointDetails{
				ClusterCustomEndpointName: parts[0],
				Region:                    region,
				EndpointType:              RDSEndpointTypeCustom,
			}, nil

		case strings.HasPrefix(parts[1], "cluster-ro-"):
			return &RDSEndpointDetails{
				ClusterID:    parts[0],
				Region:       region,
				EndpointType: RDSEndpointTypeReader,
			}, nil

		case strings.HasPrefix(parts[1], "cluster-"):
			return &RDSEndpointDetails{
				ClusterID:    parts[0],
				Region:       region,
				EndpointType: RDSEndpointTypePrimary,
			}, nil

		case strings.HasPrefix(parts[1], "proxy-"):
			return &RDSEndpointDetails{
				ProxyName: parts[0],
				Region:    region,
			}, nil

		default:
			return &RDSEndpointDetails{
				InstanceID:   parts[0],
				Region:       region,
				EndpointType: RDSEndpointTypeInstance,
			}, nil
		}

	case 3:
		if strings.HasPrefix(parts[2], "proxy-") && parts[1] == "endpoint" {
			return &RDSEndpointDetails{
				ProxyCustomEndpointName: parts[0],
				Region:                  region,
			}, nil
		}
		return nil, trace.BadParameter("failed to parse %v as RDS Proxy custom endpoint", endpoint)

	default:
		return nil, trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}
}

// ParseRedshiftEndpoint extracts cluster ID and region from the provided
// Redshift endpoint.
func ParseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	if strings.ContainsRune(endpoint, ':') {
		endpoint, _, err = net.SplitHostPort(endpoint)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
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

// RedshiftServerlessEndpointDetails contains information about an Redshift
// Serverless endpoint.
type RedshiftServerlessEndpointDetails struct {
	// WorkgroupName is the name of the workgroup.
	WorkgroupName string
	// EndpointName is the name of the VPC endpoint.
	EndpointName string
	// AccountID is the AWS Account ID.
	AccountID string
	// Region is the AWS region the database resides in.
	Region string
}

// ParseRedshiftServerlessEndpoint extracts name, AWS Account ID, and region
// from the provided Redshift Serverless endpoint.
func ParseRedshiftServerlessEndpoint(endpoint string) (details *RedshiftServerlessEndpointDetails, err error) {
	if strings.ContainsRune(endpoint, ':') {
		endpoint, _, err = net.SplitHostPort(endpoint)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		// TODO(greedy52) add AWS China support when Redshift Serverless come to those regions.
		return nil, trace.NotImplemented("failed to parse %v as Redshift Serverless endpoint: AWS China regions are not supported yet", endpoint)
	}
	return parseRedshiftServerlessEndpoint(endpoint)
}

// parseRedshiftServerlessEndpoint extracts name, AWS account ID, and region
// from the provided Redshift Serverless endpoint for standard regions.
//
// Workgroup endpoint looks like this:
// <workgroup-name>.<account-id>.<region>.redshift-serverless.amazonaws.com
//
// VPC endpoint looks like this:
// <vpc-endpoint-name>-endpoint-<some-hash>.<account-id>.<region>.redshift-serverless.amazonaws.com
func parseRedshiftServerlessEndpoint(endpoint string) (*RedshiftServerlessEndpointDetails, error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSEndpointSuffix) || len(parts) != 6 || parts[3] != RedshiftServerlessServiceName {
		return nil, trace.BadParameter("failed to parse %v as Redshift Serverless endpoint", endpoint)
	}
	if endpointName, _, found := strings.Cut(parts[0], "-endpoint-"); found {
		return &RedshiftServerlessEndpointDetails{
			EndpointName: endpointName,
			AccountID:    parts[1],
			Region:       parts[2],
		}, nil
	}

	return &RedshiftServerlessEndpointDetails{
		WorkgroupName: parts[0],
		AccountID:     parts[1],
		Region:        parts[2],
	}, nil
}

// RedisEndpointInfo describes details extracted from a ElastiCache or MemoryDB
// Redis endpoint.
type RedisEndpointInfo struct {
	// ID is the identifier of the endpoint.
	ID string
	// Region is the AWS region for the endpoint.
	Region string
	// TransitEncryptionEnabled specifies if in-transit encryption (TLS) is
	// enabled.
	TransitEncryptionEnabled bool
	// EndpointType specifies the type of the endpoint.
	EndpointType string
}

const (
	// ElastiCacheConfigurationEndpoint is the configuration endpoint that used
	// for cluster mode connection.
	ElastiCacheConfigurationEndpoint = "configuration"
	// ElastiCachePrimaryEndpoint is the endpoint of the primary node in the
	// node group.
	ElastiCachePrimaryEndpoint = "primary"
	// ElastiCacheReaderEndpoint is the endpoint of the replica nodes in the
	// node group.
	ElastiCacheReaderEndpoint = "reader"
	// ElastiCacheNodeEndpoint is the endpoint that used to connect to an
	// individual node.
	ElastiCacheNodeEndpoint = "node"

	// MemoryDBClusterEndpoint is the cluster configuration endpoint for a
	// MemoryDB cluster.
	MemoryDBClusterEndpoint = "cluster"
	// MemoryDBNodeEndpoint is the endpoint of an individual MemoryDB node.
	MemoryDBNodeEndpoint = "node"

	// OpenSearchDefaultEndpoint is the default endpoint for domain.
	OpenSearchDefaultEndpoint = "default"
	// OpenSearchCustomEndpoint is the custom endpoint configured for domain.
	OpenSearchCustomEndpoint = "custom"
	// OpenSearchVPCEndpoint is the VPC endpoint for domain.
	OpenSearchVPCEndpoint = "vpc"

	// RDSEndpointTypePrimary is the endpoint that specifies the connection for
	// the primary instance of the RDS cluster.
	RDSEndpointTypePrimary = "primary"
	// RDSEndpointTypeReader is the endpoint that load-balances connections
	// across the Aurora Replicas that are available in an RDS cluster.
	RDSEndpointTypeReader = "reader"
	// RDSEndpointTypeCustom is the endpoint that specifies one of the custom
	// endpoints associated with the RDS cluster.
	RDSEndpointTypeCustom = "custom"
	// RDSEndpointTypeInstance is the endpoint of an RDS DB instance.
	RDSEndpointTypeInstance = "instance"
)

// ParseElastiCacheEndpoint extracts the details from the provided
// ElastiCache Redis endpoint.
//
// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/GettingStarted.ConnectToCacheNode.html
func ParseElastiCacheEndpoint(endpoint string) (*RedisEndpointInfo, error) {
	endpoint, err := removeSchemaAndPort(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove partition suffix. Note that endpoints for CN regions use the same
	// format except they end with AWSCNEndpointSuffix.
	endpointWithoutSuffix, _, err := removePartitionSuffix(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Split into parts to extract details. They look like this in general:
	//   <part>.<part>.<part>.<short-region>.cache
	//
	// Note that ElastiCache uses short region codes like "use1".
	//
	// For Redis with cluster mode enabled, users can connect through either
	// "configuration" endpoint or individual "node" endpoints.
	// For Redis with cluster mode disabled, users can connect through either
	// "primary", "reader", or individual "node" endpoints.
	parts := strings.Split(endpointWithoutSuffix, ".")
	if len(parts) == 5 && parts[4] == ElastiCacheServiceName {
		region, ok := ShortRegionToRegion(parts[3])
		if !ok {
			return nil, trace.BadParameter("%v is not a valid region", parts[3])
		}

		// Configuration endpoint for Redis with TLS enabled looks like:
		// clustercfg.my-redis-shards.xxxxxx.use1.cache.<suffix>:6379
		if parts[0] == "clustercfg" {
			return &RedisEndpointInfo{
				ID:                       parts[1],
				Region:                   region,
				TransitEncryptionEnabled: true,
				EndpointType:             ElastiCacheConfigurationEndpoint,
			}, nil
		}

		// Configuration endpoint for Redis with TLS disabled looks like:
		// my-redis-shards.xxxxxx.clustercfg.use1.cache.<suffix>:6379
		if parts[2] == "clustercfg" {
			return &RedisEndpointInfo{
				ID:                       parts[0],
				Region:                   region,
				TransitEncryptionEnabled: false,
				EndpointType:             ElastiCacheConfigurationEndpoint,
			}, nil
		}

		// Node endpoint for Redis with TLS disabled looks like:
		// my-redis-cluster-001.xxxxxx.0001.use0.cache.<suffix>:6379
		// my-redis-shards-0001-001.xxxxxx.0001.use0.cache.<suffix>:6379
		if isElasticCacheShardID(parts[2]) {
			return &RedisEndpointInfo{
				ID:                       trimElastiCacheShardAndNodeID(parts[0]),
				Region:                   region,
				TransitEncryptionEnabled: false,
				EndpointType:             ElastiCacheNodeEndpoint,
			}, nil
		}

		// Node, primary, reader endpoints for Redis with TLS enabled look like:
		// my-redis-cluster-001.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		// my-redis-shards-0001-001.my-redis-shards.xxxxxx.use1.cache.<suffix>:6379
		// master.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		// replica.my-redis-cluster.xxxxxx.use1.cache.<suffix>:6379
		var endpointType string
		switch strings.ToLower(parts[0]) {
		case "master":
			endpointType = ElastiCachePrimaryEndpoint
		case "replica":
			endpointType = ElastiCacheReaderEndpoint
		default:
			endpointType = ElastiCacheNodeEndpoint
		}
		return &RedisEndpointInfo{
			ID:                       parts[1],
			Region:                   region,
			TransitEncryptionEnabled: true,
			EndpointType:             endpointType,
		}, nil
	}

	// Primary and reader endpoints for Redis with TLS disabled have an extra
	// shard ID in the endpoints, and they look like:
	// my-redis-cluster.xxxxxx.ng.0001.use1.cache.<suffix>:6379
	// my-redis-cluster-ro.xxxxxx.ng.0001.use1.cache.<suffix>:6379
	if len(parts) == 6 && parts[5] == ElastiCacheServiceName && isElasticCacheShardID(parts[3]) {
		region, ok := ShortRegionToRegion(parts[4])
		if !ok {
			return nil, trace.BadParameter("%v is not a valid region", parts[4])
		}

		// Remove "-ro" from reader endpoint.
		if strings.HasSuffix(parts[0], "-ro") {
			return &RedisEndpointInfo{
				ID:                       strings.TrimSuffix(parts[0], "-ro"),
				Region:                   region,
				TransitEncryptionEnabled: false,
				EndpointType:             ElastiCacheReaderEndpoint,
			}, nil
		}

		return &RedisEndpointInfo{
			ID:                       parts[0],
			Region:                   region,
			TransitEncryptionEnabled: false,
			EndpointType:             ElastiCachePrimaryEndpoint,
		}, nil
	}

	return nil, trace.BadParameter("unknown ElastiCache Redis endpoint format %q", endpoint)
}

// isElasticCacheShardID returns true if the input part is in shard ID format.
// The shard ID is a 4 character string of an integer left padded with zeros
// (e.g. 0001).
func isElasticCacheShardID(part string) bool {
	if len(part) != 4 {
		return false
	}
	_, err := strconv.Atoi(part)
	return err == nil
}

// isElasticCacheNodeID returns true if the input part is in node ID format.
// The node ID is a 3 character string of an integer left padded with zeros
// (e.g. 001).
func isElasticCacheNodeID(part string) bool {
	if len(part) != 3 {
		return false
	}
	_, err := strconv.Atoi(part)
	return err == nil
}

// trimElastiCacheShardAndNodeID trims shard and node ID suffix from input.
func trimElastiCacheShardAndNodeID(input string) string {
	// input can be one of:
	// <replication-group-id>
	// <replication-group-id>-<node-id>
	// <replication-group-id>-<shard-id>-<node-id>
	parts := strings.Split(input, "-")
	if len(parts) > 0 {
		if isElasticCacheNodeID(parts[len(parts)-1]) {
			parts = parts[:len(parts)-1]
		}
	}
	if len(parts) > 0 {
		if isElasticCacheShardID(parts[len(parts)-1]) {
			parts = parts[:len(parts)-1]
		}
	}
	return strings.Join(parts, "-")
}

// ParseMemoryDBEndpoint extracts the details from the provided
// MemoryDB endpoint.
//
// https://docs.aws.amazon.com/memorydb/latest/devguide/endpoints.html
func ParseMemoryDBEndpoint(endpoint string) (*RedisEndpointInfo, error) {
	endpoint, err := removeSchemaAndPort(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Here is a sample endpoint for MemoryDB:
	// clustercfg.my-memorydb.scwzlu.memorydb.ca-central-1.amazonaws.com
	//
	// Unlike RDS/Redshift endpoints, the service subdomain is before region.
	// Unlike ElastiCache endpoints, MemoryDB uses full region name.
	endpointWithoutSuffix, _, err := removePartitionSuffix(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parts := strings.Split(endpointWithoutSuffix, ".")
	if len(parts) != 5 || parts[3] != MemoryDBSServiceName {
		return nil, trace.BadParameter("unknown MemoryDB endpoint format")
	}

	switch {
	// TLS disabled cluster endpoints look like this:
	// <cluster-name>.<xxxx>.clustercfg.memorydb.<region>.<suffix>
	case parts[2] == "clustercfg":
		return &RedisEndpointInfo{
			ID:                       parts[0],
			Region:                   parts[4],
			TransitEncryptionEnabled: false,
			EndpointType:             MemoryDBClusterEndpoint,
		}, nil

	// TLS enabled cluster endpoints look like this:
	// clustercfg.<cluster-name>.<xxxx>.memorydb.<region>.<suffix>
	case parts[0] == "clustercfg":
		return &RedisEndpointInfo{
			ID:                       parts[1],
			Region:                   parts[4],
			TransitEncryptionEnabled: true,
			EndpointType:             MemoryDBClusterEndpoint,
		}, nil

	// TLS disabled node endpoints look like this:
	// <cluster-name>-<shard-id>-<node-id>.<xxxx>.<shard-id>.memorydb.<region>.<suffix>
	//
	// MemoryDB and ElastiCache share same shard/node ID format.
	case isElasticCacheShardID(parts[2]):
		return &RedisEndpointInfo{
			ID:                       trimElastiCacheShardAndNodeID(parts[0]),
			Region:                   parts[4],
			TransitEncryptionEnabled: false,
			EndpointType:             MemoryDBNodeEndpoint,
		}, nil

	// TLS enabled node endpoints look like this:
	// <cluster-name>-<shard-id>-<node-id>.<cluster-name>.<xxxx>.memorydb.<region>.<suffix>
	default:
		return &RedisEndpointInfo{
			ID:                       trimElastiCacheShardAndNodeID(parts[0]),
			Region:                   parts[4],
			TransitEncryptionEnabled: true,
			EndpointType:             MemoryDBNodeEndpoint,
		}, nil
	}
}

// isAWSServiceEndpoint returns true if uri is a valid AWS endpoint and uri
// contains the provided service name as a subdomain.
func isAWSServiceEndpoint(uri, serviceName string) bool {
	return strings.Contains(uri, fmt.Sprintf(".%s.", serviceName)) &&
		IsAWSEndpoint(uri)
}

func removeSchemaAndPort(endpoint string) (string, error) {
	// Add a temporary schema to make a valid URL for url.Parse.
	if !strings.Contains(endpoint, "://") {
		endpoint = "schema://" + endpoint
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return parsedURL.Hostname(), nil
}

func removePartitionSuffix(endpoint string) (string, string, error) {
	switch {
	case strings.HasSuffix(endpoint, AWSEndpointSuffix):
		return strings.TrimSuffix(endpoint, AWSEndpointSuffix), AWSEndpointSuffix, nil

	case strings.HasSuffix(endpoint, AWSCNEndpointSuffix):
		return strings.TrimSuffix(endpoint, AWSCNEndpointSuffix), AWSCNEndpointSuffix, nil

	default:
		return "", "", trace.BadParameter("%v is not a valid AWS endpoint", endpoint)
	}
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

	// RedshiftServerlessServiceName is the service name for AWS Redshift Serverless.
	RedshiftServerlessServiceName = "redshift-serverless"

	// ElastiCacheServiceName is the service name for AWS ElastiCache.
	ElastiCacheServiceName = "cache"

	// MemoryDBSServiceName is the service name for AWS MemoryDB.
	MemoryDBSServiceName = "memorydb"

	// DynamoDBServiceName is the service name for AWS DynamoDB.
	DynamoDBServiceName = "dynamodb"
	// DynamoDBFipsServiceName is the fips variant service name for AWS DynamoDB.
	DynamoDBFipsServiceName = "dynamodb-fips"
	// DynamoDBStreamsServiceName is the AWS DynamoDB Streams service name.
	DynamoDBStreamsServiceName = "streams.dynamodb"
	// DAXServiceName is the AWS DynamoDB Accelerator service name.
	DAXServiceName = "dax"

	// OpenSearchServiceName is the AWS OpenSearch service name.
	OpenSearchServiceName = "es"
)

// CassandraEndpointURLForRegion returns a Cassandra endpoint based on the provided region.
// https://docs.aws.amazon.com/keyspaces/latest/devguide/programmatic.endpoints.html
func CassandraEndpointURLForRegion(region string) string {
	if IsCNRegion(region) {
		return fmt.Sprintf("cassandra.%s%s:9142", region, AWSCNEndpointSuffix)
	}
	return fmt.Sprintf("cassandra.%s%s:9142", region, AWSEndpointSuffix)
}

// CassandraEndpointRegion returns an AWS region from cassandra endpoint:
// where endpoint looks like cassandra.us-east-2.amazonaws.com
// https://docs.aws.amazon.com/keyspaces/latest/devguide/programmatic.endpoints.html
func CassandraEndpointRegion(endpoint string) (string, error) {
	parts, _, err := extractAWSEndpointParts(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(parts) != 2 {
		return "", trace.BadParameter("invalid Cassandra endpoint")
	}
	return parts[1], nil
}

// DynamoDBEndpointInfo describes info extracted from a DynamoDB endpoint.
type DynamoDBEndpointInfo struct {
	// Service is the service subdomain of the endpoint, for example "dynamodb" or "dax".
	Service string
	// Region is the AWS region for the endpoint, for example "us-west-1".
	Region string
	// Partition is the AWS partition for the endpoint, for example ".amazonaws.com"
	Partition string
}

// ParseDynamoDBEndpoint parses and extract info from the provided DynamoDB endpoint.
func ParseDynamoDBEndpoint(endpoint string) (*DynamoDBEndpointInfo, error) {
	endpoint = strings.ToLower(endpoint)
	parts, partition, err := extractAWSEndpointParts(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch len(parts) {
	case 2, 3:
	default:
		return nil, trace.BadParameter("invalid DynamoDB endpoint %q", endpoint)
	}
	info := &DynamoDBEndpointInfo{
		Service:   strings.Join(parts[:len(parts)-1], "."),
		Region:    parts[len(parts)-1],
		Partition: partition,
	}

	// check for recognized service name.
	switch info.Service {
	case DynamoDBServiceName, DynamoDBFipsServiceName,
		DynamoDBStreamsServiceName, DAXServiceName:
	default:
		return nil, trace.BadParameter("invalid DynamoDB endpoint %q", endpoint)
	}

	// check that the partition is valid for the region.
	if info.Region == "" || info.Partition == "" {
		return nil, trace.BadParameter("invalid DynamoDB endpoint %q", endpoint)
	}
	switch {
	case info.Partition == AWSCNEndpointSuffix && IsCNRegion(info.Region):
	case info.Partition == AWSEndpointSuffix && !IsCNRegion(info.Region):
	default:
		return nil, trace.BadParameter("invalid AWS region %q for AWS partition %q",
			info.Region, info.Partition)
	}
	return info, nil
}

// OpenSearchEndpointInfo describes info extracted from an AWS endpoint.
type OpenSearchEndpointInfo struct {
	// Service is the service subdomain of the endpoint. Only "es" allowed for now.
	Service string
	// Region is the AWS region for the endpoint, for example "us-west-1".
	Region string
	// Partition is the AWS partition for the endpoint, for example ".amazonaws.com"
	Partition string
}

// ParseOpensearchEndpoint parses and extract info from the provided OpenSearch endpoint.
func ParseOpensearchEndpoint(endpoint string) (*OpenSearchEndpointInfo, error) {
	endpoint = strings.ToLower(endpoint)
	parts, partition, err := extractAWSEndpointParts(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(parts) != 3 {
		return nil, trace.BadParameter("invalid OpenSearch endpoint %q, wrong number of parts %v", endpoint, len(parts))
	}

	info := &OpenSearchEndpointInfo{
		Region:    parts[len(parts)-2],
		Service:   parts[len(parts)-1],
		Partition: partition,
	}

	// check for recognized service name.
	if info.Service != OpenSearchServiceName {
		return nil, trace.BadParameter("invalid OpenSearch endpoint %q, invalid service %q", endpoint, info.Service)
	}

	// check that the partition is valid for the region.
	switch {
	case info.Region == "" || info.Partition == "":
		return nil, trace.BadParameter("invalid OpenSearch endpoint %q, empty partition and region", endpoint)
	case info.Region == "":
		return nil, trace.BadParameter("invalid OpenSearch endpoint %q, empty region", endpoint)
	case info.Partition == "":
		return nil, trace.BadParameter("invalid OpenSearch endpoint %q, empty partition", endpoint)
	}

	switch {
	case info.Partition == AWSCNEndpointSuffix && IsCNRegion(info.Region):
	case info.Partition == AWSEndpointSuffix && !IsCNRegion(info.Region):
	default:
		return nil, trace.BadParameter("invalid AWS region %q for AWS partition %q",
			info.Region, info.Partition)
	}
	return info, nil
}

// DynamoDBURIForRegion constructs a DynamoDB URI based on the AWS region.
// The URI uses a custom schema aws:// to differentiate an auto-generated URI from
// a user-configured URI in the engine.
// When the Teleport DynamoDB engine sees this custom URI schema, it will resolve
// the real endpoint using the request API target.
// https://docs.aws.amazon.com/general/latest/gr/ddb.html
func DynamoDBURIForRegion(region string) string {
	var suffix string
	if IsCNRegion(region) {
		suffix = AWSCNEndpointSuffix
	} else {
		suffix = AWSEndpointSuffix
	}
	return fmt.Sprintf("aws://dynamodb.%s%s", region, suffix)
}

// extractAWSEndpointParts strips the schema, port, and AWS suffix,
// then splits the prefix by subdomain separator (".") and returns the parts and suffix.
func extractAWSEndpointParts(endpoint string) ([]string, string, error) {
	uri, err := removeSchemaAndPort(endpoint)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	prefix, suffix, err := removePartitionSuffix(uri)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return strings.Split(prefix, "."), suffix, nil
}
