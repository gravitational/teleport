// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package generic

import (
	"context"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// MarshalFunc is a type signature for a marshaling function.
type MarshalFunc[T any] func(T, ...services.MarshalOption) ([]byte, error)

// UnmarshalFunc is a type signature for an unmarshalling function.
type UnmarshalFunc[T any] func([]byte, ...services.MarshalOption) (T, error)

// ServiceCommon is a common generic service interface.
type ServiceCommon[T any] interface {
	// WithPrefix will return a service with the given parts appended to the backend prefix.
	WithPrefix(parts ...string) ServiceCommon[T]
	// GetResources returns a list of all resources.
	GetResources(ctx context.Context) ([]T, error)
	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error)
	// GetResource returns the specified resource.
	GetResource(ctx context.Context, name string) (resource T, err error)
	// CreateResource creates a new resource.
	CreateResource(ctx context.Context, resource T) (T, error)
	// UpdateResource updates an existing resource.
	UpdateResource(ctx context.Context, resource T) (T, error)
	// UpsertResource upserts a resource.
	UpsertResource(ctx context.Context, resource T) (T, error)
	// DeleteResource removes the specified resource.
	DeleteResource(ctx context.Context, name string) error
	// DeleteAllResources removes all resources.
	DeleteAllResources(ctx context.Context) error
	// UpdateAndSwapResource will get the resource from the backend, modify it, and swap the new value into the backend.
	UpdateAndSwapResource(ctx context.Context, name string, modify func(T) error) (T, error)
	// MakeBackendItem will check and make the backend item.
	MakeBackendItem(resource T, name string) (backend.Item, error)
	// RunWhileLocked will run the given function in a backend lock. This is a wrapper around the backend.RunWhileLocked function.
	RunWhileLocked(ctx context.Context, lockName string, ttl time.Duration, fn func(context.Context, backend.Backend) error) error
}
