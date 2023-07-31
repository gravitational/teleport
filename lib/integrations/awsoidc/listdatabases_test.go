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
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdsTypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func stringPointer(s string) *string {
	return &s
}

type mockListDatabasesClient struct {
	pageSize    int
	dbInstances []rdsTypes.DBInstance
	dbClusters  []rdsTypes.DBCluster
}

// Returns information about provisioned RDS instances.
// This API supports pagination.
func (m mockListDatabasesClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	requestedPage := 1

	totalInstances := len(m.dbInstances)

	if params.Marker != nil {
		currentMarker, err := strconv.Atoi(*params.Marker)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalInstances {
		sliceEnd = totalInstances
	}

	ret := &rds.DescribeDBInstancesOutput{
		DBInstances: m.dbInstances[sliceStart:sliceEnd],
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

	pageSize := 100
	t.Run("pagination", func(t *testing.T) {
		totalDBs := 203

		allInstances := make([]rdsTypes.DBInstance, 0, totalDBs)
		for i := 0; i < totalDBs; i++ {
			allInstances = append(allInstances, rdsTypes.DBInstance{
				DBInstanceStatus:     stringPointer("available"),
				DBInstanceIdentifier: stringPointer(fmt.Sprintf("db-%v", i)),
				DbiResourceId:        stringPointer("db-123"),
				DBInstanceArn:        stringPointer("arn:aws:iam::123456789012:role/MyARN"),
				Engine:               stringPointer("postgres"),
				Endpoint: &rdsTypes.Endpoint{
					Address: stringPointer("endpoint.amazonaws.com"),
					Port:    5432,
				},
			})
		}

		mockListClient := mockListDatabasesClient{
			pageSize:    pageSize,
			dbInstances: allInstances,
		}

		// First page must return pageSize number of DBs
		resp, err := ListDatabases(ctx, mockListClient, ListDatabasesRequest{
			Region:    "us-east-1",
			RDSType:   "instance",
			Engines:   []string{"postgres"},
			NextToken: "",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.Databases, pageSize)
		nextPageToken := resp.NextToken

		// Second page must return pageSize number of DBs
		resp, err = ListDatabases(ctx, mockListClient, ListDatabasesRequest{
			Region:    "us-east-1",
			RDSType:   "instance",
			Engines:   []string{"postgres"},
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.Databases, pageSize)
		nextPageToken = resp.NextToken

		// Third page must return only the remaining DBs and an empty nextToken
		resp, err = ListDatabases(ctx, mockListClient, ListDatabasesRequest{
			Region:    "us-east-1",
			RDSType:   "instance",
			Engines:   []string{"postgres"},
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.Databases, 3)
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
					Port:    5432,
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
							"account-id":     "123456789012",
							"endpoint-type":  "instance",
							"engine":         "postgres",
							"engine-version": "",
							"region":         "",
							"status":         "available",
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
				require.Equal(t, expectedDB, ldr.Databases[0])
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
						Port:    5432,
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
							"account-id":     "123456789012",
							"endpoint-type":  "instance",
							"engine":         "postgres",
							"engine-version": "",
							"region":         "",
							"status":         "available",
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
				require.Equal(t, expectedDB, ldr.Databases[0])
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
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")
				expectedDB, err := types.NewDatabaseV3(
					types.Metadata{
						Name:        "my-dbc",
						Description: "Aurora cluster in ",
						Labels: map[string]string{
							"account-id":     "123456789012",
							"endpoint-type":  "primary",
							"engine":         "aurora-postgresql",
							"engine-version": "",
							"region":         "",
							"status":         "available",
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
							},
						},
					},
				)
				require.NoError(t, err)
				require.Equal(t, expectedDB, ldr.Databases[0])
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
			errCheck: func(err error) bool {
				return trace.IsBadParameter(err)
			},
		},
		{
			name: "invalid rds type",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "aurora",
				Engines:   []string{"postgres"},
				NextToken: "",
			},
			errCheck: func(err error) bool {
				return trace.IsBadParameter(err)
			},
		},
		{
			name: "empty engines list",
			req: ListDatabasesRequest{
				Region:    "us-east-1",
				RDSType:   "instance",
				Engines:   []string{},
				NextToken: "",
			},
			errCheck: func(err error) bool {
				return trace.IsBadParameter(err)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockListClient := mockListDatabasesClient{
				pageSize:    pageSize,
				dbInstances: tt.mockInstances,
				dbClusters:  tt.mockClusters,
			}
			resp, err := ListDatabases(ctx, mockListClient, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if err != nil {
				return
			}

			tt.respCheck(t, resp)
		})
	}
}
