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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// GroupMaxPageSize is the max page size of the group.
const GroupMaxPageSize = 200

// UserGroupService manages user groups in the Backend.
type UserGroupService struct {
	svc generic.Service[types.UserGroup]
}

// NewUserGroupService creates a new UserGroupService.
func NewUserGroupService(b backend.Backend) (*UserGroupService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.UserGroup]{
		Backend:       b,
		PageLimit:     GroupMaxPageSize,
		ResourceKind:  types.KindUserGroup,
		BackendPrefix: backend.NewKey(userGroupPrefix),
		MarshalFunc:   services.MarshalUserGroup,
		UnmarshalFunc: services.UnmarshalUserGroup,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UserGroupService{
		svc: *svc,
	}, nil
}

// ListUserGroups returns a paginated list of user group resources.
func (s *UserGroupService) ListUserGroups(ctx context.Context, pageSize int, pageToken string) ([]types.UserGroup, string, error) {
	groups, next, err := s.svc.ListResources(ctx, pageSize, pageToken)
	return groups, next, trace.Wrap(err)
}

// GetUserGroup returns the specified user group resource.
func (s *UserGroupService) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	group, err := s.svc.GetResource(ctx, name)
	return group, trace.Wrap(err)
}

// CreateUserGroup creates a new user group resource.
func (s *UserGroupService) CreateUserGroup(ctx context.Context, group types.UserGroup) error {
	_, err := s.svc.CreateResource(ctx, group)
	return trace.Wrap(err)
}

// UpdateUserGroup updates an existing user group resource.
func (s *UserGroupService) UpdateUserGroup(ctx context.Context, group types.UserGroup) error {
	_, err := s.svc.UpdateResource(ctx, group)
	return trace.Wrap(err)
}

// DeleteUserGroup removes the specified user group resource.
func (s *UserGroupService) DeleteUserGroup(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllUserGroups removes all user group resources.
func (s *UserGroupService) DeleteAllUserGroups(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

const (
	userGroupPrefix = "user_group"
)
