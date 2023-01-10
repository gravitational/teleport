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
