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
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// armRedisClient is an interface defines a subset of functions of armredis.Client.
type armRedisClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, name string, options *armredis.ClientListKeysOptions) (armredis.ClientListKeysResponse, error)
	NewListByResourceGroupPager(resourceGroupName string, options *armredis.ClientListByResourceGroupOptions) *runtime.Pager[armredis.ClientListByResourceGroupResponse]
	NewListBySubscriptionPager(options *armredis.ClientListBySubscriptionOptions) *runtime.Pager[armredis.ClientListBySubscriptionResponse]
}

// redisClient is an Azure Redis client.
type redisClient struct {
	api armRedisClient
}

// NewRedisClient creates a new Azure Redis client by subscription and credentials.
func NewRedisClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CacheForRedisClient, error) {
	logrus.Debug("Initializing Azure Redis client.")
	api, err := armredis.NewClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewRedisClientByAPI(api), nil
}

// NewRedisClientByAPI creates a new Azure Redis client by ARM API client.
func NewRedisClientByAPI(api armRedisClient) CacheForRedisClient {
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
func (c *redisClient) ListAll(ctx context.Context) ([]RedisServer, error) {
	var servers []RedisServer
	pager := c.api.NewListBySubscriptionPager(&armredis.ClientListBySubscriptionOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, redisListResultToRedisServers(page.Value)...)
	}
	return servers, nil
}

// ListWithinGroup returns all Azure Redis servers within an Azure resource group.
func (c *redisClient) ListWithinGroup(ctx context.Context, group string) ([]RedisServer, error) {
	var servers []RedisServer
	pager := c.api.NewListByResourceGroupPager(group, &armredis.ClientListByResourceGroupOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, redisListResultToRedisServers(page.Value)...)
	}
	return servers, nil
}

func redisListResultToRedisServers(resources []*armredis.ResourceInfo) []RedisServer {
	var servers []RedisServer
	for _, resourceInfo := range resources {
		if resourceInfo == nil { // should never happen, but checking just in case.
			logrus.Debugf("Resource info is nil.")
			continue
		}
		if resourceInfo.Properties == nil { // should never happen, but checking just in case.
			logrus.Debugf("Missing properties for armredis.ResourceInfo %v.", stringVal(resourceInfo.Name))
			continue
		}
		servers = append(servers, &redisServer{
			ResourceInfo: resourceInfo,
		})
	}
	return servers
}

type redisServer struct {
	*armredis.ResourceInfo
}

func (s *redisServer) GetName() string {
	return stringVal(s.Name)
}
func (s *redisServer) GetLocation() string {
	return stringVal(s.Location)
}
func (s *redisServer) GetResourceID() string {
	return stringVal(s.ID)
}
func (s *redisServer) GetHostname() string {
	return stringVal(s.Properties.HostName)
}
func (s *redisServer) GetPort() string {
	if s.Properties.SSLPort != nil {
		return strconv.Itoa(int(*s.Properties.SSLPort))
	}
	return ""
}
func (s *redisServer) GetTags() map[string]string {
	return convertTags(s.Tags)
}
func (s *redisServer) GetClusteringPolicy() string {
	// Does not apply to Azure Redis (non-Enterprise).
	return ""
}
func (s *redisServer) GetEngineVersion() string {
	return stringVal(s.Properties.RedisVersion)
}
func (s *redisServer) IsSupported() (bool, error) {
	if s.Properties.SSLPort == nil { // should never happen, but checking just in case.
		return false, trace.NotFound("missing SSL port")
	}
	return true, nil
}
func (s *redisServer) IsAvailable() (bool, error) {
	switch armredis.ProvisioningState(stringVal(s.Properties.ProvisioningState)) {
	case armredis.ProvisioningStateSucceeded,
		armredis.ProvisioningStateLinking,
		armredis.ProvisioningStateRecoveringScaleFailure,
		armredis.ProvisioningStateScaling,
		armredis.ProvisioningStateUnlinking,
		armredis.ProvisioningStateUpdating:
		return true, nil
	case armredis.ProvisioningStateCreating,
		armredis.ProvisioningStateDeleting,
		armredis.ProvisioningStateDisabled,
		armredis.ProvisioningStateFailed,
		armredis.ProvisioningStateProvisioning,
		armredis.ProvisioningStateUnprovisioning:
		return false, trace.BadParameter("the current provisioning state is %v", stringVal(s.Properties.ProvisioningState))
	default:
		logrus.Warnf("Unknown status type: %q. Assuming Azure Redis %q is available.",
			stringVal(s.Properties.ProvisioningState),
			s.GetName(),
		)
		return true, nil
	}
}
