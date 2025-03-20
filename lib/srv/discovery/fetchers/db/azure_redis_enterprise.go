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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

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

	database, err := common.NewDatabaseFromAzureRedisEnterprise(server.Cluster, server.Database)
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
