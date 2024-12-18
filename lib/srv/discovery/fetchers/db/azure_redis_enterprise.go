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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureRedisEnterpriseFetcher creates a fetcher for Azure Redis Enterprise.
func newAzureRedisEnterpriseFetcher(config azureFetcherConfig) (common.Fetcher, error) {
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

func (p *azureRedisEnterprisePlugin) NewDatabaseFromServer(ctx context.Context, server *azure.RedisEnterpriseDatabase, logger *slog.Logger) types.Database {
	if server.Properties == nil || server.Cluster.Properties == nil {
		return nil
	}

	if azure.StringVal(server.Properties.ClientProtocol) != string(armredisenterprise.ProtocolEncrypted) {
		logger.DebugContext(ctx, "Skipping Azure Redis Enterprise with unsupported protocol",
			"server", server,
			"protocol", azure.StringVal(server.Properties.ClientProtocol),
		)
		return nil
	}

	if !p.isAvailable(server) {
		logger.DebugContext(ctx, "Skipping unavailable Azure Redis Enterprise server",
			"server", server,
			"status", azure.StringVal(server.Properties.ProvisioningState),
		)
		return nil
	}

	database, err := common.NewDatabaseFromAzureRedisEnterprise(server.Cluster, server.Database)
	if err != nil {
		logger.WarnContext(ctx, "Could not convert Azure Redis Enterprise to database resource",
			"server", server,
			"error", err,
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
		slog.WarnContext(context.Background(), "Assuming Azure Enterprise Redis with unknown status type is available",
			"status", azure.StringVal(server.Properties.ProvisioningState),
			"server", server,
		)
		return true
	}
}
