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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// elasticacheFetcherConfig is the elasticache databases fetcher configuration.
type elasticacheFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// elasticache is the elasticache API client.
	elasticache elasticacheiface.ElastiCacheAPI
	// Region is the AWS region to query databases in.
	Region string
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *elasticacheFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.elasticache == nil {
		return trace.BadParameter("missing parameter elasticache")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// elasticacheFetcher retrieves elasticache databases.
type elasticacheFetcher struct {
	cfg elasticacheFetcherConfig
	log logrus.FieldLogger
}

// newElasticacheFetcher returns a new elasticache databases fetcher instance.
func newElasticacheFetcher(config elasticacheFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &elasticacheFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:elasticache",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns elasticache and Aurora databases matching the watcher's selectors.
func (f *elasticacheFetcher) Get(ctx context.Context) (types.Databases, error) {
	clusters, err := getElasticacheClusters(ctx, f.cfg.elasticache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		// TODO(jakule): filter by engine as memcache is not supported.
		if !services.IsElasticacheClusterAvailable(cluster) {
			f.log.Debugf("The current status of elasticache cluster %q is %q. Skipping.",
				aws.StringValue(cluster.ReplicationGroupId),
				aws.StringValue(cluster.Status))
			continue
		}

		database, err := services.NewDatabaseFromElasticacheCluster(cluster)
		if err != nil {
			f.log.Infof("Could not convert elasticache cluster %q to database resource: %v.",
				aws.StringValue(cluster.ReplicationGroupId), err)
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
func (f *elasticacheFetcher) String() string {
	return fmt.Sprintf("elasticacheFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
}

// getElasticacheClusters fetches all Elasticache clusters using the provided client,
// up to the specified max number of pages
func getElasticacheClusters(ctx context.Context, elasticacheClient elasticacheiface.ElastiCacheAPI) ([]*elasticache.ReplicationGroup, error) {
	var clusters []*elasticache.ReplicationGroup
	var pageNum int
	err := elasticacheClient.DescribeReplicationGroupsPagesWithContext(
		ctx,
		&elasticache.DescribeReplicationGroupsInput{},
		func(page *elasticache.DescribeReplicationGroupsOutput, lastPage bool) bool {
			pageNum++
			clusters = append(clusters, page.ReplicationGroups...)
			return pageNum <= maxPages
		},
	)
	return clusters, common.ConvertError(err)
}
