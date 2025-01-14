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
	"context"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/gravitational/trace"

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

func (p *azureRedisPlugin) NewDatabaseFromServer(ctx context.Context, server *armredis.ResourceInfo, logger *slog.Logger) types.Database {
	if server.Properties.SSLPort == nil { // should never happen, but checking just in case.
		logger.DebugContext(ctx, "Skipping Azure Redis server with missing SSL port", "server", azure.StringVal(server.Name))
		return nil
	}

	if !p.isAvailable(server) {
		logger.DebugContext(ctx, "Skipping unavailable Azure Redis server",
			"server", azure.StringVal(server.Name),
			"status", azure.StringVal(server.Properties.ProvisioningState),
		)
		return nil
	}

	database, err := common.NewDatabaseFromAzureRedis(server)
	if err != nil {
		logger.WarnContext(ctx, "Could not convert Azure Redis server to database resource",
			"server", azure.StringVal(server.Name),
			"error", err,
		)
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
		slog.WarnContext(context.Background(), "Assuming Azure Redis with unknown status type is available",
			"status", azure.StringVal(server.Properties.ProvisioningState),
			"server", azure.StringVal(server.Name),
		)
		return true
	}
}
