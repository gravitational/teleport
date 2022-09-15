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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/gravitational/trace"
)

type armRedisEnterpriseDatabaseClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, clusterName string, databaseName string, options *armredisenterprise.DatabasesClientListKeysOptions) (armredisenterprise.DatabasesClientListKeysResponse, error)
}

// TODO
type RedisEnterpriseClient struct {
	databaseAPI armRedisEnterpriseDatabaseClient
}

// NewRedisClient creates a new Azure Redis Enterprise client.
func NewRedisEnterpriseClient(databaseAPI armRedisEnterpriseDatabaseClient) *RedisEnterpriseClient {
	return &RedisEnterpriseClient{
		databaseAPI: databaseAPI,
	}
}

// GetToken implements CacheForRedisClient.
func (c *RedisEnterpriseClient) GetToken(ctx context.Context, group, name string) (string, error) {
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

func (c *RedisEnterpriseClient) getClusterAndDatabaseName(resourceName string) (string, string) {
	// The resource name can be either:
	//   - cluster resource name: <clusterName>
	//   - database resource name: <clusterName>/databases/<databaseName>
	//
	// Though it seems an Enterprise cluster only has one database, the
	// database name is always "default".
	clusterName, databaseName, ok := strings.Cut(resourceName, "/databases/")
	if ok {
		return clusterName, databaseName
	}
	return clusterName, "default"
}
