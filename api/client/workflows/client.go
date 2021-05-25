/*
Copyright 2021 Gravitational, Inc.

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

package workflows

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// Client is the client interface needed by the workflows package.
// Client is implemented by the api/client package.
type Client interface {
	// CreateAccessRequest registers a new access request with the auth server.
	CreateAccessRequest(ctx context.Context, req types.AccessRequest) error
	// GetAccessRequests retrieves a list of all access requests matching the provided filter.
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	// SetAccessRequestState updates the state of an existing access request.
	SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error
	// GetPluginData loads all plugin data matching the supplied filter.
	GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error)
	// UpdatePluginData updates a per-resource PluginData entry.
	UpdatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error
	// NewWatcher returns a new streamWatcher
	NewWatcher(ctx context.Context, filter types.Watch) (types.Watcher, error)
}
