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

// newAzureSQLServer creates a fetcher for Azure SQL Servers.
func newAzureSQLServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armsql.Server, azure.SQLServerClient](config, &azureSQLServerFetcher{})
}

// azureSQLServerFetcher implements azureFetcherPlugin for Azure SQL Servers.
type azureSQLServerFetcher struct{}

func (f *azureSQLServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.SQLServerClient, error) {
	client, err := cfg.AzureClients.GetAzureSQLServerClient(subID)
	return client, trace.Wrap(err)
}

func (f *azureSQLServerFetcher) GetServerLocation(server *armsql.Server) string {
	return azure.StringVal(server.Location)
}

func (f *azureSQLServerFetcher) NewDatabaseFromServer(server *armsql.Server, log logrus.FieldLogger) types.Database {
	database, err := services.NewDatabaseFromAzureSQLServer(server)
	if err != nil {
		log.Warnf("Could not convert Azure SQL server %q to database resource: %v.", azure.StringVal(server.Name), err)
		return nil
	}

	// The method used to list the SQL servers only return running servers so
	// there is no need to check the status here.
	return database
}
