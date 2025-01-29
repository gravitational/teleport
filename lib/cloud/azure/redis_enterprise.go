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

package azure

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
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

// ListAll returns all Azure Redis Enterprise databases within an Azure subscription.
func (c *redisEnterpriseClient) ListAll(ctx context.Context) ([]*RedisEnterpriseDatabase, error) {
	var allDatabases []*RedisEnterpriseDatabase
	pager := c.clusterAPI.NewListPager(&armredisenterprise.ClientListOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		allDatabases = append(allDatabases, c.listDatabasesByClusters(ctx, page.Value)...)
	}
	return allDatabases, nil
}

// ListWithinGroup returns all Azure Redis Enterprise databases within an Azure resource group.
func (c *redisEnterpriseClient) ListWithinGroup(ctx context.Context, group string) ([]*RedisEnterpriseDatabase, error) {
	var allDatabases []*RedisEnterpriseDatabase
	pager := c.clusterAPI.NewListByResourceGroupPager(group, &armredisenterprise.ClientListByResourceGroupOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		allDatabases = append(allDatabases, c.listDatabasesByClusters(ctx, page.Value)...)
	}
	return allDatabases, nil
}

// listDatabasesByClusters fetches databases for the provided clusters.
func (c *redisEnterpriseClient) listDatabasesByClusters(ctx context.Context, clusters []*armredisenterprise.Cluster) []*RedisEnterpriseDatabase {
	var allDatabases []*RedisEnterpriseDatabase
	for _, cluster := range clusters {
		if cluster == nil { // should never happen, but checking just in case.
			continue
		}

		// If listDatabasesByCluster fails for any reason, make a log and continue to
		// other clusters.
		databases, err := c.listDatabasesByCluster(ctx, cluster)
		if err != nil {
			if trace.IsAccessDenied(err) || trace.IsNotFound(err) {
				slog.DebugContext(ctx, "Failed to listDatabasesByCluster on Redis Enterprise cluster",
					"cluster", StringVal(cluster.Name),
					"error", err)
			} else {
				slog.WarnContext(ctx, "Failed to listDatabasesByCluster on Redis Enterprise cluster",
					"cluster", StringVal(cluster.Name),
					"error", err,
				)
			}
			continue
		}

		allDatabases = append(allDatabases, databases...)
	}
	return allDatabases
}

// listDatabasesByCluster fetches databases for the provided cluster.
func (c *redisEnterpriseClient) listDatabasesByCluster(ctx context.Context, cluster *armredisenterprise.Cluster) ([]*RedisEnterpriseDatabase, error) {
	resourceID, err := arm.ParseResourceID(StringVal(cluster.ID))
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	var databases []*RedisEnterpriseDatabase
	pager := c.databaseAPI.NewListByClusterPager(resourceID.ResourceGroupName, StringVal(cluster.Name), &armredisenterprise.DatabasesClientListByClusterOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		for _, database := range page.Value {
			databases = append(databases, &RedisEnterpriseDatabase{
				Database: database,
				Cluster:  cluster,
			})
		}
	}
	return databases, nil
}

// RedisEnterpriseDatabase is a wrapper of a armredisenterprise.Database and
// its parent cluster.
type RedisEnterpriseDatabase struct {
	*armredisenterprise.Database

	// Cluster is the parent cluster.
	Cluster *armredisenterprise.Cluster
}

// String returns the description of the database.
func (d *RedisEnterpriseDatabase) String() string {
	if StringVal(d.Name) == RedisEnterpriseClusterDefaultDatabase {
		return fmt.Sprintf("cluster %v", StringVal(d.Cluster.Name))
	}
	return fmt.Sprintf("cluster %v (database %v)", StringVal(d.Cluster.Name), StringVal(d.Database.Name))
}

const (
	// RedisEnterpriseClusterDefaultDatabase is the default database name for a
	// Redis Enterprise cluster.
	RedisEnterpriseClusterDefaultDatabase = "default"
	// RedisEnterpriseClusterPolicyOSS indicates the Redis Enterprise cluster
	// is running in OSS mode.
	RedisEnterpriseClusterPolicyOSS = string(armredisenterprise.ClusteringPolicyOSSCluster)
)
