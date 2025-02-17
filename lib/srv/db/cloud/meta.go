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
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	memorydb "github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	opensearch "github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/db/common"
	discoverycommon "github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// elasticacheClient defines a subset of the AWS ElastiCache client API.
type elasticacheClient interface {
	elasticache.DescribeReplicationGroupsAPIClient
}

// iamClient defines a subset of the AWS IAM client API.
type iamClient interface {
	DeleteRolePolicy(context.Context, *iam.DeleteRolePolicyInput, ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error)
	DeleteUserPolicy(context.Context, *iam.DeleteUserPolicyInput, ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error)
	GetRolePolicy(context.Context, *iam.GetRolePolicyInput, ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error)
	GetUserPolicy(context.Context, *iam.GetUserPolicyInput, ...func(*iam.Options)) (*iam.GetUserPolicyOutput, error)
	PutRolePolicy(context.Context, *iam.PutRolePolicyInput, ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
	PutUserPolicy(context.Context, *iam.PutUserPolicyInput, ...func(*iam.Options)) (*iam.PutUserPolicyOutput, error)
}

// memoryDBClient defines a subset of the AWS MemoryDB client API.
type memoryDBClient interface {
	memorydb.DescribeClustersAPIClient
}

// openSearchClient defines a subset of the AWS OpenSearch client API.
type openSearchClient interface {
	DescribeDomains(context.Context, *opensearch.DescribeDomainsInput, ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error)
}

// rdsClient defines a subset of the AWS RDS client API.
type rdsClient interface {
	rds.DescribeDBClustersAPIClient
	rds.DescribeDBInstancesAPIClient
	rds.DescribeDBProxiesAPIClient
	rds.DescribeDBProxyEndpointsAPIClient
	ModifyDBCluster(ctx context.Context, params *rds.ModifyDBClusterInput, optFns ...func(*rds.Options)) (*rds.ModifyDBClusterOutput, error)
	ModifyDBInstance(ctx context.Context, params *rds.ModifyDBInstanceInput, optFns ...func(*rds.Options)) (*rds.ModifyDBInstanceOutput, error)
}

// redshiftClient defines a subset of the AWS Redshift client API.
type redshiftClient interface {
	redshift.DescribeClustersAPIClient
}

// rssClient defines a subset of the AWS Redshift Serverless client API.
type rssClient interface {
	GetEndpointAccess(ctx context.Context, params *rss.GetEndpointAccessInput, optFns ...func(*rss.Options)) (*rss.GetEndpointAccessOutput, error)
	GetWorkgroup(ctx context.Context, params *rss.GetWorkgroupInput, optFns ...func(*rss.Options)) (*rss.GetWorkgroupOutput, error)
}

// stsClient defines a subset of the AWS STS client API.
type stsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// awsClientProvider is an AWS SDK client provider.
type awsClientProvider interface {
	getElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) elasticacheClient
	getIAMClient(cfg aws.Config, optFns ...func(*iam.Options)) iamClient
	getMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) memoryDBClient
	getOpenSearchClient(cfg aws.Config, optFns ...func(*opensearch.Options)) openSearchClient
	getRDSClient(cfg aws.Config, optFns ...func(*rds.Options)) rdsClient
	getRedshiftClient(cfg aws.Config, optFns ...func(*redshift.Options)) redshiftClient
	getRedshiftServerlessClient(cfg aws.Config, optFns ...func(*rss.Options)) rssClient
	getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient
}

type defaultAWSClients struct{}

func (defaultAWSClients) getElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) elasticacheClient {
	return elasticache.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getIAMClient(cfg aws.Config, optFns ...func(*iam.Options)) iamClient {
	return iamutils.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) memoryDBClient {
	return memorydb.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getOpenSearchClient(cfg aws.Config, optFns ...func(*opensearch.Options)) openSearchClient {
	return opensearch.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getRDSClient(cfg aws.Config, optFns ...func(*rds.Options)) rdsClient {
	return rds.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getRedshiftClient(cfg aws.Config, optFns ...func(*redshift.Options)) redshiftClient {
	return redshift.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getRedshiftServerlessClient(cfg aws.Config, optFns ...func(*rss.Options)) rssClient {
	return rss.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient {
	return stsutils.NewFromConfig(cfg, optFns...)
}

// MetadataConfig is the cloud metadata service config.
type MetadataConfig struct {
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider

	// awsClients is an SDK client provider.
	awsClients awsClientProvider
}

// Check validates the metadata service config.
func (c *MetadataConfig) Check() error {
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}

	if c.awsClients == nil {
		c.awsClients = defaultAWSClients{}
	}
	return nil
}

// Metadata is a service that fetches cloud databases metadata.
type Metadata struct {
	cfg    MetadataConfig
	logger *slog.Logger
}

// NewMetadata returns a new cloud metadata service.
func NewMetadata(config MetadataConfig) (*Metadata, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Metadata{
		cfg:    config,
		logger: slog.With(teleport.ComponentKey, "meta"),
	}, nil
}

// Update updates cloud metadata of the provided database.
func (m *Metadata) Update(ctx context.Context, database types.Database) error {
	switch database.GetType() {
	case types.DatabaseTypeRDS:
		return m.updateAWS(ctx, database, m.fetchRDSMetadata)
	case types.DatabaseTypeRDSProxy:
		return m.updateAWS(ctx, database, m.fetchRDSProxyMetadata)
	case types.DatabaseTypeRedshift:
		return m.updateAWS(ctx, database, m.fetchRedshiftMetadata)
	case types.DatabaseTypeRedshiftServerless:
		return m.updateAWS(ctx, database, m.fetchRedshiftServerlessMetadata)
	case types.DatabaseTypeElastiCache:
		return m.updateAWS(ctx, database, m.fetchElastiCacheMetadata)
	case types.DatabaseTypeMemoryDB:
		return m.updateAWS(ctx, database, m.fetchMemoryDBMetadata)
	}
	return nil
}

// updateAWS updates cloud metadata of the provided AWS database.
func (m *Metadata) updateAWS(ctx context.Context, database types.Database, fetchFn func(context.Context, types.Database) (*types.AWS, error)) error {
	meta := database.GetAWS()
	fetchedMeta, err := fetchFn(ctx, database)
	if err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			m.logger.DebugContext(ctx, "No permissions to fetch metadata for database",
				"error", err,
				"database", database,
			)
			return nil
		}
		return trace.Wrap(err)
	}

	m.logger.DebugContext(ctx, "Fetched metadata for database", "database", database, "metadata", logutils.StringerAttr(fetchedMeta))
	fetchedMeta.AssumeRoleARN = meta.AssumeRoleARN
	fetchedMeta.ExternalID = meta.ExternalID
	database.SetStatusAWS(*fetchedMeta)
	return nil
}

// fetchRDSMetadata fetches metadata for the provided RDS or Aurora database.
func (m *Metadata) fetchRDSMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	meta := database.GetAWS()
	awsCfg, err := m.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := m.cfg.awsClients.getRDSClient(awsCfg)

	if meta.RDS.ClusterID != "" {
		return fetchRDSClusterMetadata(ctx, clt, meta.RDS.ClusterID)
	}

	// Try to fetch the RDS instance fetchedMeta.
	fetchedMeta, err := fetchRDSInstanceMetadata(ctx, clt, meta.RDS.InstanceID)
	if err != nil && !trace.IsNotFound(err) && !trace.IsAccessDenied(err) {
		return nil, trace.Wrap(err)
	}
	// If RDS instance metadata wasn't found, it may be an Aurora cluster.
	if fetchedMeta == nil {
		// Aurora cluster ID may be either explicitly specified or parsed
		// from endpoint in which case it will be in InstanceID field.
		clusterID := meta.RDS.ClusterID
		if clusterID == "" {
			clusterID = meta.RDS.InstanceID
		}
		return fetchRDSClusterMetadata(ctx, clt, clusterID)
	}
	// If instance was found, it may be a part of an Aurora cluster.
	if fetchedMeta.RDS.ClusterID != "" {
		return fetchRDSClusterMetadata(ctx, clt, fetchedMeta.RDS.ClusterID)
	}
	return fetchedMeta, nil
}

// fetchRDSProxyMetadata fetches metadata for the provided RDS Proxy database.
func (m *Metadata) fetchRDSProxyMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	meta := database.GetAWS()
	awsCfg, err := m.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := m.cfg.awsClients.getRDSClient(awsCfg)

	if meta.RDSProxy.CustomEndpointName != "" {
		return fetchRDSProxyCustomEndpointMetadata(ctx, clt, meta.RDSProxy.CustomEndpointName, database.GetURI())
	}
	return fetchRDSProxyMetadata(ctx, clt, meta.RDSProxy.Name)
}

// fetchRedshiftMetadata fetches metadata for the provided Redshift database.
func (m *Metadata) fetchRedshiftMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	meta := database.GetAWS()
	awsCfg, err := m.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	redshift := m.cfg.awsClients.getRedshiftClient(awsCfg)
	cluster, err := describeRedshiftCluster(ctx, redshift, meta.Redshift.ClusterID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoverycommon.MetadataFromRedshiftCluster(cluster)
}

// fetchRedshiftServerlessMetadata fetches metadata for the provided Redshift
// Serverless database.
func (m *Metadata) fetchRedshiftServerlessMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	meta := database.GetAWS()
	awsCfg, err := m.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := m.cfg.awsClients.getRedshiftServerlessClient(awsCfg)

	if meta.RedshiftServerless.EndpointName != "" {
		return fetchRedshiftServerlessVPCEndpointMetadata(ctx, clt, meta.RedshiftServerless.EndpointName)
	}
	return fetchRedshiftServerlessWorkgroupMetadata(ctx, clt, meta.RedshiftServerless.WorkgroupName)
}

// fetchElastiCacheMetadata fetches metadata for the provided ElastiCache database.
func (m *Metadata) fetchElastiCacheMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	meta := database.GetAWS()
	awsCfg, err := m.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := m.cfg.awsClients.getElastiCacheClient(awsCfg)
	cluster, err := describeElastiCacheCluster(ctx, clt, meta.ElastiCache.ReplicationGroupID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Endpoint type does not change.
	endpointType := meta.ElastiCache.EndpointType
	return discoverycommon.MetadataFromElastiCacheCluster(cluster, endpointType)
}

// fetchMemoryDBMetadata fetches metadata for the provided MemoryDB database.
func (m *Metadata) fetchMemoryDBMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	meta := database.GetAWS()
	awsCfg, err := m.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := m.cfg.awsClients.getMemoryDBClient(awsCfg)
	cluster, err := describeMemoryDBCluster(ctx, clt, meta.MemoryDB.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Endpoint type does not change.
	endpointType := meta.MemoryDB.EndpointType
	return discoverycommon.MetadataFromMemoryDBCluster(cluster, endpointType)
}

// fetchRDSInstanceMetadata fetches metadata about specified RDS instance.
func fetchRDSInstanceMetadata(ctx context.Context, clt rdsClient, instanceID string) (*types.AWS, error) {
	rdsInstance, err := describeRDSInstance(ctx, clt, instanceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoverycommon.MetadataFromRDSInstance(rdsInstance)
}

// describeRDSInstance returns AWS RDS instance for the specified ID.
func describeRDSInstance(ctx context.Context, clt rdsClient, instanceID string) (*rdstypes.DBInstance, error) {
	out, err := clt.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBInstances) != 1 {
		return nil, trace.BadParameter("expected 1 RDS instance for %v, got %d", instanceID, len(out.DBInstances))
	}
	return &out.DBInstances[0], nil
}

// fetchRDSClusterMetadata fetches metadata about specified Aurora cluster.
func fetchRDSClusterMetadata(ctx context.Context, clt rdsClient, clusterID string) (*types.AWS, error) {
	rdsCluster, err := describeRDSCluster(ctx, clt, clusterID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoverycommon.MetadataFromRDSCluster(rdsCluster)
}

// describeRDSCluster returns AWS Aurora cluster for the specified ID.
func describeRDSCluster(ctx context.Context, clt rdsClient, clusterID string) (*rdstypes.DBCluster, error) {
	out, err := clt.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBClusters) != 1 {
		return nil, trace.BadParameter("expected 1 RDS cluster for %v, got %+v", clusterID, out.DBClusters)
	}
	return &out.DBClusters[0], nil
}

// describeRedshiftCluster returns AWS Redshift cluster for the specified ID.
func describeRedshiftCluster(ctx context.Context, clt redshiftClient, clusterID string) (*redshifttypes.Cluster, error) {
	out, err := clt.DescribeClusters(ctx, &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.Clusters) != 1 {
		return nil, trace.BadParameter("expected 1 Redshift cluster for %v, got %+v", clusterID, out.Clusters)
	}
	return &out.Clusters[0], nil
}

// describeElastiCacheCluster returns AWS ElastiCache Redis cluster for the
// specified ID.
func describeElastiCacheCluster(ctx context.Context, elastiCacheClient elasticacheClient, replicationGroupID string) (*ectypes.ReplicationGroup, error) {
	out, err := elastiCacheClient.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(replicationGroupID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.ReplicationGroups) != 1 {
		return nil, trace.BadParameter("expected 1 ElastiCache cluster for %v, got %+v", replicationGroupID, out.ReplicationGroups)
	}
	return &out.ReplicationGroups[0], nil
}

// describeMemoryDBCluster returns AWS MemoryDB cluster for the specified ID.
func describeMemoryDBCluster(ctx context.Context, client memoryDBClient, clusterName string) (*memorydbtypes.Cluster, error) {
	out, err := client.DescribeClusters(ctx, &memorydb.DescribeClustersInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.Clusters) != 1 {
		return nil, trace.BadParameter("expected 1 MemoryDB cluster for %v, got %+v", clusterName, out.Clusters)
	}
	return &out.Clusters[0], nil
}

// fetchRDSProxyMetadata fetches metadata about specified RDS Proxy name.
func fetchRDSProxyMetadata(ctx context.Context, clt rdsClient, proxyName string) (*types.AWS, error) {
	rdsProxy, err := describeRDSProxy(ctx, clt, proxyName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoverycommon.MetadataFromRDSProxy(rdsProxy)
}

// describeRDSProxy returns AWS RDS Proxy for the specified RDS Proxy name.
func describeRDSProxy(ctx context.Context, clt rdsClient, proxyName string) (*rdstypes.DBProxy, error) {
	out, err := clt.DescribeDBProxies(ctx, &rds.DescribeDBProxiesInput{
		DBProxyName: aws.String(proxyName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBProxies) != 1 {
		return nil, trace.BadParameter("expected 1 RDS Proxy for %v, got %d", proxyName, len(out.DBProxies))
	}
	return &out.DBProxies[0], nil
}

// fetchRDSProxyCustomEndpointMetadata fetches metadata about specified RDS
// proxy custom endpoint.
func fetchRDSProxyCustomEndpointMetadata(ctx context.Context, clt rdsClient, proxyEndpointName, uri string) (*types.AWS, error) {
	rdsProxyEndpoint, err := describeRDSProxyCustomEndpointAndFindURI(ctx, clt, proxyEndpointName, uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rdsProxy, err := describeRDSProxy(ctx, clt, aws.ToString(rdsProxyEndpoint.DBProxyName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return discoverycommon.MetadataFromRDSProxyCustomEndpoint(rdsProxy, rdsProxyEndpoint)
}

// describeRDSProxyCustomEndpointAndFindURI returns AWS RDS Proxy endpoint for
// the specified RDS Proxy custom endpoint.
func describeRDSProxyCustomEndpointAndFindURI(ctx context.Context, clt rdsClient, proxyEndpointName, uri string) (*rdstypes.DBProxyEndpoint, error) {
	out, err := clt.DescribeDBProxyEndpoints(ctx, &rds.DescribeDBProxyEndpointsInput{
		DBProxyEndpointName: aws.String(proxyEndpointName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	var endpoints []string
	for _, e := range out.DBProxyEndpoints {
		endpoint := aws.ToString(e.Endpoint)
		if endpoint == "" {
			continue
		}
		// Double check if it has the same URI in case multiple custom
		// endpoints have the same name.
		if strings.Contains(uri, endpoint) {
			return &e, nil
		}
		endpoints = append(endpoints, endpoint)
	}
	return nil, trace.BadParameter("could not find RDS Proxy custom endpoint %v with URI %v, got %s", proxyEndpointName, uri, endpoints)
}

func fetchRedshiftServerlessWorkgroupMetadata(ctx context.Context, client rssClient, workgroupName string) (*types.AWS, error) {
	workgroup, err := describeRedshiftServerlessWorkgroup(ctx, client, workgroupName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoverycommon.MetadataFromRedshiftServerlessWorkgroup(workgroup)
}
func fetchRedshiftServerlessVPCEndpointMetadata(ctx context.Context, client rssClient, endpointName string) (*types.AWS, error) {
	endpoint, err := describeRedshiftServerlessVCPEndpoint(ctx, client, endpointName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	workgroup, err := describeRedshiftServerlessWorkgroup(ctx, client, aws.ToString(endpoint.WorkgroupName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoverycommon.MetadataFromRedshiftServerlessVPCEndpoint(endpoint, workgroup)
}
func describeRedshiftServerlessWorkgroup(ctx context.Context, client rssClient, workgroupName string) (*rsstypes.Workgroup, error) {
	output, err := client.GetWorkgroup(ctx, &rss.GetWorkgroupInput{
		WorkgroupName: aws.String(workgroupName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	return output.Workgroup, nil
}

func describeRedshiftServerlessVCPEndpoint(ctx context.Context, client rssClient, endpointName string) (*rsstypes.EndpointAccess, error) {
	output, err := client.GetEndpointAccess(ctx, &rss.GetEndpointAccessInput{
		EndpointName: aws.String(endpointName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	return output.Endpoint, nil
}
