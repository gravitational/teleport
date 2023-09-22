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

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdsTypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

var (
	// filterEngine is the filter name for filtering Databses based on their engine.
	filterEngine = "engine"
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

// ListDatabasesResponse contains a page of AWS Databases.
type ListDatabasesResponse struct {
	// Databases contains the page of Databases
	Databases []types.Database

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

// NewListDatabasesClient creates a new ListDatabasesClient using a AWSClientRequest.
func NewListDatabasesClient(ctx context.Context, req *AWSClientRequest) (ListDatabasesClient, error) {
	return newRDSClient(ctx, req)
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
		Filters: []rdsTypes.Filter{
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

	ret.Databases = make([]types.Database, 0, len(rdsDBs.DBInstances))
	for _, db := range rdsDBs.DBInstances {
		if !services.IsRDSInstanceAvailable(db.DBInstanceStatus, db.DBInstanceIdentifier) {
			continue
		}

		dbServer, err := services.NewDatabaseFromRDSV2Instance(&db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.Databases = append(ret.Databases, dbServer)
	}

	return ret, nil
}

func listDBClusters(ctx context.Context, clt ListDatabasesClient, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	describeDBClusterInput := &rds.DescribeDBClustersInput{
		Filters: []rdsTypes.Filter{
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

	ret.Databases = make([]types.Database, 0, len(rdsDBs.DBClusters))
	for _, db := range rdsDBs.DBClusters {
		if !services.IsRDSClusterAvailable(db.Status, db.DBClusterIdentifier) {
			continue
		}

		awsDB, err := services.NewDatabaseFromRDSV2Cluster(&db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.Databases = append(ret.Databases, awsDB)
	}

	return ret, nil
}
