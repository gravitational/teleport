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

package db

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// ElastiCacheClient is a subset of the AWS ElastiCache API.
type ElastiCacheClient interface {
	elasticache.DescribeCacheClustersAPIClient
	elasticache.DescribeCacheSubnetGroupsAPIClient
	elasticache.DescribeReplicationGroupsAPIClient

	ListTagsForResource(ctx context.Context, in *elasticache.ListTagsForResourceInput, optFns ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error)
}

// newElastiCacheFetcher returns a new AWS fetcher for ElastiCache databases.
func newElastiCacheFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &elastiCachePlugin{})
}

// elastiCachePlugin retrieves ElastiCache Redis databases.
type elastiCachePlugin struct{}

func (f *elastiCachePlugin) ComponentShortName() string {
	return "elasticache"
}

// GetDatabases returns ElastiCache Redis databases matching the watcher's selectors.
//
// TODO(greedy52) support ElastiCache global datastore.
func (f *elastiCachePlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetElastiCacheClient(awsCfg)
	clusters, err := getElastiCacheClusters(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eligibleClusters []ectypes.ReplicationGroup
	for _, cluster := range clusters {
		if !libcloudaws.IsElastiCacheClusterSupported(&cluster) {
			cfg.Logger.DebugContext(ctx, "Skipping unsupported ElastiCache cluster",
				"cluster", aws.ToString(cluster.ReplicationGroupId),
			)
			continue
		}

		if !libcloudaws.IsElastiCacheClusterAvailable(&cluster) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable ElastiCache cluster",
				"cluster", aws.ToString(cluster.ReplicationGroupId),
				"status", aws.ToString(cluster.Status),
			)
			continue
		}

		eligibleClusters = append(eligibleClusters, cluster)
	}

	if len(eligibleClusters) == 0 {
		return nil, nil
	}

	// Fetch more information to provide extra labels. Do not fail because some
	// of these labels are missing.
	allNodes, err := getElastiCacheNodes(ctx, clt)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Logger.DebugContext(ctx, "No permissions to describe nodes", "error", err)
		} else {
			cfg.Logger.InfoContext(ctx, "Failed to describe nodes", "error", err)
		}
	}
	allSubnetGroups, err := getElastiCacheSubnetGroups(ctx, clt)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Logger.DebugContext(ctx, "No permissions to describe subnet groups", "error", err)
		} else {
			cfg.Logger.InfoContext(ctx, "Failed to describe subnet groups", "error", err)
		}
	}

	var databases types.Databases
	for _, cluster := range eligibleClusters {
		// Resource tags are not found in ectypes.ReplicationGroup but can
		// be on obtained by ectypes.ListTagsForResource (one call per
		// resource).
		tags, err := getElastiCacheResourceTags(ctx, clt, cluster.ARN)
		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Logger.DebugContext(ctx, "No permissions to list resource tags", "error", err)
			} else {
				cfg.Logger.InfoContext(ctx, "Failed to list resource tags for ElastiCache cluster",
					"cluster", aws.ToString(cluster.ReplicationGroupId),
					"error", err,
				)
			}
		}

		extraLabels := common.ExtraElastiCacheLabels(&cluster, tags, allNodes, allSubnetGroups)

		if dbs, err := common.NewDatabasesFromElastiCacheReplicationGroup(&cluster, extraLabels); err != nil {
			cfg.Logger.InfoContext(ctx, "Could not convert ElastiCache cluster to database resources",
				"cluster", aws.ToString(cluster.ReplicationGroupId),
				"error", err,
			)
		} else {
			databases = append(databases, dbs...)
		}
	}
	return databases, nil
}

// getElastiCacheClusters fetches all ElastiCache replication groups.
func getElastiCacheClusters(ctx context.Context, client ElastiCacheClient) ([]ectypes.ReplicationGroup, error) {
	var out []ectypes.ReplicationGroup
	pager := elasticache.NewDescribeReplicationGroupsPaginator(client,
		&elasticache.DescribeReplicationGroupsInput{},
		func(opts *elasticache.DescribeReplicationGroupsPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}
		out = append(out, page.ReplicationGroups...)
	}
	return out, nil
}

// getElastiCacheNodes fetches all ElastiCache nodes that associated with a
// replication group.
func getElastiCacheNodes(ctx context.Context, client ElastiCacheClient) ([]ectypes.CacheCluster, error) {
	var out []ectypes.CacheCluster
	pager := elasticache.NewDescribeCacheClustersPaginator(client,
		&elasticache.DescribeCacheClustersInput{},
		func(opts *elasticache.DescribeCacheClustersPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}
		// There are three types of ectypes.CacheCluster:
		// 1) a Memcache cluster.
		// 2) a Redis node belongs to a single node deployment (legacy, no TLS support).
		// 3) a Redis node belongs to a Redis replication group.
		// Only the ones belong to replication groups are wanted.
		for _, cacheCluster := range page.CacheClusters {
			if cacheCluster.ReplicationGroupId != nil {
				out = append(out, cacheCluster)
			}
		}
	}
	return out, nil
}

// getElastiCacheSubnetGroups fetches all ElastiCache subnet groups.
func getElastiCacheSubnetGroups(ctx context.Context, client ElastiCacheClient) ([]ectypes.CacheSubnetGroup, error) {
	var out []ectypes.CacheSubnetGroup
	pager := elasticache.NewDescribeCacheSubnetGroupsPaginator(client,
		&elasticache.DescribeCacheSubnetGroupsInput{},
		func(opts *elasticache.DescribeCacheSubnetGroupsPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}
		out = append(out, page.CacheSubnetGroups...)
	}
	return out, nil
}

// getElastiCacheResourceTags fetches resource tags for provided ElastiCache
// replication group.
func getElastiCacheResourceTags(ctx context.Context, client ElastiCacheClient, resourceName *string) ([]ectypes.Tag, error) {
	input := &elasticache.ListTagsForResourceInput{
		ResourceName: resourceName,
	}
	output, err := client.ListTagsForResource(ctx, input)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	return output.TagList, nil
}
