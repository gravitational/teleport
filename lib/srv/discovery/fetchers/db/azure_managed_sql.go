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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureManagedSQLServerFetcher creates a fetcher for Azure SQL Servers.
func newAzureManagedSQLServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armsql.ManagedInstance, azure.ManagedSQLServerClient](config, &azureManagedSQLServerFetcher{})
}

// azureManagedSQLServerFetcher implements azureFetcherPlugin for Azure Managed
// SQL Servers.
type azureManagedSQLServerFetcher struct{}

func (f *azureManagedSQLServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.ManagedSQLServerClient, error) {
	client, err := cfg.AzureClients.GetAzureManagedSQLServerClient(subID)
	return client, trace.Wrap(err)
}

func (f *azureManagedSQLServerFetcher) GetServerLocation(server *armsql.ManagedInstance) string {
	return azure.StringVal(server.Location)
}

func (f *azureManagedSQLServerFetcher) NewDatabaseFromServer(server *armsql.ManagedInstance, log logrus.FieldLogger) types.Database {
	if !f.isAvailable(server) {
		log.Debugf("The current status of Azure Managed SQL server %q is %q. Skipping.",
			azure.StringVal(server.Name),
			azure.StringVal(server.Properties.ProvisioningState))
		return nil
	}

	database, err := common.NewDatabaseFromAzureManagedSQLServer(server)
	if err != nil {
		log.Warnf("Could not convert Azure Managed SQL server %q to database resource: %v.", azure.StringVal(server.Name), err)
		return nil
	}

	return database
}

// isAvailable checks the status of the server and returns true if the server
// is available.
func (f *azureManagedSQLServerFetcher) isAvailable(server *armsql.ManagedInstance) bool {
	switch armsql.ManagedInstancePropertiesProvisioningState(azure.StringVal(server.Properties.ProvisioningState)) {
	case armsql.ManagedInstancePropertiesProvisioningStateAccepted,
		armsql.ManagedInstancePropertiesProvisioningStateCanceled,
		armsql.ManagedInstancePropertiesProvisioningStateCreating,
		armsql.ManagedInstancePropertiesProvisioningStateDeleted,
		armsql.ManagedInstancePropertiesProvisioningStateDeleting,
		armsql.ManagedInstancePropertiesProvisioningStateFailed,
		armsql.ManagedInstancePropertiesProvisioningStateNotSpecified,
		armsql.ManagedInstancePropertiesProvisioningStateTimedOut,
		armsql.ManagedInstancePropertiesProvisioningStateRegistering,
		armsql.ManagedInstancePropertiesProvisioningStateUnknown,
		armsql.ManagedInstancePropertiesProvisioningStateUnrecognized:
		return false
	case armsql.ManagedInstancePropertiesProvisioningStateCreated,
		armsql.ManagedInstancePropertiesProvisioningStateRunning,
		armsql.ManagedInstancePropertiesProvisioningStateSucceeded,
		armsql.ManagedInstancePropertiesProvisioningStateUpdating:
		return true
	default:
		logrus.Warnf("Unknown status type: %q. Assuming Managed SQL Server %q is available.",
			azure.StringVal(server.Properties.ProvisioningState),
			azure.StringVal(server.Name),
		)
		return true
	}
}
