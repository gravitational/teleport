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
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type ClientMap[ClientType any] struct {
	mu        sync.RWMutex
	clients   map[string]ClientType
	newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (ClientType, error)
}

func newClientMap[ClientType any](newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (ClientType, error)) ClientMap[ClientType] {
	return ClientMap[ClientType]{
		clients:   make(map[string]ClientType),
		newClient: newClient,
	}
}

func (m *ClientMap[ClientType]) Get(subscription string, getCredential func() (azcore.TokenCredential, error)) (client ClientType, err error) {
	m.mu.RLock()
	if client, ok := m.clients[subscription]; ok {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// If some other thread already got here first.
	if client, ok := m.clients[subscription]; ok {
		return client, nil
	}

	cred, err := getCredential()
	if err != nil {
		return client, trace.Wrap(err)
	}

	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	client, err = m.newClient(subscription, cred, options)
	if err != nil {
		return client, trace.Wrap(err)
	}

	m.clients[subscription] = client
	return client, nil
}

func NewRedisClientMap() ClientMap[CacheForRedisClient] {
	return newClientMap(func(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CacheForRedisClient, error) {
		logrus.Debug("Initializing Azure Redis client.")
		api, err := armredis.NewClient(subscription, cred, options)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewRedisClient(api), nil
	})
}

func NewRedisEnterpriseClientMap() ClientMap[CacheForRedisClient] {
	return newClientMap(func(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CacheForRedisClient, error) {
		logrus.Debug("Initializing Azure Redis Enterprise client.")
		databaseAPI, err := armredisenterprise.NewDatabasesClient(subscription, cred, options)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// TODO(greedy52) Redis Enterprise requires a 2nd client (armredisenterprise.Client) for auto-discovery.
		return NewRedisEnterpriseClient(databaseAPI), nil
	})
}
