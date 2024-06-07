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

package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdsTypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	cloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

var (
	// filterEngine is the filter name for filtering Databses based on their engine.
	filterEngine = "engine"

	// filterDBClusterID is the filter name for filtering RDS Instances for a given RDS Cluster.
	filterDBClusterID = "db-cluster-id"
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
		if !cloudaws.IsRDSInstanceAvailable(db.DBInstanceStatus, db.DBInstanceIdentifier) {
			continue
		}

		dbServer, err := common.NewDatabaseFromRDSV2Instance(&db)
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
		if !cloudaws.IsRDSClusterAvailable(db.Status, db.DBClusterIdentifier) {
			continue
		}

		// RDS Clusters do not return VPC and Subnets.
		// To get this value, a member of the cluster is fetched and its Network Information is used to
		// populate the RDS Cluster information.
		// All the members have the same network information, so picking one at random should not matter.
		clusterInstance, err := fetchSingleRDSDBInstance(ctx, clt, req, aws.ToString(db.DBClusterIdentifier))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		awsDB, err := common.NewDatabaseFromRDSV2Cluster(&db, clusterInstance)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.Databases = append(ret.Databases, awsDB)
	}

	return ret, nil
}

func fetchSingleRDSDBInstance(ctx context.Context, clt ListDatabasesClient, req ListDatabasesRequest, clusterID string) (*rdsTypes.DBInstance, error) {
	describeDBInstanceInput := &rds.DescribeDBInstancesInput{
		Filters: []rdsTypes.Filter{
			{Name: &filterDBClusterID, Values: []string{clusterID}},
		},
	}

	rdsDBs, err := clt.DescribeDBInstances(ctx, describeDBInstanceInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(rdsDBs.DBInstances) == 0 {
		return nil, trace.BadParameter("database cluster %s has no instance", clusterID)
	}

	return &rdsDBs.DBInstances[0], nil
}
