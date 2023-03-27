/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/stretchr/testify/require"
)

func TestRedisEnterpriseClient(t *testing.T) {
	t.Run("GetToken", func(t *testing.T) {
		tests := []struct {
			name            string
			mockDatabaseAPI armRedisEnterpriseDatabaseClient
			resourceID      string
			expectError     bool
			expectToken     string
		}{
			{
				name:       "access denied",
				resourceID: "cluster-name",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					NoAuth: true,
				},
				expectError: true,
			},
			{
				name:       "succeed (default database name)",
				resourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-teleport",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					TokensByDatabaseName: map[string]string{
						"default": "some-token",
					},
				},
				expectToken: "some-token",
			},
			{
				name:       "succeed (specific database name)",
				resourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-teleport/databases/some-database",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					TokensByDatabaseName: map[string]string{
						"some-database": "some-token",
					},
				},
				expectToken: "some-token",
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				c := NewRedisEnterpriseClientByAPI(nil, test.mockDatabaseAPI)
				token, err := c.GetToken(context.TODO(), test.resourceID)

				if test.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, test.expectToken, token)
				}
			})
		}
	})

	t.Run("List", func(t *testing.T) {
		mockClusterAPI := &ARMRedisEnterpriseClusterMock{
			Clusters: []*armredisenterprise.Cluster{
				makeRedisEnterpriceCluster("redis-prod-1", "group-prod"),
				makeRedisEnterpriceCluster("redis-prod-2", "group-prod"),
				makeRedisEnterpriceCluster("redis-dev", "group-dev"),
			},
		}

		mockDatabaseAPI := &ARMRedisEnterpriseDatabaseMock{
			Databases: []*armredisenterprise.Database{
				makeRedisEnterpriceDatabase("default", "redis-prod-1", "group-prod"),
				makeRedisEnterpriceDatabase("db-x", "redis-prod-2", "group-prod"),
				makeRedisEnterpriceDatabase("db-y", "redis-prod-2", "group-prod"),
				makeRedisEnterpriceDatabase("default", "redis-dev", "group-dev"),
			},
		}

		t.Run("ListALL", func(t *testing.T) {
			t.Parallel()

			expectClusterDatabases := map[string][]string{
				"redis-prod-1": []string{"default"},
				"redis-prod-2": []string{"db-x", "db-y"},
				"redis-dev":    []string{"default"},
			}

			c := NewRedisEnterpriseClientByAPI(mockClusterAPI, mockDatabaseAPI)
			clusters, err := c.ListAll(context.TODO())
			require.NoError(t, err)
			requireClusterDatabases(t, expectClusterDatabases, clusters)
		})
		t.Run("ListWithinGroup", func(t *testing.T) {
			t.Parallel()

			expectClusterDatabases := map[string][]string{
				"redis-prod-1": []string{"default"},
				"redis-prod-2": []string{"db-x", "db-y"},
			}

			c := NewRedisEnterpriseClientByAPI(mockClusterAPI, mockDatabaseAPI)
			clusters, err := c.ListWithinGroup(context.TODO(), "group-prod")
			require.NoError(t, err)
			requireClusterDatabases(t, expectClusterDatabases, clusters)
		})
	})
}

func requireClusterDatabases(t *testing.T, expectClusterDatabases map[string][]string, databases []*RedisEnterpriseDatabase) {
	actualClusterDatabases := make(map[string][]string)
	for _, database := range databases {
		actualClusterDatabases[StringVal(database.Cluster.Name)] = append(actualClusterDatabases[StringVal(database.Cluster.Name)], StringVal(database.Name))
	}
	require.Equal(t, expectClusterDatabases, actualClusterDatabases)
}

func makeRedisEnterpriceCluster(name, group string) *armredisenterprise.Cluster {
	return &armredisenterprise.Cluster{
		Name:     to.Ptr(name),
		ID:       to.Ptr(fmt.Sprintf("/subscriptions/sub-id/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v", group, name)),
		Type:     to.Ptr("Microsoft.Cache/redisEnterprise"),
		Location: to.Ptr("local"),
	}
}

func makeRedisEnterpriceDatabase(name, clusterName, group string) *armredisenterprise.Database {
	return &armredisenterprise.Database{
		Name: to.Ptr(name),
		ID:   to.Ptr(fmt.Sprintf("/subscriptions/sub-id/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v/databases/%v", group, clusterName, name)),
	}
}
