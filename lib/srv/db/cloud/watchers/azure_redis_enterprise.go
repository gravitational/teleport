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
	return newAzureFetcher[*azure.RedisEnterpriseCluster, azure.RedisEnterpriseClient](config, &azureRedisEnterprisePlugin{})
}

type azureRedisEnterprisePlugin struct {
}

func (p *azureRedisEnterprisePlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.RedisEnterpriseClient, error) {
	client, err := cfg.AzureClients.GetAzureRedisEnterpriseClient(subID)
	return client, trace.Wrap(err)
}

func (p *azureRedisEnterprisePlugin) GetServerLocation(cluster *azure.RedisEnterpriseCluster) string {
	return stringVal(cluster.Location)
}

func (p *azureRedisEnterprisePlugin) NewDatabasesFromServer(cluster *azure.RedisEnterpriseCluster, log logrus.FieldLogger) types.Databases {
	if cluster.Properties == nil {
		return nil
	}

	var databases types.Databases
	for _, clusterDatabase := range cluster.Databases {
		if clusterDatabase == nil || clusterDatabase.Properties == nil {
			continue
		}

		if stringVal(clusterDatabase.Properties.ClientProtocol) != string(armredisenterprise.ProtocolEncrypted) {
			log.Debugf("Azure Redis Enterprise cluster %v (database %v) is running unsupported protocol %v. Skipping.",
				stringVal(cluster.Name),
				stringVal(clusterDatabase.Name),
				stringVal(clusterDatabase.Properties.ClientProtocol),
			)
			continue
		}

		if !p.isDatabaseAvailable(cluster.Cluster, clusterDatabase) {
			log.Debugf("The current status of Azure Enterprise Redis clsuter %q (database %v) is %q. Skipping.",
				stringVal(clusterDatabase.Properties.ResourceState),
				stringVal(cluster.Name),
				stringVal(clusterDatabase.Name),
			)
			return nil
		}

		database, err := services.NewDatabaseFromAzureRedisEnterprise(cluster.Cluster, clusterDatabase)
		if err != nil {
			log.Warnf("Could not convert Azure Redis Enterprise Redis cluster %q (database %v) to database resource: %v.",
				stringVal(cluster.Name),
				stringVal(clusterDatabase.Name),
				err,
			)
			return nil
		}

		databases = append(databases, database)
	}
	return databases
}

func (p *azureRedisEnterprisePlugin) isDatabaseAvailable(cluster *armredisenterprise.Cluster, clusterDatabase *armredisenterprise.Database) bool {
	switch armredisenterprise.ProvisioningState(stringVal(clusterDatabase.Properties.ResourceState)) {
	case armredisenterprise.ProvisioningStateSucceeded,
		armredisenterprise.ProvisioningStateUpdating:
		return true
	case armredisenterprise.ProvisioningStateCanceled,
		armredisenterprise.ProvisioningStateCreating,
		armredisenterprise.ProvisioningStateDeleting,
		armredisenterprise.ProvisioningStateFailed:
		return false
	default:
		logrus.Warnf("Unknown status type: %q. Assuming Azure Enterprise Redis cluster %q (database %v) is available.",
			stringVal(clusterDatabase.Properties.ProvisioningState),
			stringVal(cluster.Name),
			stringVal(clusterDatabase.Name),
		)
		return true
	}
}
