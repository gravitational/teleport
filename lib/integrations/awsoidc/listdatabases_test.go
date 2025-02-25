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
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdsTypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func stringPointer(s string) *string {
	return &s
}

type mockListDatabasesClient struct {
	dbInstances []rdsTypes.DBInstance
	dbClusters  []rdsTypes.DBCluster
}

// Returns information about provisioned RDS instances.
// This API supports pagination.
func (m mockListDatabasesClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	requestedPage := 1

	instances := m.dbInstances
	for _, f := range params.Filters {
		if aws.ToString(f.Name) != filterDBClusterID {
			continue
		}
		clusterIDFilter := f.Values[0]
		var filtered []rdsTypes.DBInstance
		for _, db := range instances {
			if aws.ToString(db.DBClusterIdentifier) == clusterIDFilter {
				filtered = append(filtered, db)
			}
		}
		instances = filtered
		break
	}

	if params.Marker != nil {
		currentMarker, err := strconv.Atoi(*params.Marker)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := int(listDatabasesPageSize) * (requestedPage - 1)
	sliceEnd := int(listDatabasesPageSize) * requestedPage
	totalInstances := len(instances)
	if sliceEnd > totalInstances {
		sliceEnd = totalInstances
	}

	ret := &rds.DescribeDBInstancesOutput{
		DBInstances: instances[sliceStart:sliceEnd],
	}

	if sliceEnd < totalInstances {
		nextToken := fmt.Sprintf("%d", requestedPage+1)
		ret.Marker = stringPointer(nextToken)
	}

	return ret, nil
}

// Returns information about Amazon Aurora DB clusters and Multi-AZ DB clusters.
// This API supports pagination
func (m mockListDatabasesClient) DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{
		DBClusters: m.dbClusters,
	}, nil
}

func TestListDatabases(t *testing.T) {
	ctx := context.Background()

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	clusterPort := int32(5432)

	t.Run("pagination", func(t *testing.T) {
		pageOffset := 7
		databasesPerVPC := int(listDatabasesPageSize)*3 + pageOffset
		vpcIDs := []string{"vpc-1", "vpc-2", "vpc-3"}
		totalDBs := databasesPerVPC * len(vpcIDs)

		allInstances := make([]rdsTypes.DBInstance, 0, totalDBs)
		for i, vpcID := range vpcIDs {
			for j := 0; j < databasesPerVPC; j++ {
				allInstances = append(allInstances, rdsTypes.DBInstance{
					DBInstanceStatus:     stringPointer("available"),
					DBInstanceIdentifier: stringPointer(fmt.Sprintf("db-%v", i*databasesPerVPC+j)),
					DbiResourceId:        stringPointer("db-123"),
					DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
					DBSubnetGroup: &rdsTypes.DBSubnetGroup{
						VpcId: aws.String(vpcID),
					},
					Engine: stringPointer("postgres"),
					Endpoint: &rdsTypes.Endpoint{
						Address: stringPointer("endpoint.amazonaws.com"),
						Port:    aws.Int32(5432),
					},
				})
			}
		}

		mockListClient := mockListDatabasesClient{
			dbInstances: allInstances,
		}

		t.Run("without vpc filter", func(t *testing.T) {
			t.Parallel()
			logger := utils.NewSlogLoggerForTests().With("test", t.Name())
			// First page must return pageSize number of DBs
			req := ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				VpcId:     "",
				NextToken: "",
			}
			for i := 0; i < totalDBs/int(listDatabasesPageSize); i++ {
				resp, err := ListDatabases(ctx, mockListClient, logger, req)
				require.NoError(t, err)
				require.Len(t, resp.Databases, int(listDatabasesPageSize))
				require.NotEmpty(t, resp.NextToken)
				req.NextToken = resp.NextToken
			}
			// Last page must return remaining databases and an empty token.
			resp, err := ListDatabases(ctx, mockListClient, logger, req)
			require.NoError(t, err)
			require.Len(t, resp.Databases, totalDBs%int(listDatabasesPageSize))
			require.Empty(t, resp.NextToken)
		})

		t.Run("with vpc filter", func(t *testing.T) {
			t.Parallel()
			logger := utils.NewSlogLoggerForTests().With("test", t.Name())
			// First page must return at least pageSize number of DBs
			var gotDatabases []types.Database
			wantVPC := "vpc-2"
			resp, err := ListDatabases(ctx, mockListClient, logger, ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				VpcId:     wantVPC,
				NextToken: "",
			})
			require.NoError(t, err)
			// the first few pages of databases are in vpc-1, and filtering is done
			// client side, so we keep fetching pages until we fill at least
			// pageSize.
			// The first page with vpc-2 databases will have pageSize-offset
			// databases in vpc-2, which is less than pageSize, so we fetch another
			// page, this time getting a full pageSize of databases in vpc-2, and
			// then we return pageSize-offset+pageSize databases.
			require.Len(t, resp.Databases, 2*int(listDatabasesPageSize)-pageOffset)
			require.NotEmpty(t, resp.NextToken)
			nextPageToken := resp.NextToken
			gotDatabases = append(gotDatabases, resp.Databases...)

			// Second page must return pageSize number of DBs
			resp, err = ListDatabases(ctx, mockListClient, logger, ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				VpcId:     wantVPC,
				NextToken: nextPageToken,
			})
			require.NoError(t, err)
			require.Len(t, resp.Databases, int(listDatabasesPageSize))
			require.NotEmpty(t, resp.NextToken)
			nextPageToken = resp.NextToken
			gotDatabases = append(gotDatabases, resp.Databases...)

			// Third page must return only the remaining DBs and an empty nextToken
			resp, err = ListDatabases(ctx, mockListClient, logger, ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				VpcId:     wantVPC,
				NextToken: nextPageToken,
			})
			require.NoError(t, err)
			require.Len(t, resp.Databases, databasesPerVPC-len(gotDatabases))
			require.Empty(t, resp.NextToken)
			gotDatabases = append(gotDatabases, resp.Databases...)

			for i, db := range gotDatabases {
				require.Equal(t, wantVPC, db.GetAWS().RDS.VPCID, "database %d should be in the requested VPC", i)
			}
		})
	})

	for _, tt := range []struct {
		name          string
		req           ListDatabasesRequest
		mockInstances []rdsTypes.DBInstance
		mockClusters  []rdsTypes.DBCluster
		errCheck      func(error) bool
		respCheck     func(*testing.T, *ListDatabasesResponse)
	}{
		{
			name: "valid for listing instances",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			mockInstances: []rdsTypes.DBInstance{{
				DBInstanceStatus:     stringPointer("available"),
				DBInstanceIdentifier: stringPointer("my-db"),
				Engine:               stringPointer("postgres"),
				DbiResourceId:        stringPointer("db-123"),
				DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
				Endpoint: &rdsTypes.Endpoint{
					Address: stringPointer("endpoint.amazonaws.com"),
					Port:    aws.Int32(5432),
				},
			},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-db",
						Description: "RDS instance in ",
						Labels: map[string]string{
							"account-id":         "123456789012",
							"endpoint-type":      "instance",
							"engine":             "postgres",
							"engine-version":     "",
							"region":             "",
							"status":             "available",
							"teleport.dev/cloud": "AWS",
						},
					},
					types.DatabaseSpecV3{
						Protocol: "postgres",
						URI:      "endpoint.amazonaws.com:5432",
						AWS: types.AWS{
							AccountID: "123456789012",
							RDS: types.RDS{
								InstanceID: "my-db",
								ResourceID: "db-123",
							},
						},
					},
				)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedDB, ldr.Databases[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "valid for listing instances with a vpc filter",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				NextToken: "",
				VpcId:     "vpc-2",
			},
			mockInstances: []rdsTypes.DBInstance{
				{
					DBInstanceStatus:     stringPointer("available"),
					DBInstanceIdentifier: stringPointer("my-db-1"),
					Engine:               stringPointer("postgres"),
					DbiResourceId:        stringPointer("db-123"),
					DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
					DBSubnetGroup: &rdsTypes.DBSubnetGroup{
						Subnets: []rdsTypes.Subnet{{SubnetIdentifier: aws.String("subnet-a")}},
						VpcId:   aws.String("vpc-1"),
					},
					Endpoint: &rdsTypes.Endpoint{
						Address: stringPointer("endpoint.amazonaws.com"),
						Port:    aws.Int32(5432),
					},
				},
				{
					DBInstanceStatus:     stringPointer("available"),
					DBInstanceIdentifier: stringPointer("my-db-2"),
					Engine:               stringPointer("postgres"),
					DbiResourceId:        stringPointer("db-123"),
					DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
					DBSubnetGroup: &rdsTypes.DBSubnetGroup{
						Subnets: []rdsTypes.Subnet{{SubnetIdentifier: aws.String("subnet-b")}},
						VpcId:   aws.String("vpc-2"),
					},
					VpcSecurityGroups: []rdsTypes.VpcSecurityGroupMembership{
						{VpcSecurityGroupId: aws.String("")},
						{VpcSecurityGroupId: aws.String("sg-1")},
						{VpcSecurityGroupId: aws.String("sg-2")},
					},
					Endpoint: &rdsTypes.Endpoint{
						Address: stringPointer("endpoint.amazonaws.com"),
						Port:    aws.Int32(5432),
					},
				},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-db-2",
						Description: "RDS instance in ",
						Labels: map[string]string{
							"account-id":         "123456789012",
							"endpoint-type":      "instance",
							"engine":             "postgres",
							"engine-version":     "",
							"region":             "",
							"status":             "available",
							"teleport.dev/cloud": "AWS",
							"vpc-id":             "vpc-2",
						},
					},
					types.DatabaseSpecV3{
						Protocol: "postgres",
						URI:      "endpoint.amazonaws.com:5432",
						AWS: types.AWS{
							AccountID: "123456789012",
							RDS: types.RDS{
								InstanceID: "my-db-2",
								ResourceID: "db-123",
								VPCID:      "vpc-2",
								Subnets:    []string{"subnet-b"},
								SecurityGroups: []string{
									"sg-1",
									"sg-2",
								},
							},
						},
					},
				)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedDB, ldr.Databases[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "listing instances returns all valid instances and ignores the others",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			mockInstances: []rdsTypes.DBInstance{
				{
					DBInstanceStatus:     stringPointer("available"),
					DBInstanceIdentifier: stringPointer("my-db"),
					Engine:               stringPointer("postgres"),
					DbiResourceId:        stringPointer("db-123"),
					DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
					Endpoint: &rdsTypes.Endpoint{
						Address: stringPointer("endpoint.amazonaws.com"),
						Port:    aws.Int32(5432),
					},
				},
				{
					DBInstanceStatus:     stringPointer("creating"),
					DBInstanceIdentifier: stringPointer("db-without-endpoint"),
					Engine:               stringPointer("postgres"),
					DbiResourceId:        stringPointer("db-123"),
					DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
					Endpoint:             nil,
				},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-db",
						Description: "RDS instance in ",
						Labels: map[string]string{
							"account-id":         "123456789012",
							"endpoint-type":      "instance",
							"engine":             "postgres",
							"engine-version":     "",
							"region":             "",
							"status":             "available",
							"teleport.dev/cloud": "AWS",
						},
					},
					types.DatabaseSpecV3{
						Protocol: "postgres",
						URI:      "endpoint.amazonaws.com:5432",
						AWS: types.AWS{
							AccountID: "123456789012",
							RDS: types.RDS{
								InstanceID: "my-db",
								ResourceID: "db-123",
							},
						},
					},
				)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedDB, ldr.Databases[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "valid for listing clusters",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "cluster",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			mockClusters: []rdsTypes.DBCluster{{
				Status:              stringPointer("available"),
				DBClusterIdentifier: stringPointer("my-dbc"),
				DbClusterResourceId: stringPointer("db-123"),
				Engine:              stringPointer("aurora-postgresql"),
				Endpoint:            stringPointer("aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com"),
				Port:                &clusterPort,
				DBClusterArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
			}},
			mockInstances: []rdsTypes.DBInstance{{
				DBClusterIdentifier: stringPointer("my-dbc"),
				DBSubnetGroup: &rdsTypes.DBSubnetGroup{
					Subnets: []rdsTypes.Subnet{{SubnetIdentifier: aws.String("subnet-999")}},
					VpcId:   aws.String("vpc-999"),
				},
			}},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")
				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-dbc",
						Description: "Aurora cluster in ",
						Labels: map[string]string{
							"account-id":         "123456789012",
							"endpoint-type":      "primary",
							"engine":             "aurora-postgresql",
							"engine-version":     "",
							"region":             "",
							"status":             "available",
							"vpc-id":             "vpc-999",
							"teleport.dev/cloud": "AWS",
						},
					},
					types.DatabaseSpecV3{
						Protocol: "postgres",
						URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
						AWS: types.AWS{
							AccountID: "123456789012",
							RDS: types.RDS{
								ClusterID:  "my-dbc",
								InstanceID: "aurora-instance-1",
								ResourceID: "db-123",
								Subnets:    []string{"subnet-999"},
								VPCID:      "vpc-999",
							},
						},
					},
				)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedDB, ldr.Databases[0]))
			},
			errCheck: noErrorFunc,
		},

		{
			name: "valid for listing clusters with vpc filter",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "cluster",
				Engines:   []string{"postgres"},
				NextToken: "",
				VpcId:     "vpc-2",
			},
			mockClusters: []rdsTypes.DBCluster{
				{
					Status:              stringPointer("available"),
					DBClusterIdentifier: stringPointer("my-dbc-1"),
					DbClusterResourceId: stringPointer("db-123"),
					Engine:              stringPointer("aurora-postgresql"),
					Endpoint:            stringPointer("aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com"),
					Port:                &clusterPort,
					DBClusterArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
				},
				{
					Status:              stringPointer("available"),
					DBClusterIdentifier: stringPointer("my-dbc-2"),
					DbClusterResourceId: stringPointer("db-123"),
					Engine:              stringPointer("aurora-postgresql"),
					Endpoint:            stringPointer("aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com"),
					Port:                &clusterPort,
					DBClusterArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
				},
			},
			mockInstances: []rdsTypes.DBInstance{
				{
					DBClusterIdentifier: stringPointer("my-dbc-1"),
					DBSubnetGroup: &rdsTypes.DBSubnetGroup{
						Subnets: []rdsTypes.Subnet{{SubnetIdentifier: aws.String("subnet-1")}},
						VpcId:   aws.String("vpc-1"),
					},
				},
				{
					DBClusterIdentifier: stringPointer("my-dbc-2"),
					DBSubnetGroup: &rdsTypes.DBSubnetGroup{
						Subnets: []rdsTypes.Subnet{{SubnetIdentifier: aws.String("subnet-2")}},
						VpcId:   aws.String("vpc-2"),
					},
				},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")
				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-dbc-2",
						Description: "Aurora cluster in ",
						Labels: map[string]string{
							"account-id":         "123456789012",
							"endpoint-type":      "primary",
							"engine":             "aurora-postgresql",
							"engine-version":     "",
							"region":             "",
							"status":             "available",
							"vpc-id":             "vpc-2",
							"teleport.dev/cloud": "AWS",
						},
					},
					types.DatabaseSpecV3{
						Protocol: "postgres",
						URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
						AWS: types.AWS{
							AccountID: "123456789012",
							RDS: types.RDS{
								ClusterID:  "my-dbc-2",
								InstanceID: "aurora-instance-1",
								ResourceID: "db-123",
								Subnets:    []string{"subnet-2"},
								VPCID:      "vpc-2",
							},
						},
					},
				)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedDB, ldr.Databases[0]))
			},
			errCheck: noErrorFunc,
		},

		{
			name: "listing clusters returns all valid clusters and ignores the others",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "cluster",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			mockInstances: []rdsTypes.DBInstance{{
				DBClusterIdentifier: stringPointer("my-dbc"),
				DBSubnetGroup: &rdsTypes.DBSubnetGroup{
					Subnets: []rdsTypes.Subnet{{SubnetIdentifier: aws.String("subnet-999")}},
					VpcId:   aws.String("vpc-999"),
				},
			}},
			mockClusters: []rdsTypes.DBCluster{
				{
					Status:              stringPointer("available"),
					DBClusterIdentifier: stringPointer("my-empty-cluster"),
					DbClusterResourceId: stringPointer("db-456"),
					Engine:              stringPointer("aurora-mysql"),
					Endpoint:            stringPointer("aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com"),
					Port:                &clusterPort,
					DBClusterArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
				},
				{
					Status:              stringPointer("available"),
					DBClusterIdentifier: stringPointer("my-dbc"),
					DbClusterResourceId: stringPointer("db-123"),
					Engine:              stringPointer("aurora-postgresql"),
					Endpoint:            stringPointer("aurora-instance-2.abcdefghijklmnop.us-west-1.rds.amazonaws.com"),
					Port:                &clusterPort,
					DBClusterArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
				},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")
				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-dbc",
						Description: "Aurora cluster in ",
						Labels: map[string]string{
							"account-id":         "123456789012",
							"endpoint-type":      "primary",
							"engine":             "aurora-postgresql",
							"engine-version":     "",
							"region":             "",
							"status":             "available",
							"vpc-id":             "vpc-999",
							"teleport.dev/cloud": "AWS",
						},
					},
					types.DatabaseSpecV3{
						Protocol: "postgres",
						URI:      "aurora-instance-2.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
						AWS: types.AWS{
							AccountID: "123456789012",
							RDS: types.RDS{
								ClusterID:  "my-dbc",
								InstanceID: "aurora-instance-2",
								ResourceID: "db-123",
								Subnets:    []string{"subnet-999"},
								VPCID:      "vpc-999",
							},
						},
					},
				)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedDB, ldr.Databases[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "no region",
			req: ListDatabasesRequest{
				Region:    "",
				RDSType:   "instance",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			errCheck: trace.IsBadParameter,
		},
		{
			name: "invalid rds type",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "aurora",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			errCheck: trace.IsBadParameter,
		},
		{
			name: "empty engines list",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{},
				NextToken: "",
			},
			errCheck: trace.IsBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockListClient := mockListDatabasesClient{
				dbInstances: tt.mockInstances,
				dbClusters:  tt.mockClusters,
			}
			logger := utils.NewSlogLoggerForTests().With("test", t.Name())
			resp, err := ListDatabases(ctx, mockListClient, logger, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if err != nil {
				return
			}

			tt.respCheck(t, resp)
		})
	}
}
