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

	"github.com/gravitational/teleport/api/types"
)

// ServersClient provides an interface for getting MySQL servers.
type ServersClient interface {
	// ListServers lists all Azure MySQL servers within an Azure subscription by resource group.
	// If the resource group is "*", then all resources are queried.
	ListServers(ctx context.Context, group string, maxPages int) ([]Server, error)
	// TODO(gavin)
	Kind() string
	// TODO(gavin)
	Subscription() string
}

// TODO(gavin)
type SubscriptionsClient interface {
	// TODO(gavin)
	ListSubscriptions(ctx context.Context, maxPages int, useCache bool) ([]string, error)
}

// TODO(gavin)
type Server interface {
	// TODO(gavin)
	GetName() string
	// TODO(gavin)
	GetRegion() string
	// TODO(gavin)
	GetVersion() string
	// TODO(gavin)
	GetEndpoint() string
	// TODO(gavin)
	GetProtocol() string
	// TODO(gavin)
	GetState() string
	// TODO(gavin)
	GetID() types.AzureResourceID
	// TODO(gavin)
	GetTags() map[string]string
	// TODO(gavin)
	IsVersionSupported() bool
	// TODO(gavin)
	IsAvailable() bool
}
