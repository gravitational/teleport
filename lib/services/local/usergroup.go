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

// UserGroupService manages user groups in the Backend.
type UserGroupService struct {
	backend.Backend
}

// NewUserGroupService creates a new UserGroupService.
func NewUserGroupService(backend backend.Backend) *UserGroupService {
	return &UserGroupService{Backend: backend}
}

// ListUserGroups returns a paginated list of user group resources.
func (g *UserGroupService) ListUserGroups(ctx context.Context, pageSize int, pageToken string) ([]types.UserGroup, string, error) {
	rangeStart := backend.Key(userGroupPrefix, pageToken)
	rangeEnd := backend.RangeEnd(rangeStart)

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > groupMaxPageSize {
		pageSize = groupMaxPageSize
	}

	// Increment pageSize to allow for the extra item represented by nextKey.
	// We skip this item in the results below.
	limit := pageSize + 1
	var out []types.UserGroup

	// no filter provided get the range directly
	result, err := g.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out = make([]types.UserGroup, 0, len(result.Items))
	for _, item := range result.Items {
		group, err := services.UnmarshalUserGroup(item.Value)
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

// GetUserGroup returns the specified user group resource.
func (g *UserGroupService) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	item, err := g.Get(ctx, backend.Key(userGroupPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("group %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	group, err := services.UnmarshalUserGroup(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return group, nil
}

// CreateUserGroup creates a new user group resource.
func (g *UserGroupService) CreateUserGroup(ctx context.Context, group types.UserGroup) error {
	if err := group.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalUserGroup(group)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(userGroupPrefix, group.GetName()),
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

// UpdateUserGroup updates an existing user group resource.
func (g *UserGroupService) UpdateUserGroup(ctx context.Context, group types.UserGroup) error {
	if err := group.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalUserGroup(group)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(userGroupPrefix, group.GetName()),
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

// DeleteUserGroup removes the specified user group resource.
func (s *UserGroupService) DeleteUserGroup(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(userGroupPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("group %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllUserGroups removes all user group resources.
func (s *UserGroupService) DeleteAllUserGroups(ctx context.Context) error {
	startKey := backend.Key(userGroupPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	userGroupPrefix = "user_group"
)
