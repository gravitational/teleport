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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureMySQLFlexServerFetcher creates a fetcher for Azure MySQL Flexible server.
func newAzureMySQLFlexServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armmysqlflexibleservers.Server, azure.MySQLFlexServersClient](config, &azureMySQLFlexServerFetcher{})
}

// azureMySQLFlexServerFetcher implements azureFetcherPlugin for Azure MySQL Flexible server.
type azureMySQLFlexServerFetcher struct{}

// GetListClient returns a server-listing client for Azure MySQL Flexible server.
func (f *azureMySQLFlexServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.MySQLFlexServersClient, error) {
	client, err := cfg.AzureClients.GetAzureMySQLFlexServersClient(subID)
	return client, trace.Wrap(err)
}

// GetServerLocation returns the location of an Azure MySQL Flexible server.
func (f *azureMySQLFlexServerFetcher) GetServerLocation(server *armmysqlflexibleservers.Server) string {
	return azure.StringVal(server.Location)
}

// NewDatabaseFromServer converts an Azure MySQL Flexible server to a Teleport database.
func (f *azureMySQLFlexServerFetcher) NewDatabaseFromServer(server *armmysqlflexibleservers.Server, log logrus.FieldLogger) types.Database {
	if !f.isAvailable(server, log) {
		log.Debugf("The current status of Azure MySQL Flexible server %q is %q. Skipping.",
			azure.StringVal(server.Name),
			azure.StringVal(server.Properties.State))
		return nil
	}

	database, err := services.NewDatabaseFromAzureMySQLFlexServer(server)
	if err != nil {
		log.Warnf("Could not convert Azure MySQL server %q to database resource: %v.", azure.StringVal(server.Name), err)
		return nil
	}
	return database
}

// isAvailable checks the status of the server and returns true if the server
// is available.
func (f *azureMySQLFlexServerFetcher) isAvailable(server *armmysqlflexibleservers.Server, log logrus.FieldLogger) bool {
	state := armmysqlflexibleservers.ServerState(azure.StringVal(server.Properties.State))
	switch state {
	case armmysqlflexibleservers.ServerStateReady, armmysqlflexibleservers.ServerStateUpdating:
		return true
	case armmysqlflexibleservers.ServerStateDisabled,
		armmysqlflexibleservers.ServerStateDropping,
		armmysqlflexibleservers.ServerStateStarting,
		armmysqlflexibleservers.ServerStateStopped,
		armmysqlflexibleservers.ServerStateStopping:
		// server state is known and it's not available.
		return false
	}
	log.Warnf("Unknown status type: %q. Assuming Azure MySQL Flexible server %q is available.",
		state,
		azure.StringVal(server.Name))
	return true
}
