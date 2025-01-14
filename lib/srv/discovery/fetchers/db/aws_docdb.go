/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newDocumentDBFetcher returns a new AWS fetcher for RDS Aurora
// databases.
func newDocumentDBFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &rdsDocumentDBFetcher{})
}

// rdsDocumentDBFetcher retrieves DocumentDB clusters.
type rdsDocumentDBFetcher struct{}

func (f *rdsDocumentDBFetcher) ComponentShortName() string {
	return "docdb"
}

// GetDatabases returns a list of database resources representing DocumentDB endpoints.
func (f *rdsDocumentDBFetcher) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetRDSClient(awsCfg)
	clusters, err := f.getAllDBClusters(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases := make(types.Databases, 0)
	for _, cluster := range clusters {
		if !libcloudaws.IsDocumentDBClusterSupported(&cluster) {
			cfg.Logger.DebugContext(ctx, "DocumentDB cluster doesn't support IAM authentication. Skipping.",
				"cluster", aws.StringValue(cluster.DBClusterIdentifier),
				"engine_version", aws.StringValue(cluster.EngineVersion))
			continue
		}

		if !libcloudaws.IsDBClusterAvailable(cluster.Status, cluster.DBClusterIdentifier) {
			cfg.Logger.DebugContext(ctx, "DocumentDB cluster is not available. Skipping.",
				"cluster", aws.StringValue(cluster.DBClusterIdentifier),
				"status", aws.StringValue(cluster.Status))
			continue
		}

		dbs, err := common.NewDatabasesFromDocumentDBCluster(&cluster)
		if err != nil {
			cfg.Logger.WarnContext(ctx, "Could not convert DocumentDB cluster to database resources.",
				"cluster", aws.StringValue(cluster.DBClusterIdentifier),
				"error", err)
		}
		databases = append(databases, dbs...)
	}
	return databases, nil
}

func (f *rdsDocumentDBFetcher) getAllDBClusters(ctx context.Context, clt RDSClient) ([]rdstypes.DBCluster, error) {
	pager := rds.NewDescribeDBClustersPaginator(clt,
		&rds.DescribeDBClustersInput{
			Filters: rdsEngineFilter([]string{"docdb"}),
		},
		func(pagerOpts *rds.DescribeDBClustersPaginatorOptions) {
			pagerOpts.StopOnDuplicateToken = true
		},
	)

	var clusters []rdstypes.DBCluster
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureErrorV2(err))
		}
		clusters = append(clusters, page.DBClusters...)
	}
	return clusters, nil
}
