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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
)

// newAzureRedisFetcher creates a fetcher for Azure Redis.
func newAzureRedisFetcher(config azureFetcherConfig) (Fetcher, error) {
	return newAzureFetcher[*armredis.ResourceInfo, azure.RedisClient](config, &azureRedisPlugin{})
}

// azureRedisPlugin implements azureFetcherPlugin for Azure Redis.
type azureRedisPlugin struct{}

func (p *azureRedisPlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.RedisClient, error) {
	client, err := cfg.AzureClients.GetAzureRedisClient(subID)
	return client, trace.Wrap(err)
}

func (p *azureRedisPlugin) GetServerLocation(server *armredis.ResourceInfo) string {
	return azure.StringVal(server.Location)
}

func (p *azureRedisPlugin) NewDatabaseFromServer(server *armredis.ResourceInfo, log logrus.FieldLogger) types.Database {
	if server.Properties.SSLPort == nil { // should never happen, but checking just in case.
		log.Debugf("Azure Redis server %v is missing SSL port. Skipping.", azure.StringVal(server.Name))
		return nil
	}

	if !p.isAvailable(server) {
		log.Debugf("The current status of Azure Redis server %q is %q. Skipping.",
			azure.StringVal(server.Name),
			azure.StringVal(server.Properties.ProvisioningState))
		return nil
	}

	database, err := services.NewDatabaseFromAzureRedis(server)
	if err != nil {
		log.Warnf("Could not convert Azure Redis server %q to database resource: %v.", azure.StringVal(server.Name), err)
		return nil
	}
	return database
}

// isAvailable checks the status of the server and returns true if the server
// is available.
func (p *azureRedisPlugin) isAvailable(server *armredis.ResourceInfo) bool {
	switch armredis.ProvisioningState(azure.StringVal(server.Properties.ProvisioningState)) {
	case armredis.ProvisioningStateSucceeded,
		armredis.ProvisioningStateLinking,
		armredis.ProvisioningStateRecoveringScaleFailure,
		armredis.ProvisioningStateScaling,
		armredis.ProvisioningStateUnlinking,
		armredis.ProvisioningStateUpdating:
		return true
	case armredis.ProvisioningStateCreating,
		armredis.ProvisioningStateDeleting,
		armredis.ProvisioningStateDisabled,
		armredis.ProvisioningStateFailed,
		armredis.ProvisioningStateProvisioning,
		armredis.ProvisioningStateUnprovisioning:
		return false
	default:
		logrus.Warnf("Unknown status type: %q. Assuming Azure Redis %q is available.",
			azure.StringVal(server.Properties.ProvisioningState),
			azure.StringVal(server.Name),
		)
		return true
	}
}
