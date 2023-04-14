/*
Copyright 2023 Gravitational, Inc.

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

package awsoidc

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/services"
)

var (
	// filterEngine is the filter name for filtering Databses based on their engine.
	filterEngine = "engine"

	// EnginesCluster is a list of engines which are considered AWS RDS Clusters.
	// If the request engine is part of this list, the api call is DescribeDBClusters
	// List extracted from here and filtered by the supported RDS databases.
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-rds-dbcluster.html
	// > The name of the database engine to be used for this DB cluster.
	EnginesCluster = []string{
		services.RDSEngineAurora,
		services.RDSEngineAuroraMySQL,
		services.RDSEngineAuroraPostgres,
		services.RDSEngineMySQL,
		services.RDSEnginePostgres,
	}

	// EnginesInstances is a list of engines which are considered AWS RDS Instances.
	// If the request engine is part of this list, the api call is DescribeDBInstances
	// List extracted from here and filtered by the supported RDS databases.
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-rds-dbinstance.html
	// > The name of the database engine that you want to use for this DB instance.
	EnginesInstances = []string{
		services.RDSEngineMySQL,
		services.RDSEnginePostgres,
		services.RDSEngineMariaDB,
	}
)

const (
	// rdsTypeInstance identifies RDS DBs of type Instance.
	rdsTypeInstance = "instance"

	// rdsTypeCluster identifies RDS DBs of type Cluster (Aurora).
	rdsTypeCluster = "cluster"
)

// ListDatabasesRequest contains the required fields to list AWS Databases.
type ListDatabasesRequest struct {
	// Region is the AWS Region
	Region string
	// RDSType is either `instance` or `cluster`.
	RDSType string
	// Engines filters the returned Databases based on their engine.
	// Eg, mysql, postgres, mariadb, aurora, aurora-mysql, aurora-postgresql
	Engines []string
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *ListDatabasesRequest) CheckAndSetDefaults() error {
	if req.Region == "" {
		return trace.BadParameter("region is required")
	}

	if !(req.RDSType == rdsTypeCluster || req.RDSType == rdsTypeInstance) {
		return trace.BadParameter("invalid rds type, supported values: instance, cluster")
	}

	if len(req.Engines) == 0 {
		return trace.BadParameter("a list of engines is required")
	}

	return nil
}

// AWSDatabase is a representation of an AWS RDS Database
type AWSDatabase struct {
	// Name is the the Database's name.
	Name string
	// Engine of the database. Eg, sqlserver-ex
	Engine string
	// Status contains the Instance status. Eg, available (https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html)
	Status string
	// Endpoint contains the URI for connecting to this Database
	Endpoint string
	// Labels contains the Instance tags.
	Labels map[string]string
}

// ListDatabasesResponse contains a page of AWS Databases.
type ListDatabasesResponse struct {
	// Databases contains the page of Databases
	Databases []AWSDatabase

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string
}

// ListDatabasesClient describes the required methods to List Databases (Instances and Clusters) using a 3rd Party API.
type ListDatabasesClient interface {
	// Returns information about provisioned RDS instances.
	// This API supports pagination.
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)

	// Returns information about Amazon Aurora DB clusters and Multi-AZ DB clusters.
	// This API supports pagination
	DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

// ListDatabases calls the following AWS API:
// https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBClusters.html
// https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBInstances.html
// It returns a list of Databases and an optional NextToken that can be used to fetch the next page
func ListDatabases(ctx context.Context, clt ListDatabasesClient, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Uses https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBInstances.html
	if req.RDSType == rdsTypeInstance {
		ret, err := listDBInstances(ctx, clt, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ret, nil
	}

	// Uses https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBClusters.html
	ret, err := listDBClusters(ctx, clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ret, nil
}

func listDBInstances(ctx context.Context, clt ListDatabasesClient, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	describeDBInstanceInput := &rds.DescribeDBInstancesInput{
		Filters: []types.Filter{
			{Name: &filterEngine, Values: req.Engines},
		},
	}
	if req.NextToken != "" {
		describeDBInstanceInput.Marker = &req.NextToken
	}

	rdsDBs, err := clt.DescribeDBInstances(ctx, describeDBInstanceInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &ListDatabasesResponse{}

	if rdsDBs.Marker != nil && *rdsDBs.Marker != "" {
		ret.NextToken = *rdsDBs.Marker
	}

	ret.Databases = make([]AWSDatabase, 0, len(rdsDBs.DBInstances))
	for _, db := range rdsDBs.DBInstances {
		awsDB, err := convertDBInstanceToDatabase(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.Databases = append(ret.Databases, *awsDB)
	}

	return ret, nil
}

// convertDBInstanceToDatabase converts a Database from its rds/types.DBInstance representation into
// a Teleport representation.
func convertDBInstanceToDatabase(in types.DBInstance) (*AWSDatabase, error) {
	ret := &AWSDatabase{}

	if in.DBInstanceIdentifier == nil || *in.DBInstanceIdentifier == "" {
		return nil, trace.BadParameter("database identifier not present")
	}
	ret.Name = *in.DBInstanceIdentifier

	if in.DBInstanceStatus == nil || *in.DBInstanceStatus == "" {
		return nil, trace.BadParameter("database status not present")
	}
	ret.Status = *in.DBInstanceStatus

	if in.Engine == nil || *in.Engine == "" {
		return nil, trace.BadParameter("database engine not present")
	}
	ret.Engine = *in.Engine

	ret.Labels = convertTagListToLabels(in.TagList)

	// Endpoint may not exist.
	// This is the case when the Database was just created created but is not yet available.
	if in.Endpoint != nil && in.Endpoint.Address != nil && *in.Endpoint.Address != "" && in.Endpoint.Port != 0 {
		ret.Endpoint = fmt.Sprintf("%s:%d", *in.Endpoint.Address, in.Endpoint.Port)
	}

	return ret, nil
}

func listDBClusters(ctx context.Context, clt ListDatabasesClient, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	describeDBClusterInput := &rds.DescribeDBClustersInput{
		Filters: []types.Filter{
			{Name: &filterEngine, Values: req.Engines},
		},
	}
	if req.NextToken != "" {
		describeDBClusterInput.Marker = &req.NextToken
	}

	rdsDBs, err := clt.DescribeDBClusters(ctx, describeDBClusterInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &ListDatabasesResponse{}

	if rdsDBs.Marker != nil && *rdsDBs.Marker != "" {
		ret.NextToken = *rdsDBs.Marker
	}

	ret.Databases = make([]AWSDatabase, 0, len(rdsDBs.DBClusters))
	for _, db := range rdsDBs.DBClusters {
		awsDB, err := convertDBClusterToDatabase(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.Databases = append(ret.Databases, *awsDB)
	}

	return ret, nil
}

// convertDBClusterToDatabase converts a Database from its rds/types.DBCluster representation into
// a Teleport representation.
func convertDBClusterToDatabase(in types.DBCluster) (*AWSDatabase, error) {
	ret := &AWSDatabase{}

	if in.DBClusterIdentifier == nil || *in.DBClusterIdentifier == "" {
		return nil, trace.BadParameter("database identifier not present")
	}
	ret.Name = *in.DBClusterIdentifier

	if in.Status == nil || *in.Status == "" {
		return nil, trace.BadParameter("database status not present")
	}
	ret.Status = *in.Status

	if in.Engine == nil || *in.Engine == "" {
		return nil, trace.BadParameter("database engine not present")
	}
	ret.Engine = *in.Engine

	ret.Labels = convertTagListToLabels(in.TagList)

	// Endpoint may not exist.
	// This is the case when the Database was just created created but is not yet available.
	if in.Endpoint != nil && *in.Endpoint != "" {
		ret.Endpoint = *in.Endpoint
	}

	return ret, nil
}

// convertTagListToLabels converts an AWS RDS list of Tags into a map[string]string.
func convertTagListToLabels(in []types.Tag) map[string]string {
	ret := make(map[string]string, len(in))
	for _, kv := range in {
		if kv.Key == nil || kv.Value == nil {
			continue
		}

		ret[*kv.Key] = *kv.Value
	}
	return ret
}
