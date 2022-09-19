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
	"strings"

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

// NewRedisClient creates a new Azure Redis Enterprise client.
func NewRedisEnterpriseClient(databaseAPI armRedisEnterpriseDatabaseClient) CacheForRedisClient {
	return &redisEnterpriseClient{
		databaseAPI: databaseAPI,
	}
}

// NewRedisClientMap creates a new map of Redis Enterprise clients.
func NewRedisEnterpriseClientMap() ClientMap[CacheForRedisClient] {
	return newClientMap(func(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CacheForRedisClient, error) {
		logrus.Debug("Initializing Azure Redis Enterprise client.")
		databaseAPI, err := armredisenterprise.NewDatabasesClient(subscription, cred, options)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// TODO(greedy52) Redis Enterprise requires a different API client
		// (armredisenterprise.Client) for auto-discovery.
		return NewRedisEnterpriseClient(databaseAPI), nil
	})
}

// GetToken implements CacheForRedisClient.
func (c *redisEnterpriseClient) GetToken(ctx context.Context, group, name string) (string, error) {
	clusterName, databaseName := c.getClusterAndDatabaseName(name)
	resp, err := c.databaseAPI.ListKeys(ctx, group, clusterName, databaseName, &armredisenterprise.DatabasesClientListKeysOptions{})
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

func (c *redisEnterpriseClient) getClusterAndDatabaseName(resourceName string) (string, string) {
	// The resource name can be either:
	//   - cluster resource name: <clusterName>
	//   - database resource name: <clusterName>/databases/<databaseName>
	//
	// Though it appears an Enterprise cluster always has only one "database",
	// and the database name is always "default".
	clusterName, databaseName, ok := strings.Cut(resourceName, "/databases/")
	if ok {
		return clusterName, databaseName
	}
	return clusterName, "default"
}
