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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureRedisFetcher creates a fetcher for Azure Redis.
func newAzureRedisFetcher(config azureFetcherConfig) (common.Fetcher, error) {
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

	database, err := common.NewDatabaseFromAzureRedis(server)
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
