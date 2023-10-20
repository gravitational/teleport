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
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
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
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := getRedshiftClusters(ctx, redshiftClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		if !services.IsRedshiftClusterAvailable(cluster) {
			cfg.Log.Debugf("The current status of Redshift cluster %q is %q. Skipping.",
				aws.StringValue(cluster.ClusterIdentifier),
				aws.StringValue(cluster.ClusterStatus))
			continue
		}

		database, err := services.NewDatabaseFromRedshiftCluster(cluster)
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
