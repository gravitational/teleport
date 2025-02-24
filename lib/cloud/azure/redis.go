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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/gravitational/trace"
)

// armRedisClient is an interface defines a subset of functions of armredis.Client.
type armRedisClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, name string, options *armredis.ClientListKeysOptions) (armredis.ClientListKeysResponse, error)
	NewListBySubscriptionPager(options *armredis.ClientListBySubscriptionOptions) *runtime.Pager[armredis.ClientListBySubscriptionResponse]
	NewListByResourceGroupPager(resourceGroupName string, options *armredis.ClientListByResourceGroupOptions) *runtime.Pager[armredis.ClientListByResourceGroupResponse]
}

// redisClient is an Azure Redis client.
type redisClient struct {
	api armRedisClient
}

// NewRedisClient creates a new Azure Redis client by subscription and credentials.
func NewRedisClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (RedisClient, error) {
	api, err := armredis.NewClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewRedisClientByAPI(api), nil
}

// NewRedisClientByAPI creates a new Azure Redis client by ARM API client.
func NewRedisClientByAPI(api armRedisClient) RedisClient {
	return &redisClient{
		api: api,
	}
}

// GetToken retrieves the auth token for provided resource group and resource
// name.
func (c *redisClient) GetToken(ctx context.Context, resourceID string) (string, error) {
	id, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err := c.api.ListKeys(ctx, id.ResourceGroupName, id.Name, &armredis.ClientListKeysOptions{})
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

// ListAll returns all Azure Redis servers within an Azure subscription.
func (c *redisClient) ListAll(ctx context.Context) ([]*armredis.ResourceInfo, error) {
	var servers []*armredis.ResourceInfo
	pager := c.api.NewListBySubscriptionPager(&armredis.ClientListBySubscriptionOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}

// ListWithinGroup returns all Azure Redis servers within an Azure resource group.
func (c *redisClient) ListWithinGroup(ctx context.Context, group string) ([]*armredis.ResourceInfo, error) {
	var servers []*armredis.ResourceInfo
	pager := c.api.NewListByResourceGroupPager(group, &armredis.ClientListByResourceGroupOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}
