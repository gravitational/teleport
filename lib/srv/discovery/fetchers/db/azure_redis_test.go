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

package db

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
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
		Types:        []string{services.AzureMatcherRedis},
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

	database, err := services.NewDatabaseFromAzureRedis(resourceInfo)
	require.NoError(t, err)
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

	database, err := services.NewDatabaseFromAzureRedisEnterprise(armCluster, armDatabase)
	require.NoError(t, err)
	return armCluster, armDatabase, database
}
