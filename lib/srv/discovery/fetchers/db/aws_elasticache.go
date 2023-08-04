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
package db

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

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
	ecClient, err := cfg.AWSClients.GetAWSElastiCacheClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := getElastiCacheClusters(ctx, ecClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eligibleClusters []*elasticache.ReplicationGroup
	for _, cluster := range clusters {
		if !services.IsElastiCacheClusterSupported(cluster) {
			cfg.Log.Debugf("ElastiCache cluster %q is not supported. Skipping.", aws.StringValue(cluster.ReplicationGroupId))
			continue
		}

		if !services.IsElastiCacheClusterAvailable(cluster) {
			cfg.Log.Debugf("The current status of ElastiCache cluster %q is %q. Skipping.",
				aws.StringValue(cluster.ReplicationGroupId),
				aws.StringValue(cluster.Status))
			continue
		}

		eligibleClusters = append(eligibleClusters, cluster)
	}

	if len(eligibleClusters) == 0 {
		return nil, nil
	}

	// Fetch more information to provide extra labels. Do not fail because some
	// of these labels are missing.
	allNodes, err := getElastiCacheNodes(ctx, ecClient)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Log.WithError(err).Debug("No permissions to describe nodes")
		} else {
			cfg.Log.WithError(err).Info("Failed to describe nodes.")
		}
	}
	allSubnetGroups, err := getElastiCacheSubnetGroups(ctx, ecClient)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Log.WithError(err).Debug("No permissions to describe subnet groups")
		} else {
			cfg.Log.WithError(err).Info("Failed to describe subnet groups.")
		}
	}

	var databases types.Databases
	for _, cluster := range eligibleClusters {
		// Resource tags are not found in elasticache.ReplicationGroup but can
		// be on obtained by elasticache.ListTagsForResource (one call per
		// resource).
		tags, err := getElastiCacheResourceTags(ctx, ecClient, cluster.ARN)
		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Log.WithError(err).Debug("No permissions to list resource tags")
			} else {
				cfg.Log.WithError(err).Infof("Failed to list resource tags for ElastiCache cluster %q.", aws.StringValue(cluster.ReplicationGroupId))
			}
		}

		extraLabels := services.ExtraElastiCacheLabels(cluster, tags, allNodes, allSubnetGroups)

		// Create database using configuration endpoint for Redis with cluster
		// mode enabled.
		if aws.BoolValue(cluster.ClusterEnabled) {
			if database, err := services.NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels); err != nil {
				cfg.Log.Infof("Could not convert ElastiCache cluster %q configuration endpoint to database resource: %v.",
					aws.StringValue(cluster.ReplicationGroupId), err)
			} else {
				databases = append(databases, database)
			}

			continue
		}

		// Create databases using primary and reader endpoints for Redis with
		// cluster mode disabled. When cluster mode is disabled, it is expected
		// there is only one node group (aka shard) with one primary endpoint
		// and one reader endpoint.
		if databasesFromNodeGroups, err := services.NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels); err != nil {
			cfg.Log.Infof("Could not convert ElastiCache cluster %q node groups to database resources: %v.",
				aws.StringValue(cluster.ReplicationGroupId), err)
		} else {
			databases = append(databases, databasesFromNodeGroups...)
		}
	}
	return databases, nil
}

// getElastiCacheClusters fetches all ElastiCache replication groups.
func getElastiCacheClusters(ctx context.Context, client elasticacheiface.ElastiCacheAPI) ([]*elasticache.ReplicationGroup, error) {
	var clusters []*elasticache.ReplicationGroup
	var pageNum int

	err := client.DescribeReplicationGroupsPagesWithContext(
		ctx,
		&elasticache.DescribeReplicationGroupsInput{},
		func(page *elasticache.DescribeReplicationGroupsOutput, lastPage bool) bool {
			pageNum++
			clusters = append(clusters, page.ReplicationGroups...)
			return pageNum <= maxAWSPages
		},
	)
	return clusters, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
}

// getElastiCacheNodes fetches all ElastiCache nodes that associated with a
// replication group.
func getElastiCacheNodes(ctx context.Context, client elasticacheiface.ElastiCacheAPI) ([]*elasticache.CacheCluster, error) {
	var nodes []*elasticache.CacheCluster
	var pageNum int

	err := client.DescribeCacheClustersPagesWithContext(
		ctx,
		&elasticache.DescribeCacheClustersInput{},
		func(page *elasticache.DescribeCacheClustersOutput, lastPage bool) bool {
			pageNum++

			// There are three types of elasticache.CacheCluster:
			// 1) a Memcache cluster.
			// 2) a Redis node belongs to a single node deployment (legacy, no TLS support).
			// 3) a Redis node belongs to a Redis replication group.
			// Only the ones belong to replication groups are wanted.
			for _, cacheCluster := range page.CacheClusters {
				if cacheCluster.ReplicationGroupId != nil {
					nodes = append(nodes, cacheCluster)
				}
			}
			return pageNum <= maxAWSPages
		},
	)
	return nodes, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
}

// getElastiCacheSubnetGroups fetches all ElastiCache subnet groups.
func getElastiCacheSubnetGroups(ctx context.Context, client elasticacheiface.ElastiCacheAPI) ([]*elasticache.CacheSubnetGroup, error) {
	var subnetGroups []*elasticache.CacheSubnetGroup
	var pageNum int

	err := client.DescribeCacheSubnetGroupsPagesWithContext(
		ctx,
		&elasticache.DescribeCacheSubnetGroupsInput{},
		func(page *elasticache.DescribeCacheSubnetGroupsOutput, lastPage bool) bool {
			pageNum++
			subnetGroups = append(subnetGroups, page.CacheSubnetGroups...)
			return pageNum <= maxAWSPages
		},
	)
	return subnetGroups, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
}

// getElastiCacheResourceTags fetches resource tags for provided ElastiCache
// replication group.
func getElastiCacheResourceTags(ctx context.Context, client elasticacheiface.ElastiCacheAPI, resourceName *string) ([]*elasticache.Tag, error) {
	input := &elasticache.ListTagsForResourceInput{
		ResourceName: resourceName,
	}
	output, err := client.ListTagsForResourceWithContext(ctx, input)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	return output.TagList, nil
}
