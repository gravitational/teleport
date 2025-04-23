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

package db

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// TestAzureMySQLFlexFetchers tests Azure MySQL Flexible server fetchers.
func TestAzureMySQLFlexFetchers(t *testing.T) {
	t.Parallel()

	azureSub := makeAzureSubscription(t, "sub123")
	azMySQLFlexServer, azMySQLFlexDB := makeAzureMySQLFlexServer(t, "mysql-flex", "sub123", "group 1", "East US", map[string]string{"env": "prod"})
	azureMatchers := []types.AzureMatcher{{
		Types:        []string{types.AzureMatcherMySQL},
		ResourceTags: types.Labels{"env": []string{"prod"}},
		Regions:      []string{"eastus"},
	}}

	clients := &cloud.TestCloudClients{
		AzureSubscriptionClient: azure.NewSubscriptionClient(&azure.ARMSubscriptionsMock{
			Subscriptions: []*armsubscription.Subscription{azureSub},
		}),
		AzureMySQL: azure.NewMySQLServersClient(&azure.ARMMySQLMock{
			NoAuth: true,
		}),
		AzureMySQLFlex: azure.NewMySQLFlexServersClientByAPI(&azure.ARMMySQLFlexServerMock{
			Servers: []*armmysqlflexibleservers.Server{azMySQLFlexServer},
		}),
	}

	fetchers := mustMakeAzureFetchers(t, clients, azureMatchers)
	require.ElementsMatch(t, types.Databases{azMySQLFlexDB}, mustGetDatabases(t, fetchers))
}

func makeAzureMySQLFlexServer(t *testing.T, name, subscription, group, region string, labels map[string]string, opts ...func(*armmysqlflexibleservers.Server)) (*armmysqlflexibleservers.Server, types.Database) {
	resourceType := "Microsoft.DBforMySQL/flexibleServers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".mysql" + azureutils.DatabaseEndpointSuffix
	state := armmysqlflexibleservers.ServerStateReady
	version := armmysqlflexibleservers.ServerVersionEight021
	server := &armmysqlflexibleservers.Server{
		Location: &region,
		Properties: &armmysqlflexibleservers.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			State:                    &state,
			Version:                  &version,
		},
		Tags: labelsToAzureTags(labels),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	for _, opt := range opts {
		opt(server)
	}
	database, err := common.NewDatabaseFromAzureMySQLFlexServer(server)
	require.NoError(t, err)
	common.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherMySQL)
	return server, database
}
