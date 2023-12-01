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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/utils"
)

const clientExpireTime = time.Hour

// ClientMap is a generic map that caches a collection of Azure clients by
// subscriptions.
type ClientMap[ClientType any] struct {
	clients   *utils.FnCache
	newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (ClientType, error)
}

// ClientMapOptions defines options for creating a client map.
type ClientMapOptions struct {
	clock clockwork.Clock
}

// ClientMapOption allows setting options as functional arguments to NewClientMap.
type ClientMapOption func(*ClientMapOptions)

func withClock(clock clockwork.Clock) ClientMapOption {
	return func(opts *ClientMapOptions) {
		opts.clock = clock
	}
}

// NewClientMap creates a new ClientMap.
func NewClientMap[ClientType any](
	newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (ClientType, error),
	opts ...ClientMapOption,
) (ClientMap[ClientType], error) {
	options := &ClientMapOptions{}
	for _, opt := range opts {
		opt(options)
	}

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   clientExpireTime,
		Clock: options.clock,
	})
	if err != nil {
		return ClientMap[ClientType]{}, trace.Wrap(err)
	}
	return ClientMap[ClientType]{
		clients:   cache,
		newClient: newClient,
	}, nil
}

// Get returns an Azure client by subscription. A new client is created if the
// subscription is not found in the map.
func (m *ClientMap[ClientType]) Get(subscription string, getCredentials func() (azcore.TokenCredential, error)) (ClientType, error) {
	client, err := utils.FnCacheGet[ClientType](context.Background(), m.clients, subscription, func(ctx context.Context) (client ClientType, err error) {
		cred, err := getCredentials()
		if err != nil {
			return client, trace.Wrap(err)
		}

		// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
		options := &arm.ClientOptions{}
		client, err = m.newClient(subscription, cred, options)
		return client, trace.Wrap(err)
	})
	return client, trace.Wrap(err)
}
