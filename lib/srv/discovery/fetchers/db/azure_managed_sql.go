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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
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

	database, err := services.NewDatabaseFromAzureManagedSQLServer(server)
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
