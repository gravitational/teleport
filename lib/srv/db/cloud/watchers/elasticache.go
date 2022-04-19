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

// elastiCacheFetcherConfig is the ElastiCache databases fetcher configuration.
type elastiCacheFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// ElastiCache is the ElastiCache API client.
	ElastiCache elasticacheiface.ElastiCacheAPI
	// Region is the AWS region to query databases in.
	Region string
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *elastiCacheFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.ElastiCache == nil {
		return trace.BadParameter("missing parameter ElastiCache")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// elastiCacheFetcher retrieves ElastiCache Redis databases.
type elastiCacheFetcher struct {
	cfg elastiCacheFetcherConfig
	log logrus.FieldLogger
}

// newElastiCacheFetcher returns a new ElastiCache databases fetcher instance.
func newElastiCacheFetcher(config elastiCacheFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &elastiCacheFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:elasticache",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns ElastiCache Redis databases matching the watcher's selectors.
func (f *elastiCacheFetcher) Get(ctx context.Context) (types.Databases, error) {
	clusters, err := getElastiCacheClusters(ctx, f.cfg.ElastiCache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, cluster := range clusters {
		if !services.IsElastiCacheClusterSupported(cluster) {
			f.log.Debugf("ElastiCache cluster %q is not supported. Skipping.", aws.StringValue(cluster.ReplicationGroupId))
			continue
		}

		if !services.IsElastiCacheClusterAvailable(cluster) {
			f.log.Debugf("The current status of ElastiCache cluster %q is %q. Skipping.",
				aws.StringValue(cluster.ReplicationGroupId),
				aws.StringValue(cluster.Status))
			continue
		}

		if aws.BoolValue(cluster.ClusterEnabled) {
			database, err := services.NewDatabaseFromElastiCacheConfigurationEndpoint(cluster)
			if err != nil {
				f.log.Infof("Could not convert ElastiCache cluster %q to database resource: %v.",
					aws.StringValue(cluster.ReplicationGroupId), err)
				continue
			}
			databases = append(databases, database)
		} else {
			databasesFromNodeGroup, err := services.NewDatabasesFromElastiCacheNodeGroup(cluster)
			if err != nil {
				f.log.Infof("Could not convert ElastiCache cluster %q to database resource: %v.",
					aws.StringValue(cluster.ReplicationGroupId), err)
				continue
			}
			databases = append(databases, databasesFromNodeGroup...)
		}
	}

	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
}

// String returns the fetcher's string description.
func (f *elastiCacheFetcher) String() string {
	return fmt.Sprintf("elastiCacheFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
}

// getElastiCacheClusters fetches all ElastiCache replication groups using the
// provided client, up to the specified max number of pages.
func getElastiCacheClusters(ctx context.Context, client elasticacheiface.ElastiCacheAPI) ([]*elasticache.ReplicationGroup, error) {
	var clusters []*elasticache.ReplicationGroup
	var pageNum int

	// DescribeReplicationGroupsPages returns Redis replication groups with
	// cluster mode both enabled and disabled.
	//
	// DescribeCacheClusters is not used here as it returns either Memcached
	// clusters, or Redis single server deployments which do not support TLS.
	err := client.DescribeReplicationGroupsPagesWithContext(
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
