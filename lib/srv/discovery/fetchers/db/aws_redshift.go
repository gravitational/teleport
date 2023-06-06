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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// redshiftFetcherConfig is the Redshift databases fetcher configuration.
type redshiftFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// Redshift is the Redshift API client.
	Redshift redshiftiface.RedshiftAPI
	// Region is the AWS region to query databases in.
	Region string
	// AssumeRole is the AWS IAM role to assume before discovering databases.
	AssumeRole types.AssumeRole
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *redshiftFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.Redshift == nil {
		return trace.BadParameter("missing parameter Redshift")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// redshiftFetcher retrieves Redshift databases.
type redshiftFetcher struct {
	awsFetcher

	cfg redshiftFetcherConfig
	log logrus.FieldLogger
}

// newRedshiftFetcher returns a new Redshift databases fetcher instance.
func newRedshiftFetcher(config redshiftFetcherConfig) (common.Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &redshiftFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:redshift",
			"labels":        config.Labels,
			"region":        config.Region,
			"role":          config.AssumeRole,
		}),
	}, nil
}

// Get returns Redshift and Aurora databases matching the watcher's selectors.
func (f *redshiftFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := getRedshiftClusters(ctx, f.cfg.Redshift)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		if !services.IsRedshiftClusterAvailable(cluster) {
			f.log.Debugf("The current status of Redshift cluster %q is %q. Skipping.",
				aws.StringValue(cluster.ClusterIdentifier),
				aws.StringValue(cluster.ClusterStatus))
			continue
		}

		database, err := services.NewDatabaseFromRedshiftCluster(cluster)
		if err != nil {
			f.log.Infof("Could not convert Redshift cluster %q to database resource: %v.",
				aws.StringValue(cluster.ClusterIdentifier), err)
			continue
		}

		databases = append(databases, database)
	}
	applyAssumeRoleToDatabases(databases, f.cfg.AssumeRole)
	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log).AsResources(), nil
}

// String returns the fetcher's string description.
func (f *redshiftFetcher) String() string {
	return fmt.Sprintf("redshiftFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
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
