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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
)

// Inventory is a subset of Presence dedicated to tracking the status of all
// teleport instances independent of any specific service.
//
// NOTE: the instance resource scales linearly with cluster size and is not cached in a traditional
// manner. as such, it is should not be accessed as part of the "hot path" of any normal request.
type Inventory interface {
	// GetInstances iterates the full teleport server inventory.
	GetInstances(ctx context.Context, req types.InstanceFilter) stream.Stream[types.Instance]
}

// InventoryInternal is a subset of the PresenceInternal interface that extends
// inventory functionality with auth-specific internal methods.
type InventoryInternal interface {
	Inventory

	// UpsertInstance creates or updates an instance resource.
	UpsertInstance(ctx context.Context, instance types.Instance) error
}
