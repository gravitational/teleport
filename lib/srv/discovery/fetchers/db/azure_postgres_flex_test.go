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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// TestAzurePostgresFlexFetchers tests Azure PostgreSQL Flexible server fetchers.
func TestAzurePostgresFlexFetchers(t *testing.T) {
	t.Parallel()

	azureSub := makeAzureSubscription(t, "sub123")
	azPostgresFlexServer, azPostgresFlexDB := makeAzurePostgresFlexServer(t, "postgres-flex", "sub123", "group 1", "East US", map[string]string{"env": "prod"})
	azureMatchers := []types.AzureMatcher{{
		Types:        []string{types.AzureMatcherPostgres},
		ResourceTags: types.Labels{"env": []string{"prod"}},
		Regions:      []string{"eastus"},
	}}

	clients := &cloud.TestCloudClients{
		AzureSubscriptionClient: azure.NewSubscriptionClient(&azure.ARMSubscriptionsMock{
			Subscriptions: []*armsubscription.Subscription{azureSub},
		}),
		AzurePostgres: azure.NewPostgresServerClient(&azure.ARMPostgresMock{
			NoAuth: true,
		}),
		AzurePostgresFlex: azure.NewPostgresFlexServersClientByAPI(&azure.ARMPostgresFlexServerMock{
			Servers: []*armpostgresqlflexibleservers.Server{azPostgresFlexServer},
		}),
	}

	fetchers := mustMakeAzureFetchers(t, clients, azureMatchers)
	require.ElementsMatch(t, types.Databases{azPostgresFlexDB}, mustGetDatabases(t, fetchers))
}

func makeAzurePostgresFlexServer(t *testing.T, name, subscription, group, region string, labels map[string]string, opts ...func(*armpostgresqlflexibleservers.Server)) (*armpostgresqlflexibleservers.Server, types.Database) {
	resourceType := "Microsoft.DBforPostgres/flexibleServers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".postgres" + azureutils.DatabaseEndpointSuffix
	state := armpostgresqlflexibleservers.ServerStateReady
	version := armpostgresqlflexibleservers.ServerVersionFourteen
	server := &armpostgresqlflexibleservers.Server{
		Location: &region,
		Properties: &armpostgresqlflexibleservers.ServerProperties{
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
	database, err := common.NewDatabaseFromAzurePostgresFlexServer(server)
	require.NoError(t, err)
	common.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherPostgres)
	return server, database
}
