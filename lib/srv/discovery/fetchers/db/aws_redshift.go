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
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// RedshiftClient is a subset of the AWS Redshift API.
type RedshiftClient interface {
	redshift.DescribeClustersAPIClient
}

// newRedshiftFetcher returns a new AWS fetcher for Redshift databases.
func newRedshiftFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &redshiftPlugin{})
}

// redshiftPlugin retrieves Redshift databases.
type redshiftPlugin struct{}

// GetDatabases returns Redshift databases matching the watcher's selectors.
func (f *redshiftPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := getRedshiftClusters(ctx, cfg.awsClients.GetRedshiftClient(awsCfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		if !libcloudaws.IsRedshiftClusterAvailable(&cluster) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable Redshift cluster",
				"cluster", aws.ToString(cluster.ClusterIdentifier),
				"status", aws.ToString(cluster.ClusterStatus),
			)
			continue
		}

		database, err := common.NewDatabaseFromRedshiftCluster(&cluster)
		if err != nil {
			cfg.Logger.InfoContext(ctx, "Could not convert Redshift cluster to database resource",
				"cluster", aws.ToString(cluster.ClusterIdentifier),
				"error", err,
			)
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
func getRedshiftClusters(ctx context.Context, clt RedshiftClient) ([]redshifttypes.Cluster, error) {
	pager := redshift.NewDescribeClustersPaginator(clt,
		&redshift.DescribeClustersInput{},
		func(dcpo *redshift.DescribeClustersPaginatorOptions) {
			dcpo.StopOnDuplicateToken = true
		},
	)
	var clusters []redshifttypes.Cluster
	for pageNum := 0; pageNum < maxAWSPages && pager.HasMorePages(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, libcloudaws.ConvertRequestFailureError(err)
		}
		clusters = append(clusters, page.Clusters...)
	}
	return clusters, nil
}
