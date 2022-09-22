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

package watchers

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
)

func newAzureMySQLFetcher(config azureFetcherConfig) (Fetcher, error) {
	return newAzureFetcher[*azure.DBServer, azure.DBServersClient](config, &azureMySQLPlugin{})
}

func newAzurePostgresFetcher(config azureFetcherConfig) (Fetcher, error) {
	return newAzureFetcher[*azure.DBServer, azure.DBServersClient](config, &azurePostgresPlugin{})
}

type azureDBServerPlugin struct {
}

func (p *azureDBServerPlugin) NewDatabasesFromServer(server *azure.DBServer, log logrus.FieldLogger) types.Databases {
	if !server.IsSupported() {
		log.Debugf("Azure server %q (version %v) does not support AAD authentication. Skipping.",
			server.Name,
			server.Properties.Version)
		return nil
	}

	if !server.IsAvailable() {
		log.Debugf("The current status of Azure server %q is %q. Skipping.",
			server.Name,
			server.Properties.UserVisibleState)
		return nil
	}

	database, err := services.NewDatabaseFromAzureServer(server)
	if err != nil {
		log.Warnf("Could not convert Azure server %q to database resource: %v.",
			server.Name,
			err)
		return nil
	}
	return types.Databases{database}
}

func (p *azureDBServerPlugin) GetServerLocation(server *azure.DBServer) string {
	return server.Location
}

type azureMySQLPlugin struct {
	azureDBServerPlugin
}

func (p *azureMySQLPlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.DBServersClient, error) {
	client, err := cfg.AzureClients.GetAzureMySQLClient(subID)
	return client, trace.Wrap(err)
}

type azurePostgresPlugin struct {
	azureDBServerPlugin
}

func (p *azurePostgresPlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.DBServersClient, error) {
	client, err := cfg.AzureClients.GetAzurePostgresClient(subID)
	return client, trace.Wrap(err)
}
