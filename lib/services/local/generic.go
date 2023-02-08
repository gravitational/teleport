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

package local

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// marshalFunc is a type signature for a marshaling function.
type marshalFunc[T types.Resource] func(T, ...services.MarshalOption) ([]byte, error)

// UnmarshalFunc is a type signature for an unmarshaling function.
type unmarshalFunc[T types.Resource] func([]byte, ...services.MarshalOption) (T, error)

// genericResourceService is a generic service for interacting with resources in the backend.
type genericResourceService[T types.Resource] struct {
	backend       backend.Backend
	resourceKind  string
	limit         int
	backendPrefix string
	marshalFunc   marshalFunc[T]
	unmarshalFunc unmarshalFunc[T]
}

// ListResources returns a paginated list of resources.
func (s *genericResourceService[T]) listResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	rangeStart := backend.Key(s.backendPrefix, pageToken)
	rangeEnd := backend.RangeEnd(rangeStart)

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > s.limit {
		pageSize = s.limit
	}

	// If backend.NoLimit is requested and the service permits it, set the limit to NoLimit.
	limit := pageSize + 1

	// no filter provided get the range directly
	result, err := s.backend.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		resource, err := s.unmarshalFunc(item.Value)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		out = append(out, resource)
	}

	var nextKey string
	if len(out) > pageSize {
		nextKey = backend.GetPaginationKey(out[len(out)-1])
		// Truncate the last item that was used to determine next row existence.
		out = out[:pageSize]
	}

	return out, nextKey, nil
}

// getResource returns the specified resource.
func (s *genericResourceService[T]) getResource(ctx context.Context, name string, extraKeyParts ...string) (resource T, err error) {
	key := s.fullKey(name, extraKeyParts...)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return resource, trace.NotFound("%s %q doesn't exist", s.resourceKind, s.displayName(name, extraKeyParts...))
		}
		return resource, trace.Wrap(err)
	}
	resource, err = s.unmarshalFunc(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	return resource, trace.Wrap(err)
}

// createResource creates a new resource.
func (s *genericResourceService[T]) createResource(ctx context.Context, resource T, name string, extraKeyParts ...string) error {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := s.marshalFunc(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     s.fullKey(name, extraKeyParts...),
		Value:   value,
		Expires: resource.Expiry(),
		ID:      resource.GetResourceID(),
	}

	_, err = s.backend.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("%s %q already exists", s.resourceKind, s.displayName(name, extraKeyParts...))
	}

	return trace.Wrap(err)
}

// updateResource updates an existing resource.
func (s *genericResourceService[T]) updateResource(ctx context.Context, resource T, name string, extraKeyParts ...string) error {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := s.marshalFunc(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     s.fullKey(name, extraKeyParts...),
		Value:   value,
		Expires: resource.Expiry(),
		ID:      resource.GetResourceID(),
	}

	_, err = s.backend.Update(ctx, item)
	if trace.IsNotFound(err) {
		return trace.NotFound("%s %q doesn't exist", s.resourceKind, s.displayName(name, extraKeyParts...))
	}

	return trace.Wrap(err)
}

// deleteResource removes the specified resource.
func (s *genericResourceService[T]) deleteResource(ctx context.Context, name string, extraKeyParts ...string) error {
	key := s.fullKey(name, extraKeyParts...)
	err := s.backend.Delete(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceKind, s.displayName(name, extraKeyParts...))
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllResources removes all resources.
func (s *genericResourceService[T]) deleteAllResources(ctx context.Context) error {
	startKey := backend.Key(s.backendPrefix)
	return trace.Wrap(s.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// fullKey calculates a key from a name and extra key parts.
func (s *genericResourceService[T]) fullKey(name string, extraKeyParts ...string) []byte {
	return backend.Key(append([]string{s.backendPrefix, name}, extraKeyParts...)...)
}

// displayName creates a display name for a name and its extra key parts.
func (s *genericResourceService[T]) displayName(name string, extraKeyParts ...string) string {
	displayName := name
	if len(extraKeyParts) != 0 {
		displayName += fmt.Sprintf(" (%s)", strings.Join(extraKeyParts, string(backend.Separator)))
	}

	return displayName
}
