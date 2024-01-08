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
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// GroupMaxPageSize is the max page size of the group.
const GroupMaxPageSize = 200

// UserGroupService manages user groups in the Backend.
type UserGroupService struct {
	svc generic.Service[types.UserGroup]
}

// NewUserGroupService creates a new UserGroupService.
func NewUserGroupService(backend backend.Backend) (*UserGroupService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.UserGroup]{
		Backend:       backend,
		PageLimit:     GroupMaxPageSize,
		ResourceKind:  types.KindUserGroup,
		BackendPrefix: userGroupPrefix,
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
