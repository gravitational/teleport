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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

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
	memDBClient, err := cfg.AWSClients.GetAWSMemoryDBClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		cloud.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := getMemoryDBClusters(ctx, memDBClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eligibleClusters []*memorydb.Cluster
	for _, cluster := range clusters {
		if !libcloudaws.IsMemoryDBClusterSupported(cluster) {
			cfg.Log.Debugf("MemoryDB cluster %q is not supported. Skipping.", aws.StringValue(cluster.Name))
			continue
		}

		if !libcloudaws.IsMemoryDBClusterAvailable(cluster) {
			cfg.Log.Debugf("The current status of MemoryDB cluster %q is %q. Skipping.",
				aws.StringValue(cluster.Name),
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
	allSubnetGroups, err := getMemoryDBSubnetGroups(ctx, memDBClient)
	if err != nil {
		if trace.IsAccessDenied(err) {
			cfg.Log.WithError(err).Debug("No permissions to describe subnet groups")
		} else {
			cfg.Log.WithError(err).Info("Failed to describe subnet groups.")
		}
	}

	var databases types.Databases
	for _, cluster := range eligibleClusters {
		tags, err := getMemoryDBResourceTags(ctx, memDBClient, cluster.ARN)
		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Log.WithError(err).Debug("No permissions to list resource tags")
			} else {
				cfg.Log.WithError(err).Infof("Failed to list resource tags for MemoryDB cluster %q.", aws.StringValue(cluster.Name))
			}
		}

		extraLabels := common.ExtraMemoryDBLabels(cluster, tags, allSubnetGroups)
		database, err := common.NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
		if err != nil {
			cfg.Log.WithError(err).Infof("Could not convert memorydb cluster %q configuration endpoint to database resource.", aws.StringValue(cluster.Name))
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// getMemoryDBClusters fetches all MemoryDB clusters.
func getMemoryDBClusters(ctx context.Context, client memorydbiface.MemoryDBAPI) ([]*memorydb.Cluster, error) {
	var clusters []*memorydb.Cluster
	var nextToken *string

	// MemoryDBAPI does NOT have "page" version of the describe API so use the
	// NextToken from the output in a loop.
	for pageNum := 0; pageNum < maxAWSPages; pageNum++ {
		output, err := client.DescribeClustersWithContext(ctx,
			&memorydb.DescribeClustersInput{
				NextToken: nextToken,
			},
		)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}

		clusters = append(clusters, output.Clusters...)
		if nextToken = output.NextToken; nextToken == nil {
			break
		}
	}
	return clusters, nil
}

// getMemoryDBSubnetGroups fetches all MemoryDB subnet groups.
func getMemoryDBSubnetGroups(ctx context.Context, client memorydbiface.MemoryDBAPI) ([]*memorydb.SubnetGroup, error) {
	var subnetGroups []*memorydb.SubnetGroup
	var nextToken *string

	for pageNum := 0; pageNum < maxAWSPages; pageNum++ {
		output, err := client.DescribeSubnetGroupsWithContext(ctx,
			&memorydb.DescribeSubnetGroupsInput{
				NextToken: nextToken,
			},
		)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}

		subnetGroups = append(subnetGroups, output.SubnetGroups...)
		if nextToken = output.NextToken; nextToken == nil {
			break
		}
	}
	return subnetGroups, nil
}

// getMemoryDBResourceTags fetches resource tags for provided ARN.
func getMemoryDBResourceTags(ctx context.Context, client memorydbiface.MemoryDBAPI, resourceARN *string) ([]*memorydb.Tag, error) {
	output, err := client.ListTagsWithContext(ctx, &memorydb.ListTagsInput{
		ResourceArn: resourceARN,
	})
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	return output.TagList, nil
}
