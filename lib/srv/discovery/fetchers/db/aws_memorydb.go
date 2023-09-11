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
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// memoryDBFetcherConfig is the MemoryDB databases fetcher configuration.
type memoryDBFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// MemoryDB is the MemoryDB API client.
	MemoryDB memorydbiface.MemoryDBAPI
	// Region is the AWS region to query databases in.
	Region string
	// AssumeRole is the AWS IAM role to assume before discovering databases.
	AssumeRole types.AssumeRole
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *memoryDBFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.MemoryDB == nil {
		return trace.BadParameter("missing parameter MemoryDB")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// memoryDBFetcher retrieves MemoryDB Redis databases.
type memoryDBFetcher struct {
	awsFetcher

	cfg memoryDBFetcherConfig
	log logrus.FieldLogger
}

// newMemoryDBFetcher returns a new MemoryDB databases fetcher instance.
func newMemoryDBFetcher(config memoryDBFetcherConfig) (common.Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &memoryDBFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:memorydb",
			"labels":        config.Labels,
			"region":        config.Region,
			"role":          config.AssumeRole,
		}),
	}, nil
}

// Get returns MemoryDB databases matching the watcher's selectors.
func (f *memoryDBFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := getMemoryDBClusters(ctx, f.cfg.MemoryDB)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eligibleClusters []*memorydb.Cluster
	for _, cluster := range clusters {
		if !services.IsMemoryDBClusterSupported(cluster) {
			f.log.Debugf("MemoryDB cluster %q is not supported. Skipping.", aws.StringValue(cluster.Name))
			continue
		}

		if !services.IsMemoryDBClusterAvailable(cluster) {
			f.log.Debugf("The current status of MemoryDB cluster %q is %q. Skipping.",
				aws.StringValue(cluster.Name),
				aws.StringValue(cluster.Status))
			continue
		}

		eligibleClusters = append(eligibleClusters, cluster)
	}

	if len(eligibleClusters) == 0 {
		return types.ResourcesWithLabels{}, nil
	}

	// Fetch more information to provide extra labels. Do not fail because some
	// of these labels are missing.
	allSubnetGroups, err := getMemoryDBSubnetGroups(ctx, f.cfg.MemoryDB)
	if err != nil {
		if trace.IsAccessDenied(err) {
			f.log.WithError(err).Debug("No permissions to describe subnet groups")
		} else {
			f.log.WithError(err).Info("Failed to describe subnet groups.")
		}
	}

	var databases types.Databases
	for _, cluster := range eligibleClusters {
		tags, err := getMemoryDBResourceTags(ctx, f.cfg.MemoryDB, cluster.ARN)
		if err != nil {
			if trace.IsAccessDenied(err) {
				f.log.WithError(err).Debug("No permissions to list resource tags")
			} else {
				f.log.WithError(err).Infof("Failed to list resource tags for MemoryDB cluster %q.", aws.StringValue(cluster.Name))
			}
		}

		extraLabels := services.ExtraMemoryDBLabels(cluster, tags, allSubnetGroups)
		database, err := services.NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
		if err != nil {
			f.log.WithError(err).Infof("Could not convert memorydb cluster %q configuration endpoint to database resource.", aws.StringValue(cluster.Name))
		} else {
			databases = append(databases, database)
		}
	}
	applyAssumeRoleToDatabases(databases, f.cfg.AssumeRole)
	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log).AsResources(), nil
}

// String returns the fetcher's string description.
func (f *memoryDBFetcher) String() string {
	return fmt.Sprintf("memorydbFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
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
