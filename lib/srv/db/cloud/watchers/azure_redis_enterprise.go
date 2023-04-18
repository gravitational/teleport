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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
)

func newAzureRedisEnterpriseFetcher(config azureFetcherConfig) (Fetcher, error) {
	return newAzureFetcher[*azure.RedisEnterpriseDatabase, azure.RedisEnterpriseClient](config, &azureRedisEnterprisePlugin{})
}

type azureRedisEnterprisePlugin struct{}

func (p *azureRedisEnterprisePlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.RedisEnterpriseClient, error) {
	client, err := cfg.AzureClients.GetAzureRedisEnterpriseClient(subID)
	return client, trace.Wrap(err)
}

func (p *azureRedisEnterprisePlugin) GetServerLocation(server *azure.RedisEnterpriseDatabase) string {
	return azure.StringVal(server.Cluster.Location)
}

func (p *azureRedisEnterprisePlugin) NewDatabaseFromServer(server *azure.RedisEnterpriseDatabase, log logrus.FieldLogger) types.Database {
	if server.Properties == nil || server.Cluster.Properties == nil {
		return nil
	}

	if azure.StringVal(server.Properties.ClientProtocol) != string(armredisenterprise.ProtocolEncrypted) {
		log.Debugf("Azure Redis Enterprise %v is running unsupported protocol %v. Skipping.",
			server,
			azure.StringVal(server.Properties.ClientProtocol),
		)
		return nil
	}

	if !p.isAvailable(server) {
		log.Debugf("The current status of Azure Redis Enterprise %v is %q. Skipping.",
			server,
			azure.StringVal(server.Properties.ProvisioningState),
		)
		return nil
	}

	database, err := services.NewDatabaseFromAzureRedisEnterprise(server.Cluster, server.Database)
	if err != nil {
		log.Warnf("Could not convert Azure Redis Enterprise %v to database resource: %v.",
			server,
			err,
		)
		return nil
	}

	return database
}

// isAvailable checks the status of the database and returns true if the
// database is available.
func (p *azureRedisEnterprisePlugin) isAvailable(server *azure.RedisEnterpriseDatabase) bool {
	switch armredisenterprise.ProvisioningState(azure.StringVal(server.Properties.ProvisioningState)) {
	case armredisenterprise.ProvisioningStateSucceeded,
		armredisenterprise.ProvisioningStateUpdating:
		return true
	case armredisenterprise.ProvisioningStateCanceled,
		armredisenterprise.ProvisioningStateCreating,
		armredisenterprise.ProvisioningStateDeleting,
		armredisenterprise.ProvisioningStateFailed:
		return false
	default:
		logrus.Warnf("Unknown status type: %q. Assuming Azure Enterprise Redis %v is available.",
			azure.StringVal(server.Properties.ProvisioningState),
			server,
		)
		return true
	}
}
