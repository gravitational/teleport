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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzurePostgresFlexServerFetcher creates a fetcher for Azure PostgreSQL Flexible server.
func newAzurePostgresFlexServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armpostgresqlflexibleservers.Server, azure.PostgresFlexServersClient](config, &azurePostgresFlexServerFetcher{})
}

// newAzurePostgresFlexServerFetcher implements azureFetcherPlugin for Azure PostgreSQL Flexible server.
type azurePostgresFlexServerFetcher struct{}

// GetListClient returns a server-listing client for Azure PostgreSQL Flexible server.
func (f *azurePostgresFlexServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.PostgresFlexServersClient, error) {
	client, err := cfg.AzureClients.GetAzurePostgresFlexServersClient(subID)
	return client, trace.Wrap(err)
}

// GetServerLocation returns the location of an Azure PostgreSQL Flexible server.
func (f *azurePostgresFlexServerFetcher) GetServerLocation(server *armpostgresqlflexibleservers.Server) string {
	return azure.StringVal(server.Location)
}

// NewDatabaseFromServer converts an Azure PostgreSQL server to a Teleport database.
func (f *azurePostgresFlexServerFetcher) NewDatabaseFromServer(server *armpostgresqlflexibleservers.Server, log logrus.FieldLogger) types.Database {
	if !f.isAvailable(server, log) {
		log.Debugf("The current status of Azure PostgreSQL Flexible server %q is %q. Skipping.",
			azure.StringVal(server.Name),
			azure.StringVal(server.Properties.State))
		return nil
	}

	database, err := services.NewDatabaseFromAzurePostgresFlexServer(server)
	if err != nil {
		log.Warnf("Could not convert Azure PostgreSQL server %q to database resource: %v.", azure.StringVal(server.Name), err)
		return nil
	}
	return database
}

// isAvailable checks the status of the server and returns true if the server
// is available.
func (f *azurePostgresFlexServerFetcher) isAvailable(server *armpostgresqlflexibleservers.Server, log logrus.FieldLogger) bool {
	state := armpostgresqlflexibleservers.ServerState(azure.StringVal(server.Properties.State))
	switch state {
	case armpostgresqlflexibleservers.ServerStateReady, armpostgresqlflexibleservers.ServerStateUpdating:
		return true
	case armpostgresqlflexibleservers.ServerStateDisabled,
		armpostgresqlflexibleservers.ServerStateDropping,
		armpostgresqlflexibleservers.ServerStateStarting,
		armpostgresqlflexibleservers.ServerStateStopped,
		armpostgresqlflexibleservers.ServerStateStopping:
		// server state is known and it's not available.
		return false
	}
	log.Warnf("Unknown status type: %q. Assuming Azure PostgreSQL Flexible server %q is available.",
		state,
		azure.StringVal(server.Name))
	return true
}
