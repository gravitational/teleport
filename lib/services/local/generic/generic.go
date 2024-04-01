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

package generic

import (
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// Resource represents a Teleport resource that may be generically
// persisted into the backend.
type Resource interface {
	GetName() string
}

// MarshalFunc is a type signature for a marshaling function, which converts from T to []byte, while respecting specified options.
type MarshalFunc[T any] func(T, ...services.MarshalOption) ([]byte, error)

// UnmarshalFunc is a type signature for an unmarshalling function, which converts from []byte to T, while respecting specified options.
type UnmarshalFunc[T any] func([]byte, ...services.MarshalOption) (T, error)

// ServiceConfig is the configuration for the service configuration.
type ServiceConfig[T Resource] struct {
	Backend       backend.Backend
	ResourceKind  string
	PageLimit     uint
	BackendPrefix string
	MarshalFunc   MarshalFunc[T]
	UnmarshalFunc UnmarshalFunc[T]
	// RunWhileLockedRetryInterval is the interval to retry the RunWhileLocked function.
	// If set to 0, the default interval of 250ms will be used.
	// WARNING: If set to a negative value, the RunWhileLocked function will retry immediately.
	RunWhileLockedRetryInterval time.Duration
}

func (c *ServiceConfig[T]) CheckAndSetDefaults() error {
	if c.Backend == nil {
		return trace.BadParameter("backend is missing")
	}
	if c.ResourceKind == "" {
		return trace.BadParameter("resource kind is missing")
	}
	// We should allow page limit to be 0 for services that don't use pagination. Some services are
	// intended to be internally facing only, and those services may not need to set this limit.
	if c.PageLimit == 0 {
		c.PageLimit = defaults.DefaultChunkSize
	}
	if c.BackendPrefix == "" {
		return trace.BadParameter("backend prefix is missing")
	}
	if c.MarshalFunc == nil {
		return trace.BadParameter("marshal func is missing")
	}
	if c.UnmarshalFunc == nil {
		return trace.BadParameter("unmarshal func is missing")
	}

	return nil
}

// Service is a generic service for interacting with resources in the backend.
type Service[T Resource] struct {
	backend                     backend.Backend
	resourceKind                string
	pageLimit                   uint
	backendPrefix               string
	marshalFunc                 MarshalFunc[T]
	unmarshalFunc               UnmarshalFunc[T]
	runWhileLockedRetryInterval time.Duration
}

// NewService will return a new generic service with the given config. This will
// panic if the configuration is invalid.
func NewService[T Resource](cfg *ServiceConfig[T]) (*Service[T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service[T]{
		backend:                     cfg.Backend,
		resourceKind:                cfg.ResourceKind,
		pageLimit:                   cfg.PageLimit,
		backendPrefix:               cfg.BackendPrefix,
		marshalFunc:                 cfg.MarshalFunc,
		unmarshalFunc:               cfg.UnmarshalFunc,
		runWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	}, nil
}

// WithPrefix will return a service with the given parts appended to the backend prefix.
func (s *Service[T]) WithPrefix(parts ...string) *Service[T] {
	if len(parts) == 0 {
		return s
	}

	return &Service[T]{
		backend:       s.backend,
		resourceKind:  s.resourceKind,
		pageLimit:     s.pageLimit,
		backendPrefix: strings.Join(append([]string{s.backendPrefix}, parts...), string(backend.Separator)),
		marshalFunc:   s.marshalFunc,
		unmarshalFunc: s.unmarshalFunc,
	}
}

// CountResources will return a count of all resources in the prefix range.
func (s *Service[T]) CountResources(ctx context.Context) (uint, error) {
	rangeStart := backend.ExactKey(s.backendPrefix)
	rangeEnd := backend.RangeEnd(rangeStart)

	count := uint(0)
	err := backend.IterateRange(ctx, s.backend, rangeStart, rangeEnd, int(s.pageLimit),
		func(items []backend.Item) (stop bool, err error) {
			count += uint(len(items))
			return false, nil
		})

	return count, trace.Wrap(err)
}

// GetResources returns a list of all resources.
func (s *Service[T]) GetResources(ctx context.Context) ([]T, error) {
	rangeStart := backend.ExactKey(s.backendPrefix)
	rangeEnd := backend.RangeEnd(rangeStart)

	// no filter provided get the range directly
	result, err := s.backend.GetRange(ctx, rangeStart, rangeEnd, backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		resource, err := s.unmarshalFunc(item.Value, services.WithRevision(item.Revision), services.WithResourceID(item.ID))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, resource)
	}

	return out, nil
}

// ListResources returns a paginated list of resources.
func (s *Service[T]) ListResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	resources, next, err := s.ListResourcesReturnNextResource(ctx, pageSize, pageToken)
	var nextKey string
	if next != nil {
		nextKey = backend.GetPaginationKey(*next)
	}
	return resources, nextKey, trace.Wrap(err)
}

// ListResourcesReturnNextResource returns a paginated list of resources. The next resource is returned, which allows consumers to construct
// the next pagination key as appropriate.
func (s *Service[T]) ListResourcesReturnNextResource(ctx context.Context, pageSize int, pageToken string) ([]T, *T, error) {
	rangeStart := backend.Key(s.backendPrefix, pageToken)
	rangeEnd := backend.RangeEnd(backend.ExactKey(s.backendPrefix))

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > int(s.pageLimit) {
		pageSize = int(s.pageLimit)
	}

	limit := pageSize + 1

	// no filter provided get the range directly
	result, err := s.backend.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	out := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		resource, err := s.unmarshalFunc(item.Value, services.WithRevision(item.Revision), services.WithResourceID(item.ID))
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		out = append(out, resource)
	}

	var next *T
	if len(out) > pageSize {
		next = &out[pageSize]
		// Truncate the last item that was used to determine next row existence.
		out = out[:pageSize]
	}

	return out, next, nil
}

// GetResource returns the specified resource.
func (s *Service[T]) GetResource(ctx context.Context, name string) (resource T, err error) {
	item, err := s.backend.Get(ctx, s.MakeKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return resource, trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return resource, trace.Wrap(err)
	}
	resource, err = s.unmarshalFunc(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	return resource, trace.Wrap(err)
}

// CreateResource creates a new resource.
func (s *Service[T]) CreateResource(ctx context.Context, resource T) (T, error) {
	var t T
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return t, trace.Wrap(err)
	}

	lease, err := s.backend.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return t, trace.AlreadyExists("%s %q already exists", s.resourceKind, resource.GetName())
	}
	if err != nil {
		return t, trace.Wrap(err)
	}

	types.SetRevision(resource, lease.Revision)
	return resource, trace.Wrap(err)
}

// UpdateResource updates an existing resource.
func (s *Service[T]) UpdateResource(ctx context.Context, resource T) (T, error) {
	var t T
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return t, trace.Wrap(err)
	}

	lease, err := s.backend.Update(ctx, item)
	if trace.IsNotFound(err) {
		return t, trace.NotFound("%s %q doesn't exist", s.resourceKind, resource.GetName())
	}
	if err != nil {
		return t, trace.Wrap(err)
	}

	types.SetRevision(resource, lease.Revision)
	return resource, trace.Wrap(err)
}

// ConditionalUpdateResource updates an existing resource if revision matches.
func (s *Service[T]) ConditionalUpdateResource(ctx context.Context, resource T) (T, error) {
	var t T
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return t, trace.Wrap(err)
	}

	lease, err := s.backend.ConditionalUpdate(ctx, item)
	if trace.IsNotFound(err) {
		return t, trace.NotFound("%s %q doesn't exist", s.resourceKind, resource.GetName())
	}
	if err != nil {
		return t, trace.Wrap(err)
	}

	types.SetRevision(resource, lease.Revision)
	return resource, trace.Wrap(err)
}

// UpsertResource upserts a resource.
func (s *Service[T]) UpsertResource(ctx context.Context, resource T) (T, error) {
	var t T
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return t, trace.Wrap(err)
	}

	lease, err := s.backend.Put(ctx, item)
	if err != nil {
		return t, trace.Wrap(err)
	}

	types.SetRevision(resource, lease.Revision)
	return resource, nil
}

// DeleteResource removes the specified resource.
func (s *Service[T]) DeleteResource(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, s.MakeKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllResources removes all resources.
func (s *Service[T]) DeleteAllResources(ctx context.Context) error {
	startKey := backend.ExactKey(s.backendPrefix)
	return trace.Wrap(s.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// UpdateAndSwapResource will get the resource from the backend, modify it, and swap the new value into the backend.
func (s *Service[T]) UpdateAndSwapResource(ctx context.Context, name string, modify func(T) error) (T, error) {
	var t T
	existingItem, err := s.backend.Get(ctx, s.MakeKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return t, trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return t, trace.Wrap(err)
	}

	resource, err := s.unmarshalFunc(existingItem.Value,
		services.WithResourceID(existingItem.ID), services.WithExpires(existingItem.Expires), services.WithRevision(existingItem.Revision))
	if err != nil {
		return t, trace.Wrap(err)
	}

	err = modify(resource)
	if err != nil {
		return t, trace.Wrap(err)
	}

	replacementItem, err := s.MakeBackendItem(resource, name)
	if err != nil {
		return t, trace.Wrap(err)
	}

	lease, err := s.backend.CompareAndSwap(ctx, *existingItem, replacementItem)
	if err != nil {
		return t, trace.Wrap(err)
	}

	types.SetRevision(resource, lease.Revision)
	return resource, trace.Wrap(err)
}

// MakeBackendItem will check and make the backend item.
func (s *Service[T]) MakeBackendItem(resource T, name string) (backend.Item, error) {
	if err := services.CheckAndSetDefaults(resource); err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	rev, err := types.GetRevision(resource)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	value, err := s.marshalFunc(resource)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      s.MakeKey(name),
		Value:    value,
		Revision: rev,
	}

	item.Expires, err = types.GetExpiry(resource)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	//nolint:staticcheck // SA1019. Added for backward compatibility.
	item.ID, err = types.GetResourceID(resource)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return item, nil
}

// MakeKey will make a key for the service given a name.
func (s *Service[T]) MakeKey(name string) []byte {
	return backend.Key(s.backendPrefix, name)
}

// RunWhileLocked will run the given function in a backend lock. This is a wrapper around the backend.RunWhileLocked function.
func (s *Service[T]) RunWhileLocked(ctx context.Context, lockName string, ttl time.Duration, fn func(context.Context, backend.Backend) error) error {
	return trace.Wrap(backend.RunWhileLocked(ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:       s.backend,
				LockName:      lockName,
				TTL:           ttl,
				RetryInterval: s.runWhileLockedRetryInterval,
			},
		}, func(ctx context.Context) error {
			return fn(ctx, s.backend)
		}))
}
