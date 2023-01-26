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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const groupMaxPageSize = 200

// GroupService manages groups in the Backend.
type GroupService struct {
	backend.Backend
}

// NewGroupService creates a new GroupService.
func NewGroupService(backend backend.Backend) *GroupService {
	return &GroupService{Backend: backend}
}

// ListGroups returns a paginated list of group resources.
func (g *GroupService) ListGroups(ctx context.Context, pageSize int, pageToken string) ([]types.Group, string, error) {
	rangeStart := backend.Key(groupPrefix, pageToken)
	rangeEnd := backend.RangeEnd(rangeStart)

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > groupMaxPageSize {
		pageSize = groupMaxPageSize
	}

	// Increment pageSize to allow for the extra item represented by nextKey.
	// We skip this item in the results below.
	limit := pageSize + 1
	var out []types.Group

	// no filter provided get the range directly
	result, err := g.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out = make([]types.Group, 0, len(result.Items))
	for _, item := range result.Items {
		group, err := services.UnmarshalGroup(item.Value)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		out = append(out, group)
	}

	var nextKey string
	if len(out) > pageSize {
		nextKey = backend.GetPaginationKey(out[len(out)-1])
		// Truncate the last item that was used to determine next row existence.
		out = out[:pageSize]
	}

	return out, nextKey, nil
}

// GetGroup returns the specified group resource.
func (g *GroupService) GetGroup(ctx context.Context, name string) (types.Group, error) {
	item, err := g.Get(ctx, backend.Key(groupPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("group %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	group, err := services.UnmarshalGroup(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return group, nil
}

// CreateGroup creates a new group resource.
func (g *GroupService) CreateGroup(ctx context.Context, group types.Group) error {
	if err := group.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalGroup(group)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(groupPrefix, group.GetName()),
		Value:   value,
		Expires: group.Expiry(),
		ID:      group.GetResourceID(),
	}
	_, err = g.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateGroup updates an existing group resource.
func (g *GroupService) UpdateGroup(ctx context.Context, group types.Group) error {
	if err := group.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalGroup(group)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(groupPrefix, group.GetName()),
		Value:   value,
		Expires: group.Expiry(),
		ID:      group.GetResourceID(),
	}
	_, err = g.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteGroup removes the specified group resource.
func (s *GroupService) DeleteGroup(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(groupPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("group %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllGroups removes all group resources.
func (s *GroupService) DeleteAllGroups(ctx context.Context) error {
	startKey := backend.Key(groupPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	groupPrefix = "group"
)
