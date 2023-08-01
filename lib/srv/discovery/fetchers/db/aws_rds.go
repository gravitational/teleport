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

package db

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

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
	rdsClient, err := cfg.AWSClients.GetAWSRDSClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instances, err := getAllDBInstances(ctx, rdsClient, maxAWSPages, cfg.Log)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}
	databases := make(types.Databases, 0, len(instances))
	for _, instance := range instances {
		if !services.IsRDSInstanceSupported(instance) {
			cfg.Log.Debugf("RDS instance %q (engine mode %v, engine version %v) doesn't support IAM authentication. Skipping.",
				aws.StringValue(instance.DBInstanceIdentifier),
				aws.StringValue(instance.Engine),
				aws.StringValue(instance.EngineVersion))
			continue
		}

		if !services.IsRDSInstanceAvailable(instance.DBInstanceStatus, instance.DBInstanceIdentifier) {
			cfg.Log.Debugf("The current status of RDS instance %q is %q. Skipping.",
				aws.StringValue(instance.DBInstanceIdentifier),
				aws.StringValue(instance.DBInstanceStatus))
			continue
		}

		database, err := services.NewDatabaseFromRDSInstance(instance)
		if err != nil {
			cfg.Log.Warnf("Could not convert RDS instance %q to database resource: %v.",
				aws.StringValue(instance.DBInstanceIdentifier), err)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// getAllDBInstances fetches all RDS instances using the provided client, up
// to the specified max number of pages.
func getAllDBInstances(ctx context.Context, rdsClient rdsiface.RDSAPI, maxPages int, log logrus.FieldLogger) ([]*rds.DBInstance, error) {
	var instances []*rds.DBInstance
	err := retryWithIndividualEngineFilters(log, rdsInstanceEngines(), func(filters []*rds.Filter) error {
		var pageNum int
		var out []*rds.DBInstance
		err := rdsClient.DescribeDBInstancesPagesWithContext(ctx, &rds.DescribeDBInstancesInput{
			Filters: filters,
		}, func(ddo *rds.DescribeDBInstancesOutput, lastPage bool) bool {
			pageNum++
			instances = append(instances, ddo.DBInstances...)
			return pageNum <= maxPages
		})
		if err == nil {
			// only append to instances on nil error, just in case we have to retry.
			instances = append(instances, out...)
		}
		return trace.Wrap(err)
	})
	return instances, trace.Wrap(err)
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
	rdsClient, err := cfg.AWSClients.GetAWSRDSClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := getAllDBClusters(ctx, rdsClient, maxAWSPages, cfg.Log)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}
	databases := make(types.Databases, 0, len(clusters))
	for _, cluster := range clusters {
		if !services.IsRDSClusterSupported(cluster) {
			cfg.Log.Debugf("Aurora cluster %q (engine mode %v, engine version %v) doesn't support IAM authentication. Skipping.",
				aws.StringValue(cluster.DBClusterIdentifier),
				aws.StringValue(cluster.EngineMode),
				aws.StringValue(cluster.EngineVersion))
			continue
		}

		if !services.IsRDSClusterAvailable(cluster.Status, cluster.DBClusterIdentifier) {
			cfg.Log.Debugf("The current status of Aurora cluster %q is %q. Skipping.",
				aws.StringValue(cluster.DBClusterIdentifier),
				aws.StringValue(cluster.Status))
			continue
		}

		// Find out what types of instances the cluster has. Some examples:
		// - Aurora cluster with one instance: one writer
		// - Aurora cluster with three instances: one writer and two readers
		// - Secondary cluster of a global database: one or more readers
		var hasWriterInstance, hasReaderInstance bool
		for _, clusterMember := range cluster.DBClusterMembers {
			if clusterMember != nil {
				if aws.BoolValue(clusterMember.IsClusterWriter) {
					hasWriterInstance = true
				} else {
					hasReaderInstance = true
				}
			}
		}

		// Add a database from primary endpoint, if any writer instances.
		if cluster.Endpoint != nil && hasWriterInstance {
			database, err := services.NewDatabaseFromRDSCluster(cluster)
			if err != nil {
				cfg.Log.Warnf("Could not convert RDS cluster %q to database resource: %v.",
					aws.StringValue(cluster.DBClusterIdentifier), err)
			} else {
				databases = append(databases, database)
			}
		}

		// Add a database from reader endpoint, if any reader instances.
		// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html#Aurora.Endpoints.Reader
		if cluster.ReaderEndpoint != nil && hasReaderInstance {
			database, err := services.NewDatabaseFromRDSClusterReaderEndpoint(cluster)
			if err != nil {
				cfg.Log.Warnf("Could not convert RDS cluster %q reader endpoint to database resource: %v.",
					aws.StringValue(cluster.DBClusterIdentifier), err)
			} else {
				databases = append(databases, database)
			}
		}

		// Add databases from custom endpoints
		if len(cluster.CustomEndpoints) > 0 {
			customEndpointDatabases, err := services.NewDatabasesFromRDSClusterCustomEndpoints(cluster)
			if err != nil {
				cfg.Log.Warnf("Could not convert RDS cluster %q custom endpoints to database resources: %v.",
					aws.StringValue(cluster.DBClusterIdentifier), err)
			}

			databases = append(databases, customEndpointDatabases...)
		}
	}
	return databases, nil
}

// getAllDBClusters fetches all RDS clusters using the provided client, up to
// the specified max number of pages.
func getAllDBClusters(ctx context.Context, rdsClient rdsiface.RDSAPI, maxPages int, log logrus.FieldLogger) ([]*rds.DBCluster, error) {
	var clusters []*rds.DBCluster
	err := retryWithIndividualEngineFilters(log, auroraEngines(), func(filters []*rds.Filter) error {
		var pageNum int
		var out []*rds.DBCluster
		err := rdsClient.DescribeDBClustersPagesWithContext(ctx, &rds.DescribeDBClustersInput{
			Filters: filters,
		}, func(ddo *rds.DescribeDBClustersOutput, lastPage bool) bool {
			pageNum++
			out = append(out, ddo.DBClusters...)
			return pageNum <= maxPages
		})
		if err == nil {
			// only append to clusters on nil error, just in case we have to retry.
			clusters = append(clusters, out...)
		}
		return trace.Wrap(err)
	})
	return clusters, trace.Wrap(err)
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
		services.RDSEngineAurora,
		services.RDSEngineAuroraMySQL,
		services.RDSEngineAuroraPostgres,
	}
}

// rdsEngineFilter is a helper func to construct an RDS filter for engine names.
func rdsEngineFilter(engines []string) []*rds.Filter {
	return []*rds.Filter{{
		Name:   aws.String("engine"),
		Values: aws.StringSlice(engines),
	}}
}

// rdsFilterFn is a function that takes RDS filters and performs some operation with them, returning any error encountered.
type rdsFilterFn func([]*rds.Filter) error

// retryWithIndividualEngineFilters is a helper error handling function for AWS RDS unrecognized engine name filter errors,
// that will call the provided RDS querying function with filters, check the returned error,
// and if the error is an AWS unrecognized engine name error then it will retry once by calling the function with one filter
// at a time. If any error other than an AWS unrecognized engine name error occurs, this function will return that error
// without retrying, or skip retrying subsequent filters if it has already started to retry.
func retryWithIndividualEngineFilters(log logrus.FieldLogger, engines []string, fn rdsFilterFn) error {
	err := fn(rdsEngineFilter(engines))
	if err == nil {
		return nil
	}
	if !isUnrecognizedAWSEngineNameError(err) {
		return trace.Wrap(err)
	}
	log.WithError(trace.Unwrap(err)).Debug("Teleport supports an engine which is unrecognized in this AWS region. Querying engine names individually.")
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
