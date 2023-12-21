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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/stretchr/testify/require"
)

func TestMySQLFlexClient(t *testing.T) {
	t.Run("List", func(t *testing.T) {
		mockAPI := &ARMMySQLFlexServerMock{
			Servers: []*armmysqlflexibleservers.Server{
				makeAzureMySQLFlexServer("mysql-flex-prod-1", "group-prod"),
				makeAzureMySQLFlexServer("mysql-flex-prod-2", "group-prod"),
				makeAzureMySQLFlexServer("mysql-flex-dev", "group-dev"),
			},
		}

		t.Run("All", func(t *testing.T) {
			t.Parallel()

			expectServers := []string{"mysql-flex-prod-1", "mysql-flex-prod-2", "mysql-flex-dev"}

			c := NewMySQLFlexServersClientByAPI(mockAPI)
			resources, err := c.ListAll(context.Background())
			require.NoError(t, err)
			requireMySQLFlexServers(t, expectServers, resources)
		})
		t.Run("WithinGroup", func(t *testing.T) {
			t.Parallel()

			expectServers := []string{"mysql-flex-prod-1", "mysql-flex-prod-2"}

			c := NewMySQLFlexServersClientByAPI(mockAPI)
			resources, err := c.ListWithinGroup(context.Background(), "group-prod")
			require.NoError(t, err)
			requireMySQLFlexServers(t, expectServers, resources)
		})
	})
}

func requireMySQLFlexServers(t *testing.T, expectServers []string, servers []*armmysqlflexibleservers.Server) {
	t.Helper()
	require.Len(t, servers, len(expectServers))
	for i, server := range servers {
		require.Equal(t, expectServers[i], StringVal(server.Name))
	}
}

func makeAzureMySQLFlexServer(name, group string) *armmysqlflexibleservers.Server {
	resourceType := "Microsoft.DBforMySQL/flexibleServers"
	return &armmysqlflexibleservers.Server{
		Name: &name,
		ID: to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
			"sub123",
			group,
			resourceType,
			name,
		)),
		Type:     &resourceType,
		Location: to.Ptr("local"),
	}
}
