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
