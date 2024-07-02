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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newDocumentDBFetcher returns a new AWS fetcher for RDS Aurora
// databases.
func newDocumentDBFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &rdsDocumentDBFetcher{})
}

// rdsDocumentDBFetcher retrieves DocumentDB clusters.
//
// Note that AWS DocumentDB internally uses the RDS APIs:
// https://github.com/aws/aws-sdk-go/blob/3248e69e16aa601ffa929be53a52439425257e5e/service/docdb/service.go#L33
// The interfaces/structs in "services/docdb" are usually a subset of those in
// "services/rds".
//
// TODO(greedy52) switch to aws-sdk-go-v2/services/docdb.
type rdsDocumentDBFetcher struct{}

func (f *rdsDocumentDBFetcher) ComponentShortName() string {
	return "docdb"
}

// GetDatabases returns a list of database resources representing DocumentDB endpoints.
func (f *rdsDocumentDBFetcher) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	rdsClient, err := cfg.AWSClients.GetAWSRDSClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		cloud.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := f.getAllDBClusters(ctx, rdsClient)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	databases := make(types.Databases, 0)
	for _, cluster := range clusters {
		if !libcloudaws.IsDocumentDBClusterSupported(cluster) {
			cfg.Log.Debugf("DocumentDB cluster %q (engine version %v) doesn't support IAM authentication. Skipping.",
				aws.StringValue(cluster.DBClusterIdentifier),
				aws.StringValue(cluster.EngineVersion))
			continue
		}

		if !libcloudaws.IsDocumentDBClusterAvailable(cluster.Status, cluster.DBClusterIdentifier) {
			cfg.Log.Debugf("The current status of DocumentDB cluster %q is %q. Skipping.",
				aws.StringValue(cluster.DBClusterIdentifier),
				aws.StringValue(cluster.Status))
			continue
		}

		dbs, err := common.NewDatabasesFromDocumentDBCluster(cluster)
		if err != nil {
			cfg.Log.Warnf("Could not convert DocumentDB cluster %q to database resources: %v.",
				aws.StringValue(cluster.DBClusterIdentifier), err)
		}
		databases = append(databases, dbs...)
	}
	return databases, nil
}

func (f *rdsDocumentDBFetcher) getAllDBClusters(ctx context.Context, rdsClient rdsiface.RDSAPI) ([]*rds.DBCluster, error) {
	var pageNum int
	var clusters []*rds.DBCluster
	err := rdsClient.DescribeDBClustersPagesWithContext(ctx, &rds.DescribeDBClustersInput{
		Filters: rdsEngineFilter([]string{"docdb"}),
	}, func(ddo *rds.DescribeDBClustersOutput, lastPage bool) bool {
		pageNum++
		clusters = append(clusters, ddo.DBClusters...)
		return pageNum <= maxAWSPages
	})
	return clusters, trace.Wrap(err)
}
