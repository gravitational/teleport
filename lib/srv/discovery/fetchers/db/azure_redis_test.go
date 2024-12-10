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
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// TestAzureRedisFetchers tests Azure Redis and Azure Redis Enterprise fetchers
// (as they share the same matcher type).
func TestAzureRedisFetchers(t *testing.T) {
	t.Parallel()

	azureSub := makeAzureSubscription(t, "sub")

	// Note that the Azure Redis APIs may return location in their display
	// names (eg. "East US"). The Azure fetcher should normalize location names
	// so region matcher "eastus" will match "East US".
	azRedisServer, azRedisDB := makeAzureRedisServer(t, "redis", "sub", "group", "East US", map[string]string{"env": "prod"})
	azRedisEnterpriseCluster, azRedisEnterpriseDatabase, azRedisEnterpriseDB := makeAzureRedisEnterpriseCluster(t, "redis-enterprise", "sub", "group", "eastus", map[string]string{"env": "prod"})

	azureMatchers := []types.AzureMatcher{{
		Types:        []string{types.AzureMatcherRedis},
		ResourceTags: types.Labels{"env": []string{"prod"}},
		Regions:      []string{"eastus"},
	}}

	clients := &cloud.TestCloudClients{
		AzureSubscriptionClient: azure.NewSubscriptionClient(&azure.ARMSubscriptionsMock{
			Subscriptions: []*armsubscription.Subscription{azureSub},
		}),
		AzureRedis: azure.NewRedisClientByAPI(&azure.ARMRedisMock{
			Servers: []*armredis.ResourceInfo{azRedisServer},
		}),
		AzureRedisEnterprise: azure.NewRedisEnterpriseClientByAPI(
			&azure.ARMRedisEnterpriseClusterMock{
				Clusters: []*armredisenterprise.Cluster{azRedisEnterpriseCluster},
			},
			&azure.ARMRedisEnterpriseDatabaseMock{
				Databases: []*armredisenterprise.Database{azRedisEnterpriseDatabase},
			},
		),
	}

	fetchers := mustMakeAzureFetchers(t, clients, azureMatchers)
	require.ElementsMatch(t, types.Databases{azRedisDB, azRedisEnterpriseDB}, mustGetDatabases(t, fetchers))
}

func makeAzureRedisServer(t *testing.T, name, subscription, group, region string, labels map[string]string) (*armredis.ResourceInfo, types.Database) {
	resourceInfo := &armredis.ResourceInfo{
		Name:     to.Ptr(name),
		ID:       to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/Redis/%v", subscription, group, name)),
		Location: to.Ptr(region),
		Tags:     labelsToAzureTags(labels),
		Properties: &armredis.Properties{
			HostName:          to.Ptr(fmt.Sprintf("%v.redis.cache.windows.net", name)),
			SSLPort:           to.Ptr(int32(6380)),
			ProvisioningState: to.Ptr(armredis.ProvisioningStateSucceeded),
		},
	}

	database, err := common.NewDatabaseFromAzureRedis(resourceInfo)
	require.NoError(t, err)
	common.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherRedis)
	return resourceInfo, database
}

func makeAzureRedisEnterpriseCluster(t *testing.T, cluster, subscription, group, region string, labels map[string]string) (*armredisenterprise.Cluster, *armredisenterprise.Database, types.Database) {
	armCluster := &armredisenterprise.Cluster{
		Name:     to.Ptr(cluster),
		ID:       to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v", subscription, group, cluster)),
		Location: to.Ptr(region),
		Tags:     labelsToAzureTags(labels),
		Properties: &armredisenterprise.ClusterProperties{
			HostName: to.Ptr(fmt.Sprintf("%v.%v.redisenterprise.cache.azure.net", cluster, region)),
		},
	}
	armDatabase := &armredisenterprise.Database{
		Name: to.Ptr("default"),
		ID:   to.Ptr(fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v/databases/default", subscription, group, cluster)),
		Properties: &armredisenterprise.DatabaseProperties{
			ProvisioningState: to.Ptr(armredisenterprise.ProvisioningStateSucceeded),
			Port:              to.Ptr(int32(10000)),
			ClusteringPolicy:  to.Ptr(armredisenterprise.ClusteringPolicyOSSCluster),
			ClientProtocol:    to.Ptr(armredisenterprise.ProtocolEncrypted),
		},
	}

	database, err := common.NewDatabaseFromAzureRedisEnterprise(armCluster, armDatabase)
	require.NoError(t, err)
	common.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherRedis)
	return armCluster, armDatabase, database
}
