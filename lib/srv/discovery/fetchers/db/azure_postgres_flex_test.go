// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/gravitational/teleport/lib/services"
)

// TestAzurePostgresFlexFetchers tests Azure PostgreSQL Flexible server fetchers.
func TestAzurePostgresFlexFetchers(t *testing.T) {
	t.Parallel()

	azureSub := makeAzureSubscription(t, "sub123")
	azPostgresFlexServer, azPostgresFlexDB := makeAzurePostgresFlexServer(t, "postgres-flex", "sub123", "group 1", "East US", map[string]string{"env": "prod"})
	azureMatchers := []types.AzureMatcher{{
		Types:        []string{services.AzureMatcherPostgres},
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
	database, err := services.NewDatabaseFromAzurePostgresFlexServer(server)
	require.NoError(t, err)
	return server, database
}
