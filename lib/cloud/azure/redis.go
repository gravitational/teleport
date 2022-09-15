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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/gravitational/trace"
)

type armRedisClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, name string, options *armredis.ClientListKeysOptions) (armredis.ClientListKeysResponse, error)
}

// TODO
type RedisClient struct {
	api armRedisClient
}

// NewRedisClient creates a new Azure Redis client.
func NewRedisClient(api armRedisClient) *RedisClient {
	return &RedisClient{
		api: api,
	}
}

// GetToken implements CacheForRedisClient.
func (c *RedisClient) GetToken(ctx context.Context, group, name string) (string, error) {
	resp, err := c.api.ListKeys(ctx, group, name, &armredis.ClientListKeysOptions{})
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
