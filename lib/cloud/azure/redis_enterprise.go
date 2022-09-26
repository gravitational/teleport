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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// armRedisEnterpriseDatabaseClient is an interface defines a subset of
// functions of armredisenterprise.DatabaseClient
type armRedisEnterpriseDatabaseClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, clusterName string, databaseName string, options *armredisenterprise.DatabasesClientListKeysOptions) (armredisenterprise.DatabasesClientListKeysResponse, error)
}

// redisEnterpriseClient is an Azure Redis Enterprise client.
type redisEnterpriseClient struct {
	databaseAPI armRedisEnterpriseDatabaseClient
}

// NewRedisEnterpriseClient creates a new Azure Redis Enterprise client by
// subscription and credentials.
func NewRedisEnterpriseClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CacheForRedisClient, error) {
	logrus.Debug("Initializing Azure Redis Enterprise client.")
	databaseAPI, err := armredisenterprise.NewDatabasesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(greedy52) Redis Enterprise requires a different API client
	// (armredisenterprise.Client) for auto-discovery.
	return NewRedisEnterpriseClientByAPI(databaseAPI), nil
}

// NewRedisEnterpriseClientByAPI creates a new Azure Redis Enterprise client by
// ARM API client(s).
func NewRedisEnterpriseClientByAPI(databaseAPI armRedisEnterpriseDatabaseClient) CacheForRedisClient {
	return &redisEnterpriseClient{
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

// RedisEnterpriseClusterDefaultDatabase is the default database name for a
// Redis Enterprise cluster.
const RedisEnterpriseClusterDefaultDatabase = "default"
