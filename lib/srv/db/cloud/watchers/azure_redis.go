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

func newAzureRedisFetcher(config azureFetcherConfig) (Fetcher, error) {
	return newAzureFetcher[*armredis.ResourceInfo, azure.RedisClient](config, &azureRedisPlugin{})
}

type azureRedisPlugin struct {
}

func (p *azureRedisPlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.RedisClient, error) {
	client, err := cfg.AzureClients.GetAzureRedisClient(subID)
	return client, trace.Wrap(err)
}

func (p *azureRedisPlugin) GetServerLocation(server *armredis.ResourceInfo) string {
	return stringVal(server.Location)
}

func (p *azureRedisPlugin) NewDatabasesFromServer(server *armredis.ResourceInfo, log logrus.FieldLogger) types.Databases {
	if server.Properties.SSLPort == nil { // should never happen, but checking just in case.
		log.Debugf("Azure Redis server %v is missing SSL port. Skipping.", stringVal(server.Name))
		return nil
	}

	if !p.isServerAvailable(server) {
		log.Debugf("The current status of Azure Redis server %q is %q. Skipping.",
			stringVal(server.Name),
			stringVal(server.Properties.ProvisioningState))
		return nil
	}

	database, err := services.NewDatabaseFromAzureRedis(server)
	if err != nil {
		log.Warnf("Could not convert Azure Redis server %q to database resource: %v.", stringVal(server.Name), err)
		return nil
	}
	return types.Databases{database}
}

func (s *azureRedisPlugin) isServerAvailable(server *armredis.ResourceInfo) bool {
	switch armredis.ProvisioningState(stringVal(server.Properties.ProvisioningState)) {
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
			stringVal(server.Properties.ProvisioningState),
			stringVal(server.Name),
		)
		return true
	}
}

func stringVal[T ~string](s *T) string {
	if s != nil {
		return string(*s)
	}
	return ""
}
