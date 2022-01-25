/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// rdsFetcherConfig is the RDS databases fetcher configuration.
type rdsFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// RDS is the RDS API client.
	RDS rdsiface.RDSAPI
	// Region is the AWS region to query databases in.
	Region string
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *rdsFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.RDS == nil {
		return trace.BadParameter("missing parameter RDS")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// rdsDBInstancesFetcher retrieves RDS DB instances.
type rdsDBInstancesFetcher struct {
	cfg rdsFetcherConfig
	log logrus.FieldLogger
}

// newRDSDBInstancesFetcher returns a new RDS DB instances fetcher instance.
func newRDSDBInstancesFetcher(config rdsFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &rdsDBInstancesFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:rds",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns RDS DB instances matching the watcher's selectors.
func (f *rdsDBInstancesFetcher) Get(ctx context.Context) (types.Databases, error) {
	rdsDatabases, err := f.getRDSDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result types.Databases
	for _, database := range rdsDatabases {
		match, _, err := services.MatchLabels(f.cfg.Labels, database.GetAllLabels())
		if err != nil {
			f.log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			result = append(result, database)
		} else {
			f.log.Debugf("%v doesn't match selector.", database)
		}
	}
	return result, nil
}

// getRDSDatabases returns a list of database resources representing RDS instances.
func (f *rdsDBInstancesFetcher) getRDSDatabases(ctx context.Context) (types.Databases, error) {
	instances, err := getAllDBInstances(ctx, f.cfg.RDS, maxPages)
	if err != nil {
		return nil, common.ConvertError(err)
	}
	databases := make(types.Databases, 0, len(instances))
	for _, instance := range instances {
		database, err := services.NewDatabaseFromRDSInstance(instance)
		if err != nil {
			f.log.Warnf("Could not convert RDS instance %q to database resource: %v.",
				aws.StringValue(instance.DBInstanceIdentifier), err)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// getAllDBInstances fetches all RDS instances using the provided client, up
// to the specified max number of pages.
func getAllDBInstances(ctx context.Context, rdsClient rdsiface.RDSAPI, maxPages int) (instances []*rds.DBInstance, err error) {
	var pageNum int
	err = rdsClient.DescribeDBInstancesPagesWithContext(ctx, &rds.DescribeDBInstancesInput{
		Filters: rdsFilters(),
	}, func(ddo *rds.DescribeDBInstancesOutput, lastPage bool) bool {
		pageNum++
		instances = append(instances, ddo.DBInstances...)
		return pageNum <= maxPages
	})
	return instances, common.ConvertError(err)
}

// String returns the fetcher's string description.
func (f *rdsDBInstancesFetcher) String() string {
	return fmt.Sprintf("rdsDBInstancesFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
}

// rdsAuroraClustersFetcher retrieves RDS Aurora clusters.
type rdsAuroraClustersFetcher struct {
	cfg rdsFetcherConfig
	log logrus.FieldLogger
}

// newRDSAuroraClustersFetcher returns a new RDS Aurora fetcher instance.
func newRDSAuroraClustersFetcher(config rdsFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &rdsAuroraClustersFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:aurora",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns Aurora clusters matching the watcher's selectors.
func (f *rdsAuroraClustersFetcher) Get(ctx context.Context) (types.Databases, error) {
	auroraDatabases, err := f.getAuroraDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result types.Databases
	for _, database := range auroraDatabases {
		match, _, err := services.MatchLabels(f.cfg.Labels, database.GetAllLabels())
		if err != nil {
			f.log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			result = append(result, database)
		} else {
			f.log.Debugf("%v doesn't match selector.", database)
		}
	}
	return result, nil
}

// getAuroraDatabases returns a list of database resources representing RDS clusters.
func (f *rdsAuroraClustersFetcher) getAuroraDatabases(ctx context.Context) (types.Databases, error) {
	clusters, err := getAllDBClusters(ctx, f.cfg.RDS, maxPages)
	if err != nil {
		return nil, common.ConvertError(err)
	}
	databases := make(types.Databases, 0, len(clusters))
	for _, cluster := range clusters {
		if !services.IsRDSClusterSupported(cluster) {
			f.log.Debugf("Aurora cluster %q (engine mode %v, engine version %v) doesn't support IAM authentication. Skipping.",
				aws.StringValue(cluster.DBClusterIdentifier),
				aws.StringValue(cluster.EngineMode),
				aws.StringValue(cluster.EngineVersion))
			continue
		}

		// Add a database from primary endpoint
		database, err := services.NewDatabaseFromRDSCluster(cluster)
		if err != nil {
			f.log.Warnf("Could not convert RDS cluster %q to database resource: %v.",
				aws.StringValue(cluster.DBClusterIdentifier), err)
		} else {
			databases = append(databases, database)
		}

		// Add a database from reader endpoint, only when the reader endpoint
		// is available and there is more than one instance. When the cluster
		// contains only a primary instance and no Aurora Replicas, the reader
		// endpoint connects to the primary instance, which makes the reader
		// database entry pointless.
		// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html#Aurora.Endpoints.Reader
		if cluster.ReaderEndpoint != nil && len(cluster.DBClusterMembers) > 1 {
			database, err := services.NewDatabaseFromRDSClusterReaderEndpoint(cluster)
			if err != nil {
				f.log.Warnf("Could not convert RDS cluster %q reader endpoint to database resource: %v.",
					aws.StringValue(cluster.DBClusterIdentifier), err)
			} else {
				databases = append(databases, database)
			}
		}

		// Add databases from custom endpoints
		if len(cluster.CustomEndpoints) > 0 {
			customEndpointDatabases, err := services.NewDatabasesFromRDSClusterCustomEndpoints(cluster)
			if err != nil {
				f.log.Warnf("Could not convert RDS cluster %q custom endpoints to database resources: %v.",
					aws.StringValue(cluster.DBClusterIdentifier), err)
			}

			databases = append(databases, customEndpointDatabases...)
		}
	}
	return databases, nil
}

// getAllDBClusters fetches all RDS clusters using the provided client, up to
// the specified max number of pages.
func getAllDBClusters(ctx context.Context, rdsClient rdsiface.RDSAPI, maxPages int) (clusters []*rds.DBCluster, err error) {
	var pageNum int
	err = rdsClient.DescribeDBClustersPagesWithContext(ctx, &rds.DescribeDBClustersInput{
		Filters: auroraFilters(),
	}, func(ddo *rds.DescribeDBClustersOutput, lastPage bool) bool {
		pageNum++
		clusters = append(clusters, ddo.DBClusters...)
		return pageNum <= maxPages
	})
	return clusters, common.ConvertError(err)
}

// String returns the fetcher's string description.
func (f *rdsAuroraClustersFetcher) String() string {
	return fmt.Sprintf("rdsAuroraClustersFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
}

// rdsFilters returns filters to make sure DescribeDBInstances call returns
// only databases with engines Teleport supports.
func rdsFilters() []*rds.Filter {
	return []*rds.Filter{{
		Name: aws.String("engine"),
		Values: aws.StringSlice([]string{
			services.RDSEnginePostgres,
			services.RDSEngineMySQL}),
	}}
}

// auroraFilters returns filters to make sure DescribeDBClusters call returns
// only databases with engines Teleport supports.
func auroraFilters() []*rds.Filter {
	return []*rds.Filter{{
		Name: aws.String("engine"),
		Values: aws.StringSlice([]string{
			services.RDSEngineAurora,
			services.RDSEngineAuroraMySQL,
			services.RDSEngineAuroraPostgres}),
	}}
}

// maxPages is the maximum number of pages to iterate over when fetching databases.
const maxPages = 10
