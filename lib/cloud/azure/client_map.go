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
	"github.com/gravitational/trace"
)

// ClientMap is a generic map that caches a collection of Azure clients by
// subscriptions.
type ClientMap[ClientType any] struct {
	mu        sync.RWMutex
	clients   map[string]ClientType
	newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (ClientType, error)
}

// NewClientMap creates a new ClientMap.
func NewClientMap[ClientType any](newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (ClientType, error)) ClientMap[ClientType] {
	return ClientMap[ClientType]{
		clients:   make(map[string]ClientType),
		newClient: newClient,
	}
}

// Get returns an Azure client by subscription. A new client is created if the
// subscription is not found in the map.
func (m *ClientMap[ClientType]) Get(subscription string, getCredentials func() (azcore.TokenCredential, error)) (client ClientType, err error) {
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

	cred, err := getCredentials()
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
