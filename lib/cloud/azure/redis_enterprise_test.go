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
