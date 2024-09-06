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
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newRedshiftFetcher returns a new AWS fetcher for Redshift databases.
func newRedshiftFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &redshiftPlugin{})
}

// redshiftPlugin retrieves Redshift databases.
type redshiftPlugin struct{}

// GetDatabases returns Redshift databases matching the watcher's selectors.
func (f *redshiftPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	redshiftClient, err := cfg.AWSClients.GetAWSRedshiftClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		cloud.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := getRedshiftClusters(ctx, redshiftClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		if !libcloudaws.IsRedshiftClusterAvailable(cluster) {
			cfg.Log.Debugf("The current status of Redshift cluster %q is %q. Skipping.",
				aws.StringValue(cluster.ClusterIdentifier),
				aws.StringValue(cluster.ClusterStatus))
			continue
		}

		database, err := common.NewDatabaseFromRedshiftCluster(cluster)
		if err != nil {
			cfg.Log.Infof("Could not convert Redshift cluster %q to database resource: %v.",
				aws.StringValue(cluster.ClusterIdentifier), err)
			continue
		}

		databases = append(databases, database)
	}
	return databases, nil
}

func (f *redshiftPlugin) ComponentShortName() string {
	return "redshift"
}

// getRedshiftClusters fetches all Reshift clusters using the provided client,
// up to the specified max number of pages
func getRedshiftClusters(ctx context.Context, redshiftClient redshiftiface.RedshiftAPI) ([]*redshift.Cluster, error) {
	var clusters []*redshift.Cluster
	var pageNum int
	err := redshiftClient.DescribeClustersPagesWithContext(
		ctx,
		&redshift.DescribeClustersInput{},
		func(page *redshift.DescribeClustersOutput, lastPage bool) bool {
			pageNum++
			clusters = append(clusters, page.Clusters...)
			return pageNum <= maxAWSPages
		},
	)
	return clusters, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
}
