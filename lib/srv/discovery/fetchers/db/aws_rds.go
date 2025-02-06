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
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// RDSClient is a subset of the AWS RDS API.
type RDSClient interface {
	rds.DescribeDBClustersAPIClient
	rds.DescribeDBInstancesAPIClient
	rds.DescribeDBProxiesAPIClient
	rds.DescribeDBProxyEndpointsAPIClient
	ListTagsForResource(context.Context, *rds.ListTagsForResourceInput, ...func(*rds.Options)) (*rds.ListTagsForResourceOutput, error)
}

// newRDSDBInstancesFetcher returns a new AWS fetcher for RDS databases.
func newRDSDBInstancesFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &rdsDBInstancesPlugin{})
}

// rdsDBInstancesPlugin retrieves RDS DB instances.
type rdsDBInstancesPlugin struct{}

func (f *rdsDBInstancesPlugin) ComponentShortName() string {
	return "rds"
}

// GetDatabases returns a list of database resources representing RDS instances.
func (f *rdsDBInstancesPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetRDSClient(awsCfg)
	instances, err := getAllDBInstances(ctx, clt, maxAWSPages, cfg.Logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databases := make(types.Databases, 0, len(instances))
	for _, instance := range instances {
		if !libcloudaws.IsRDSInstanceSupported(&instance) {
			cfg.Logger.DebugContext(ctx, "Skipping RDS instance that does not support IAM authentication",
				"instance", aws.ToString(instance.DBInstanceIdentifier),
				"engine_mode", aws.ToString(instance.Engine),
				"engine_version", aws.ToString(instance.EngineVersion),
			)
			continue
		}

		if !libcloudaws.IsRDSInstanceAvailable(instance.DBInstanceStatus, instance.DBInstanceIdentifier) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable RDS instance",
				"instance", aws.ToString(instance.DBInstanceIdentifier),
				"status", aws.ToString(instance.DBInstanceStatus),
			)
			continue
		}

		database, err := common.NewDatabaseFromRDSInstance(&instance)
		if err != nil {
			cfg.Logger.WarnContext(ctx, "Could not convert RDS instance to database resource",
				"instance", aws.ToString(instance.DBInstanceIdentifier),
				"error", err,
			)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// getAllDBInstances fetches all RDS instances using the provided client, up
// to the specified max number of pages.
func getAllDBInstances(ctx context.Context, clt RDSClient, maxPages int, logger *slog.Logger) ([]rdstypes.DBInstance, error) {
	return getAllDBInstancesWithFilters(ctx, clt, maxPages, rdsInstanceEngines(), rdsEmptyFilter(), logger)
}

// findDBInstancesForDBCluster returns the DBInstances associated with a given DB Cluster Identifier
func findDBInstancesForDBCluster(ctx context.Context, clt RDSClient, maxPages int, dbClusterIdentifier string, logger *slog.Logger) ([]rdstypes.DBInstance, error) {
	return getAllDBInstancesWithFilters(ctx, clt, maxPages, auroraEngines(), rdsClusterIDFilter(dbClusterIdentifier), logger)
}

// getAllDBInstancesWithFilters fetches all RDS instances matching the filters using the provided client, up
// to the specified max number of pages.
func getAllDBInstancesWithFilters(ctx context.Context, clt RDSClient, maxPages int, engines []string, baseFilters []rdstypes.Filter, logger *slog.Logger) ([]rdstypes.DBInstance, error) {
	var out []rdstypes.DBInstance
	err := retryWithIndividualEngineFilters(ctx, logger, engines, func(engineFilters []rdstypes.Filter) error {
		pager := rds.NewDescribeDBInstancesPaginator(clt,
			&rds.DescribeDBInstancesInput{
				Filters: append(engineFilters, baseFilters...),
			},
			func(dcpo *rds.DescribeDBInstancesPaginatorOptions) {
				dcpo.StopOnDuplicateToken = true
			},
		)
		var instances []rdstypes.DBInstance
		for i := 0; i < maxPages && pager.HasMorePages(); i++ {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			instances = append(instances, page.DBInstances...)
		}
		out = instances
		return nil
	})
	return out, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
}

// newRDSAuroraClustersFetcher returns a new AWS fetcher for RDS Aurora
// databases.
func newRDSAuroraClustersFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &rdsAuroraClustersPlugin{})
}

// rdsAuroraClustersPlugin retrieves RDS Aurora clusters.
type rdsAuroraClustersPlugin struct{}

func (f *rdsAuroraClustersPlugin) ComponentShortName() string {
	return "aurora"
}

// GetDatabases returns a list of database resources representing RDS clusters.
func (f *rdsAuroraClustersPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetRDSClient(awsCfg)
	clusters, err := getAllDBClusters(ctx, clt, maxAWSPages, cfg.Logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databases := make(types.Databases, 0, len(clusters))
	for _, cluster := range clusters {
		if !libcloudaws.IsRDSClusterSupported(&cluster) {
			cfg.Logger.DebugContext(ctx, "Skipping Aurora cluster that does not support IAM authentication",
				"cluster", aws.ToString(cluster.DBClusterIdentifier),
				"engine_mode", aws.ToString(cluster.EngineMode),
				"engine_version", aws.ToString(cluster.EngineVersion),
			)
			continue
		}

		if !libcloudaws.IsDBClusterAvailable(cluster.Status, cluster.DBClusterIdentifier) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable Aurora cluster",
				"instance", aws.ToString(cluster.DBClusterIdentifier),
				"status", aws.ToString(cluster.Status),
			)
			continue
		}

		rdsDBInstances, err := findDBInstancesForDBCluster(ctx, clt, maxAWSPages, aws.ToString(cluster.DBClusterIdentifier), cfg.Logger)
		if err != nil || len(rdsDBInstances) == 0 {
			cfg.Logger.WarnContext(ctx, "Could not fetch Member Instance for DB Cluster",
				"instance", aws.ToString(cluster.DBClusterIdentifier),
				"error", err,
			)
		}

		dbs, err := common.NewDatabasesFromRDSCluster(&cluster, rdsDBInstances)
		if err != nil {
			cfg.Logger.WarnContext(ctx, "Could not convert RDS cluster to database resources",
				"identifier", aws.ToString(cluster.DBClusterIdentifier),
				"error", err,
			)
		}
		databases = append(databases, dbs...)
	}
	return databases, nil
}

// getAllDBClusters fetches all RDS clusters using the provided client, up to
// the specified max number of pages.
func getAllDBClusters(ctx context.Context, clt RDSClient, maxPages int, logger *slog.Logger) ([]rdstypes.DBCluster, error) {
	var out []rdstypes.DBCluster
	err := retryWithIndividualEngineFilters(ctx, logger, auroraEngines(), func(filters []rdstypes.Filter) error {
		pager := rds.NewDescribeDBClustersPaginator(clt,
			&rds.DescribeDBClustersInput{
				Filters: filters,
			},
			func(pagerOpts *rds.DescribeDBClustersPaginatorOptions) {
				pagerOpts.StopOnDuplicateToken = true
			},
		)

		var clusters []rdstypes.DBCluster
		for i := 0; i < maxPages && pager.HasMorePages(); i++ {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			clusters = append(clusters, page.DBClusters...)
		}
		out = clusters
		return nil
	})
	return out, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
}

// rdsInstanceEngines returns engines to make sure DescribeDBInstances call returns
// only databases with engines Teleport supports.
func rdsInstanceEngines() []string {
	return []string{
		services.RDSEnginePostgres,
		services.RDSEngineMySQL,
		services.RDSEngineMariaDB,
	}
}

// auroraEngines returns engines to make sure DescribeDBClusters call returns
// only databases with engines Teleport supports.
func auroraEngines() []string {
	return []string{
		services.RDSEngineAuroraMySQL,
		services.RDSEngineAuroraPostgres,
	}
}

// rdsEngineFilter is a helper func to construct an RDS filter for engine names.
func rdsEngineFilter(engines []string) []rdstypes.Filter {
	return []rdstypes.Filter{{
		Name:   aws.String("engine"),
		Values: engines,
	}}
}

// rdsClusterIDFilter is a helper func to construct an RDS DB Instances for returning Instances of a specific DB Cluster.
func rdsClusterIDFilter(clusterIdentifier string) []rdstypes.Filter {
	return []rdstypes.Filter{{
		Name:   aws.String("db-cluster-id"),
		Values: []string{clusterIdentifier},
	}}
}

// rdsEmptyFilter is a helper func to construct an empty RDS filter.
func rdsEmptyFilter() []rdstypes.Filter {
	return []rdstypes.Filter{}
}

// rdsFilterFn is a function that takes RDS filters and performs some operation with them, returning any error encountered.
type rdsFilterFn func([]rdstypes.Filter) error

// retryWithIndividualEngineFilters is a helper error handling function for AWS RDS unrecognized engine name filter errors,
// that will call the provided RDS querying function with filters, check the returned error,
// and if the error is an AWS unrecognized engine name error then it will retry once by calling the function with one filter
// at a time. If any error other than an AWS unrecognized engine name error occurs, this function will return that error
// without retrying, or skip retrying subsequent filters if it has already started to retry.
func retryWithIndividualEngineFilters(ctx context.Context, logger *slog.Logger, engines []string, fn rdsFilterFn) error {
	err := fn(rdsEngineFilter(engines))
	if err == nil {
		return nil
	}
	if !isUnrecognizedAWSEngineNameError(err) {
		return trace.Wrap(err)
	}
	logger.DebugContext(ctx, "Teleport supports an engine which is unrecognized in this AWS region. Querying engine names individually.", "error", err)
	for _, engine := range engines {
		err := fn(rdsEngineFilter([]string{engine}))
		if err == nil {
			continue
		}
		if !isUnrecognizedAWSEngineNameError(err) {
			return trace.Wrap(err)
		}
		// skip logging unrecognized engine name error here, we already logged it in the initial attempt.
	}
	return nil
}

// isUnrecognizedAWSEngineNameError checks if the err is non-nil and came from using an engine filter that the
// AWS region does not recognize.
func isUnrecognizedAWSEngineNameError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unrecognized engine name")
}
