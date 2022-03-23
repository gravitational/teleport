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

package watchers

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"

	"github.com/sirupsen/logrus"
)

// redshiftFetcherConfig is the Redshift databases fetcher configuration.
type redshiftFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// Redshift is the Redshift API client.
	Redshift redshiftiface.RedshiftAPI
	// Region is the AWS region to query databases in.
	Region string
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
	cfg redshiftFetcherConfig
	log logrus.FieldLogger
}

// newRedshiftFetcher returns a new Redshift databases fetcher instance.
func newRedshiftFetcher(config redshiftFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &redshiftFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:redshift",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns Redshift and Aurora databases matching the watcher's selectors.
func (f *redshiftFetcher) Get(ctx context.Context) (types.Databases, error) {
	clusters, err := getRedshiftClusters(ctx, f.cfg.Redshift)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		database, err := services.NewDatabaseFromRedshiftCluster(cluster)
		if err != nil {
			f.log.Infof("Could not convert Redshift cluster %q to database resource: %v.",
				aws.StringValue(cluster.ClusterIdentifier), err)
			continue
		}

		match, _, err := services.MatchLabels(f.cfg.Labels, database.GetAllLabels())
		if err != nil {
			f.log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			databases = append(databases, database)
		} else {
			f.log.Debugf("%v doesn't match selector.", database)
		}
	}
	return databases, nil
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
			return pageNum <= maxPages
		},
	)
	return clusters, common.ConvertError(err)
}
