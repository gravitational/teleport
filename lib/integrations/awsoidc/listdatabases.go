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
	"log/slog"

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
	// VpcId filters databases to only include those deployed in the VPC.
	VpcId string
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

// listDatabasesPageSize is half the default RDS list input page size (100).
// We filter by VPC membership after the API call and try to return
// listDatabasesPageSize items but can return up to listDatabasesPageSize*2 -1
// items, so we use a smaller page size than the default.
var listDatabasesPageSize int32 = 50

// ListDatabases calls the following AWS API:
// https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBClusters.html
// https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBInstances.html
// It returns a list of Databases and an optional NextToken that can be used to fetch the next page
func ListDatabases(ctx context.Context, clt ListDatabasesClient, log *slog.Logger, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	all := &ListDatabasesResponse{}
	for {
		res, err := listDatabases(ctx, clt, log, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		all.Databases = append(all.Databases, res.Databases...)
		// keep fetching databases until we fill at least pageSize or run out of
		// pages, that way we don't return strange results like 0 databases with
		// a NextToken to fetch more.
		if len(all.Databases) >= int(listDatabasesPageSize) || res.NextToken == "" {
			all.NextToken = res.NextToken
			return all, nil
		}
		// re-use the request but update its NextToken for each API call.
		req.NextToken = res.NextToken
	}
}

func listDatabases(ctx context.Context, clt ListDatabasesClient, log *slog.Logger, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	// Uses https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBInstances.html
	if req.RDSType == rdsTypeInstance {
		ret, err := listDBInstances(ctx, clt, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ret, nil
	}

	// Uses https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBClusters.html
	ret, err := listDBClusters(ctx, clt, log, req)
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
		MaxRecords: &listDatabasesPageSize,
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
		if !cloudaws.IsDBClusterAvailable(db.DBInstanceStatus, db.DBInstanceIdentifier) {
			continue
		}
		if req.VpcId != "" && !subnetGroupIsInVPC(db.DBSubnetGroup, req.VpcId) {
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

func listDBClusters(ctx context.Context, clt ListDatabasesClient, log *slog.Logger, req ListDatabasesRequest) (*ListDatabasesResponse, error) {
	describeDBClusterInput := &rds.DescribeDBClustersInput{
		Filters: []rdsTypes.Filter{
			{Name: &filterEngine, Values: req.Engines},
		},
		MaxRecords: &listDatabasesPageSize,
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
		if !cloudaws.IsDBClusterAvailable(db.Status, db.DBClusterIdentifier) {
			continue
		}

		// RDS Clusters do not return VPC and Subnets.
		// To get this value, a member of the cluster is fetched and its Network Information is used to
		// populate the RDS Cluster information.
		// All the members have the same network information, so picking one at random should not matter.
		instances, err := fetchRDSClusterInstances(ctx, clt, req, aws.ToString(db.DBClusterIdentifier))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(instances) == 0 {
			log.InfoContext(ctx, "Skipping RDS cluster because it has no instances",
				"cluster", aws.ToString(db.DBClusterIdentifier),
			)
			continue
		}
		instance := &instances[0]

		if req.VpcId != "" && !subnetGroupIsInVPC(instance.DBSubnetGroup, req.VpcId) {
			continue
		}

		awsDB, err := common.NewDatabaseFromRDSV2Cluster(&db, instance)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.Databases = append(ret.Databases, awsDB)
	}

	return ret, nil
}

func fetchRDSClusterInstances(ctx context.Context, clt ListDatabasesClient, req ListDatabasesRequest, clusterID string) ([]rdsTypes.DBInstance, error) {
	describeDBInstanceInput := &rds.DescribeDBInstancesInput{
		Filters: []rdsTypes.Filter{
			{Name: &filterDBClusterID, Values: []string{clusterID}},
		},
	}

	rdsDBs, err := clt.DescribeDBInstances(ctx, describeDBInstanceInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rdsDBs.DBInstances, nil
}

// subnetGroupIsInVPC is a simple helper to check if a db subnet group is in
// a given VPC.
func subnetGroupIsInVPC(group *rdsTypes.DBSubnetGroup, vpcID string) bool {
	return group != nil && aws.ToString(group.VpcId) == vpcID
}
