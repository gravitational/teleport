/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newElastiCacheServerlessFetcher returns a new AWS fetcher for ElastiCache Serverless databases.
func newElastiCacheServerlessFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &elastiCacheServerlessPlugin{})
}

// elastiCacheServerlessPlugin retrieves ElastiCache Serverless cache databases.
type elastiCacheServerlessPlugin struct{}

func (f *elastiCacheServerlessPlugin) ComponentShortName() string {
	// (e)lasti(c)ache (s)erver(<)less
	return "ecs<"
}

// GetDatabases returns ElastiCache Serverless Redis databases matching the watcher's selectors.
func (f *elastiCacheServerlessPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(awsconfig.IntegrationMetadata{Name: cfg.Integration}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetElastiCacheClient(awsCfg)
	caches, err := getElastiCacheServerlessCaches(ctx, clt, cfg.Logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eligibleCaches []ectypes.ServerlessCache
	for _, cache := range caches {
		if !libcloudaws.IsElastiCacheServerlessCacheSupported(&cache) {
			cfg.Logger.DebugContext(ctx, "Skipping unsupported ElastiCache Serverless cache",
				"cache_name", aws.ToString(cache.ServerlessCacheName),
				"engine", aws.ToString(cache.Engine),
			)
			continue
		}
		if !libcloudaws.IsElastiCacheServerlessCacheAvailable(&cache) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable ElastiCache Serverless cache",
				"cache_name", aws.ToString(cache.ServerlessCacheName),
				"status", aws.ToString(cache.Status),
			)
			continue
		}
		eligibleCaches = append(eligibleCaches, cache)
	}

	if len(eligibleCaches) == 0 {
		return nil, nil
	}

	subnets, err := listAllSubnets(ctx, cfg.awsClients.GetEC2Client(awsCfg), cfg.Logger)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Logger.DebugContext(ctx, "No permissions to describe subnets", "error", err)
		} else {
			cfg.Logger.InfoContext(ctx, "Failed to describe subnets", "error", err)
		}
	}
	var databases types.Databases
	for _, cache := range eligibleCaches {
		// Resource tags are not found in ectypes.ServerlessCache but can
		// be obtained via ectypes.ListTagsForResource (one call per resource).
		tags, err := getElastiCacheResourceTags(ctx, clt, cache.ARN)
		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Logger.DebugContext(ctx, "No permissions to list resource tags", "error", err)
			} else {
				cfg.Logger.InfoContext(ctx, "Failed to list resource tags for ElastiCache Serverless cache",
					"cache_name", aws.ToString(cache.ServerlessCacheName),
					"error", err,
				)
			}
		}

		extraLabels := common.ExtraElastiCacheServerlessLabels(&cache, tags, subnets)
		if db, err := common.NewDatabaseFromElastiCacheServerlessCache(&cache, extraLabels); err != nil {
			cfg.Logger.InfoContext(ctx, "Could not convert ElastiCache Serverless cache to database resource",
				"cache_name", aws.ToString(cache.ServerlessCacheName),
				"error", err,
			)
		} else {
			databases = append(databases, db)
		}
	}
	return databases, nil
}

// getElastiCacheServerlessCaches fetches all ElastiCache Serverless caches.
func getElastiCacheServerlessCaches(ctx context.Context, client ElastiCacheClient, log *slog.Logger) ([]ectypes.ServerlessCache, error) {
	var out []ectypes.ServerlessCache
	pager := elasticache.NewDescribeServerlessCachesPaginator(client,
		&elasticache.DescribeServerlessCachesInput{},
		func(opts *elasticache.DescribeServerlessCachesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for page, err := range pagesWithLimit(ctx, pager, log) {
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}
		out = append(out, page.ServerlessCaches...)
	}
	return out, nil
}

type subnetsByID map[string]ec2types.Subnet

// listAllSubnets returns all subnets in the current region mapped by subnet ID.
func listAllSubnets(ctx context.Context, client EC2Client, log *slog.Logger) (subnetsByID, error) {
	out := subnetsByID{}
	pager := ec2.NewDescribeSubnetsPaginator(client,
		&ec2.DescribeSubnetsInput{},
		func(opts *ec2.DescribeSubnetsPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for page, err := range pagesWithLimit(ctx, pager, log) {
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}
		for _, s := range page.Subnets {
			out[aws.ToString(s.SubnetId)] = s
		}
	}
	return out, nil
}
