/*
Copyright 2023 Gravitational, Inc.

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
	// Backend used to persist the resource.
	Backend backend.Backend
	// ResourceKind is the friendly name of the resource.
	ResourceKind string
	// PageLimit
	PageLimit uint
	// BackendPrefix used when constructing the [backend.Item.Key].
	BackendPrefix string
	// MarshlFunc converts the resource to bytes for persistence.
	MarshalFunc MarshalFunc[T]
	// UnmarshalFunc converts the bytes read from the backend to the resource.
	UnmarshalFunc UnmarshalFunc[T]
	// ValidateFunc optionally validates the resource prior to persisting it. Any errors
	// returned from the validation function will prevent writes to the backend.
	ValidateFunc func(T) error
	// RunWhileLockedRetryInterval is the interval to retry the RunWhileLocked function.
	// If set to 0, the default interval of 250ms will be used.
	// WARNING: If set to a negative value, the RunWhileLocked function will retry immediately.
	RunWhileLockedRetryInterval time.Duration
	// KeyFunc optionally allows resource to have a custom key. If not provided the
	// name of the resource will be used.
	KeyFunc func(T) string
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

	if c.ValidateFunc == nil {
		c.ValidateFunc = func(t T) error { return nil }
	}

	if c.KeyFunc == nil {
		c.KeyFunc = func(t T) string { return t.GetName() }
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
	validateFunc                func(T) error
	runWhileLockedRetryInterval time.Duration
	keyFunc                     func(T) string
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
		validateFunc:                cfg.ValidateFunc,
		runWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
		keyFunc:                     cfg.KeyFunc,
	}, nil
}

// WithPrefix will return a service with the given parts appended to the backend prefix.
func (s *Service[T]) WithPrefix(parts ...string) *Service[T] {
	if len(parts) == 0 {
		return s
	}

	return &Service[T]{
		backend:                     s.backend,
		resourceKind:                s.resourceKind,
		pageLimit:                   s.pageLimit,
		backendPrefix:               strings.Join(append([]string{s.backendPrefix}, parts...), string(backend.Separator)),
		marshalFunc:                 s.marshalFunc,
		unmarshalFunc:               s.unmarshalFunc,
		validateFunc:                s.validateFunc,
		runWhileLockedRetryInterval: s.runWhileLockedRetryInterval,
		keyFunc:                     s.keyFunc,
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
	rangeStart := backend.NewKey(s.backendPrefix, pageToken)
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
	if err := s.validateFunc(resource); err != nil {
		return t, trace.Wrap(err)
	}

	item, err := s.MakeBackendItem(resource, s.keyFunc(resource))
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

	if err := s.validateFunc(resource); err != nil {
		return t, trace.Wrap(err)
	}

	item, err := s.MakeBackendItem(resource, s.keyFunc(resource))
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

// UpsertResource upserts a resource.
func (s *Service[T]) UpsertResource(ctx context.Context, resource T) (T, error) {
	var t T

	if err := s.validateFunc(resource); err != nil {
		return t, trace.Wrap(err)
	}

	item, err := s.MakeBackendItem(resource, s.keyFunc(resource))
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

	if err := modify(resource); err != nil {
		return t, trace.Wrap(err)
	}

	if err := s.validateFunc(resource); err != nil {
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
func (s *Service[T]) MakeKey(name string) backend.Key {
	return backend.NewKey(s.backendPrefix, name)
}

// RunWhileLocked will run the given function in a backend lock. This is a wrapper around the backend.RunWhileLocked function.
func (s *Service[T]) RunWhileLocked(ctx context.Context, lockNameComponents []string, ttl time.Duration, fn func(context.Context, backend.Backend) error) error {
	return trace.Wrap(backend.RunWhileLocked(ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            s.backend,
				LockNameComponents: lockNameComponents,
				TTL:                ttl,
				RetryInterval:      s.runWhileLockedRetryInterval,
			},
		}, func(ctx context.Context) error {
			return fn(ctx, s.backend)
		}))
}
