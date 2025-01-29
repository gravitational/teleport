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
	memorydb "github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// MemoryDBClient is a subset of the AWS MemoryDB API.
type MemoryDBClient interface {
	memorydb.DescribeClustersAPIClient
	memorydb.DescribeSubnetGroupsAPIClient

	ListTags(ctx context.Context, in *memorydb.ListTagsInput, optFns ...func(*memorydb.Options)) (*memorydb.ListTagsOutput, error)
}

// memoryDBPlugin retrieves MemoryDB Redis databases.
type memoryDBPlugin struct{}

// newMemoryDBFetcher returns a new AWS fetcher for MemoryDB databases.
func newMemoryDBFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &memoryDBPlugin{})
}

func (f *memoryDBPlugin) ComponentShortName() string {
	return "memorydb"
}

// GetDatabases returns MemoryDB databases matching the watcher's selectors.
func (f *memoryDBPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetMemoryDBClient(awsCfg)
	clusters, err := getMemoryDBClusters(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eligibleClusters []memorydbtypes.Cluster
	for _, cluster := range clusters {
		if !libcloudaws.IsMemoryDBClusterSupported(&cluster) {
			cfg.Logger.DebugContext(ctx, "Skipping unsupported MemoryDB cluster", "cluster", aws.ToString(cluster.Name))
			continue
		}

		if !libcloudaws.IsMemoryDBClusterAvailable(&cluster) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable MemoryDB cluster",
				"cluster", aws.ToString(cluster.Name),
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
	allSubnetGroups, err := getMemoryDBSubnetGroups(ctx, clt)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Logger.DebugContext(ctx, "No permissions to describe subnet groups", "error", err)
		} else {
			cfg.Logger.InfoContext(ctx, "Failed to describe subnet groups", "error", err)
		}
	}

	var databases types.Databases
	for _, cluster := range eligibleClusters {
		tags, err := getMemoryDBResourceTags(ctx, clt, cluster.ARN)
		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Logger.DebugContext(ctx, "No permissions to list resource tags", "error", err)
			} else {
				cfg.Logger.InfoContext(ctx, "Failed to list resource tags for MemoryDB cluster ",
					"error", err,
					"cluster", aws.ToString(cluster.Name),
				)
			}
		}

		extraLabels := common.ExtraMemoryDBLabels(&cluster, tags, allSubnetGroups)
		database, err := common.NewDatabaseFromMemoryDBCluster(&cluster, extraLabels)
		if err != nil {
			cfg.Logger.InfoContext(ctx, "Could not convert memorydb cluster configuration endpoint to database resource",
				"error", err,
				"cluster", aws.ToString(cluster.Name),
			)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// getMemoryDBClusters fetches all MemoryDB clusters.
func getMemoryDBClusters(ctx context.Context, client MemoryDBClient) ([]memorydbtypes.Cluster, error) {
	var out []memorydbtypes.Cluster
	pager := memorydb.NewDescribeClustersPaginator(client,
		&memorydb.DescribeClustersInput{},
		func(opts *memorydb.DescribeClustersPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureErrorV2(err))
		}
		out = append(out, page.Clusters...)
	}
	return out, nil
}

// getMemoryDBSubnetGroups fetches all MemoryDB subnet groups.
func getMemoryDBSubnetGroups(ctx context.Context, client MemoryDBClient) ([]memorydbtypes.SubnetGroup, error) {
	var out []memorydbtypes.SubnetGroup
	pager := memorydb.NewDescribeSubnetGroupsPaginator(client,
		&memorydb.DescribeSubnetGroupsInput{},
		func(opts *memorydb.DescribeSubnetGroupsPaginatorOptions) { opts.StopOnDuplicateToken = true },
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureErrorV2(err))
		}
		out = append(out, page.SubnetGroups...)
	}
	return out, nil
}

// getMemoryDBResourceTags fetches resource tags for provided ARN.
func getMemoryDBResourceTags(ctx context.Context, client MemoryDBClient, resourceARN *string) ([]memorydbtypes.Tag, error) {
	output, err := client.ListTags(ctx, &memorydb.ListTagsInput{
		ResourceArn: resourceARN,
	})
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	return output.TagList, nil
}
