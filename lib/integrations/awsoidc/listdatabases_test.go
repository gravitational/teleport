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
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func stringPointer(s string) *string {
	return &s
}

func TestDBInstanceConverter(t *testing.T) {
	for _, tt := range []struct {
		name       string
		rdsDB      types.DBInstance
		errCheck   require.ErrorAssertionFunc
		expectedDB *AWSDatabase
	}{
		{
			name: "all fields",
			rdsDB: types.DBInstance{
				DBInstanceStatus:     stringPointer("available"),
				DBInstanceIdentifier: stringPointer("name"),
				Engine:               stringPointer("postgres"),
				Endpoint: &types.Endpoint{
					Address: stringPointer("endpoint.amazonaws.com"),
					Port:    1234,
				},
				TagList: []types.Tag{
					{Key: stringPointer("X"), Value: stringPointer("Y")},
				},
			},
			errCheck: require.NoError,
			expectedDB: &AWSDatabase{
				Name:     "name",
				Engine:   "postgres",
				Status:   "available",
				Endpoint: "endpoint.amazonaws.com:1234",
				Labels:   map[string]string{"X": "Y"},
			},
		},
		{
			name: "no name",
			rdsDB: types.DBInstance{
				DBInstanceStatus: stringPointer("available"),
				Engine:           stringPointer("postgres"),
				Endpoint: &types.Endpoint{
					Address: stringPointer("endpoint.amazonaws.com"),
					Port:    1234,
				},
			},
			errCheck: require.Error,
		},
		{
			name: "no endpoint yet",
			rdsDB: types.DBInstance{
				DBInstanceStatus:     stringPointer("available"),
				DBInstanceIdentifier: stringPointer("name"),
				Engine:               stringPointer("postgres"),
				Endpoint:             nil,
			},
			errCheck: require.NoError,
			expectedDB: &AWSDatabase{
				Name:     "name",
				Engine:   "postgres",
				Status:   "available",
				Endpoint: "",
				Labels:   map[string]string{},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotDB, err := convertDBInstanceToDatabase(tt.rdsDB)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expectedDB, gotDB)
		})
	}
}

func TestDBClusterConverter(t *testing.T) {
	for _, tt := range []struct {
		name       string
		rdsDB      types.DBCluster
		errCheck   require.ErrorAssertionFunc
		expectedDB *AWSDatabase
	}{
		{
			name: "all fields",
			rdsDB: types.DBCluster{
				Status:              stringPointer("available"),
				DBClusterIdentifier: stringPointer("name"),
				Engine:              stringPointer("postgres"),
				Endpoint:            stringPointer("endpoint.amazonaws.com:1234"),
				TagList: []types.Tag{
					{Key: stringPointer("X"), Value: stringPointer("Y")},
				},
			},
			errCheck: require.NoError,
			expectedDB: &AWSDatabase{
				Name:     "name",
				Engine:   "postgres",
				Status:   "available",
				Endpoint: "endpoint.amazonaws.com:1234",
				Labels:   map[string]string{"X": "Y"},
			},
		},
		{
			name: "no name",
			rdsDB: types.DBCluster{
				Status:   stringPointer("available"),
				Engine:   stringPointer("postgres"),
				Endpoint: stringPointer("endpoint.amazonaws.com:1234"),
			},
			errCheck: require.Error,
		},
		{
			name: "no endpoint yet",
			rdsDB: types.DBCluster{
				Status:              stringPointer("available"),
				DBClusterIdentifier: stringPointer("name"),
				Engine:              stringPointer("postgres"),
				Endpoint:            nil,
			},
			errCheck: require.NoError,
			expectedDB: &AWSDatabase{
				Name:     "name",
				Engine:   "postgres",
				Status:   "available",
				Endpoint: "",
				Labels:   map[string]string{},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotDB, err := convertDBClusterToDatabase(tt.rdsDB)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expectedDB, gotDB)
		})
	}
}

type mockListDatabasesClient struct {
	pageSize    int
	dbInstances []types.DBInstance
	dbClusters  []types.DBCluster
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
	pageSize := 100

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	t.Run("pagination", func(t *testing.T) {
		totalDBs := 203

		allInstances := make([]types.DBInstance, 0, totalDBs)
		for i := 0; i < totalDBs; i++ {
			allInstances = append(allInstances, types.DBInstance{
				DBInstanceStatus:     stringPointer("available"),
				DBInstanceIdentifier: stringPointer(uuid.NewString()),
				Engine:               stringPointer("postgres"),
				Endpoint: &types.Endpoint{
					Address: stringPointer("address.amazonaws.com"),
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
		mockInstances []types.DBInstance
		mockClusters  []types.DBCluster
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
			mockInstances: []types.DBInstance{{
				DBInstanceStatus:     stringPointer("available"),
				DBInstanceIdentifier: stringPointer("my-db"),
				Engine:               stringPointer("postgres"),
				Endpoint: &types.Endpoint{
					Address: stringPointer("address.amazonaws.com"),
					Port:    5432,
				},
			},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")
				require.Equal(t, ldr.Databases[0], AWSDatabase{
					Name:     "my-db",
					Engine:   "postgres",
					Status:   "available",
					Endpoint: "address.amazonaws.com:5432",
					Labels:   make(map[string]string),
				})
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
			mockClusters: []types.DBCluster{{
				Status:              stringPointer("available"),
				DBClusterIdentifier: stringPointer("my-db"),
				Engine:              stringPointer("aurora-postgres"),
				Endpoint:            stringPointer("address.amazonaws.com:5432"),
			},
			},
			respCheck: func(t *testing.T, ldr *ListDatabasesResponse) {
				require.Len(t, ldr.Databases, 1, "expected 1 database, got %d", len(ldr.Databases))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")
				require.Equal(t, ldr.Databases[0], AWSDatabase{
					Name:     "my-db",
					Engine:   "aurora-postgres",
					Status:   "available",
					Endpoint: "address.amazonaws.com:5432",
					Labels:   make(map[string]string),
				})
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
			tt.errCheck(err)
			if err != nil {
				return
			}

			tt.respCheck(t, resp)
		})
	}
}
