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

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// armRedisEnterpriseDatabaseClient is an interface defines a subset of
// functions of armredisenterprise.DatabaseClient
type armRedisEnterpriseDatabaseClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, clusterName string, databaseName string, options *armredisenterprise.DatabasesClientListKeysOptions) (armredisenterprise.DatabasesClientListKeysResponse, error)
	NewListByClusterPager(resourceGroupName string, clusterName string, options *armredisenterprise.DatabasesClientListByClusterOptions) *runtime.Pager[armredisenterprise.DatabasesClientListByClusterResponse]
}

// armRedisEnterpriseClusterClient is an interface defines a subset of
// functions of armredisenterprise.Client
type armRedisEnterpriseClusterClient interface {
	NewListPager(options *armredisenterprise.ClientListOptions) *runtime.Pager[armredisenterprise.ClientListResponse]
	NewListByResourceGroupPager(resourceGroupName string, options *armredisenterprise.ClientListByResourceGroupOptions) *runtime.Pager[armredisenterprise.ClientListByResourceGroupResponse]
}

// redisEnterpriseClient is an Azure Redis Enterprise client.
type redisEnterpriseClient struct {
	clusterAPI  armRedisEnterpriseClusterClient
	databaseAPI armRedisEnterpriseDatabaseClient
}

// NewRedisEnterpriseClient creates a new Azure Redis Enterprise client by
// subscription and credentials.
func NewRedisEnterpriseClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (RedisEnterpriseClient, error) {
	logrus.Debug("Initializing Azure Redis Enterprise client.")
	clusterAPI, err := armredisenterprise.NewClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databaseAPI, err := armredisenterprise.NewDatabasesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRedisEnterpriseClientByAPI(clusterAPI, databaseAPI), nil
}

// NewRedisEnterpriseClientByAPI creates a new Azure Redis Enterprise client by
// ARM API client(s).
func NewRedisEnterpriseClientByAPI(clusterAPI armRedisEnterpriseClusterClient, databaseAPI armRedisEnterpriseDatabaseClient) RedisEnterpriseClient {
	return &redisEnterpriseClient{
		clusterAPI:  clusterAPI,
		databaseAPI: databaseAPI,
	}
}

// GetToken retrieves the auth token for provided resource group and resource
// name.
func (c *redisEnterpriseClient) GetToken(ctx context.Context, resourceID string) (string, error) {
	id, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	clusterName, databaseName, err := c.getClusterAndDatabaseName(id)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err := c.databaseAPI.ListKeys(ctx, id.ResourceGroupName, clusterName, databaseName, &armredisenterprise.DatabasesClientListKeysOptions{})
	if err != nil {
		return "", trace.Wrap(ConvertResponseError(err))
	}

	// There are two keys. Pick first one available.
	if resp.PrimaryKey != nil {
		return *resp.PrimaryKey, nil
	}
	if resp.SecondaryKey != nil {
		return *resp.SecondaryKey, nil
	}
	return "", trace.NotFound("missing keys")
}

// getClusterAndDatabaseName returns the cluster name and the database name
// based on the resource ID. Both armredisenterprise.Cluster.ID and
// armredisenterprise.Database.ID are supported.
func (c *redisEnterpriseClient) getClusterAndDatabaseName(id *arm.ResourceID) (string, string, error) {
	switch id.ResourceType.String() {
	case "Microsoft.Cache/redisEnterprise":
		// It appears an Enterprise cluster always has only one "database", and
		// the database name is always "default".
		return id.Name, RedisEnterpriseClusterDefaultDatabase, nil
	case "Microsoft.Cache/redisEnterprise/databases":
		return id.Parent.Name, id.Name, nil
	default:
		return "", "", trace.BadParameter("unknown Azure Cache for Redis resource type: %v", id.ResourceType)
	}
}

// ListAll returns all Azure Redis Enterprise clusters within an Azure subscription.
func (c *redisEnterpriseClient) ListAll(ctx context.Context) ([]*RedisEnterpriseCluster, error) {
	var allClusters []*RedisEnterpriseCluster
	pager := c.clusterAPI.NewListPager(&armredisenterprise.ClientListOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		clusters, err := c.listByClusters(ctx, page.Value)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		allClusters = append(allClusters, clusters...)
	}
	return allClusters, nil
}

// ListWithinGroup returns all Azure Redis Enterprise clusters within an Azure resource group.
func (c *redisEnterpriseClient) ListWithinGroup(ctx context.Context, group string) ([]*RedisEnterpriseCluster, error) {
	var allClusters []*RedisEnterpriseCluster
	pager := c.clusterAPI.NewListByResourceGroupPager(group, &armredisenterprise.ClientListByResourceGroupOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		clusters, err := c.listByClusters(ctx, page.Value)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		allClusters = append(allClusters, clusters...)
	}
	return allClusters, nil
}

func (c *redisEnterpriseClient) listByClusters(ctx context.Context, clusters []*armredisenterprise.Cluster) ([]*RedisEnterpriseCluster, error) {
	allClusters := make([]*RedisEnterpriseCluster, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster == nil { // should never happen, but checking just in case.
			continue
		}

		// If listByCluster fails for any reason, make a log and continue to
		// other clusters.
		databases, err := c.listByCluster(ctx, cluster)
		if err != nil {
			if trace.IsAccessDenied(err) || trace.IsNotFound(err) {
				logrus.Debugf("Failed to listByCluster on Redis Enterprise cluster %v: %v.", stringVal(cluster.Name), err.Error())
			} else {
				logrus.Warnf("Failed to listByCluster on Redis Enterprise cluster %v: %v.", stringVal(cluster.Name), err.Error())
			}
			continue
		}

		allClusters = append(allClusters, &RedisEnterpriseCluster{
			Cluster:   cluster,
			Databases: databases,
		})
	}
	return allClusters, nil
}

func (c *redisEnterpriseClient) listByCluster(ctx context.Context, cluster *armredisenterprise.Cluster) ([]*armredisenterprise.Database, error) {
	resourceID, err := arm.ParseResourceID(stringVal(cluster.ID))
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	var databases []*armredisenterprise.Database
	pager := c.databaseAPI.NewListByClusterPager(resourceID.ResourceGroupName, stringVal(cluster.Name), &armredisenterprise.DatabasesClientListByClusterOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		databases = append(databases, page.Value...)
	}
	return databases, nil
}

// TODO
type RedisEnterpriseCluster struct {
	*armredisenterprise.Cluster

	// TODO
	Databases []*armredisenterprise.Database
}

const (
	// RedisEnterpriseClusterDefaultDatabase is the default database name for a
	// Redis Enterprise cluster.
	RedisEnterpriseClusterDefaultDatabase = "default"
	// TODO
	RedisEnterpriseClusterPolicyOSS = string(armredisenterprise.ClusteringPolicyOSSCluster)
)
