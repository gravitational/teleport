// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

// ListRoles implements ListRoles rpc.
func (s *Handler) ListRoles(ctx context.Context, req *api.ListRolesRequest) (*api.ListRolesResponse, error) {
	cluster, _, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := cluster.GetAllRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var roleList []string

	for _, role := range roles {
		// skip internal roles
		if types.SystemResource == role.GetMetadata().Labels[types.TeleportInternalResourceType] {
			continue
		}
		roleList = append(roleList, role.GetName())
	}

	return &api.ListRolesResponse{
		Roles: roleList,
	}, nil
}

// ListUsers implements ListUsers rpc.
func (s *Handler) ListUsers(ctx context.Context, req *api.ListUsersRequest) (*api.ListUsersResponse, error) {
	cluster, _, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	users, err := cluster.GetAllUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var userList []string

	for _, user := range users {
		// skip internal roles
		if types.SystemResource == user.GetMetadata().Labels[types.TeleportInternalResourceType] {
			continue
		}
		userList = append(userList, user.GetName())
	}

	return &api.ListUsersResponse{
		Users: userList,
	}, nil
}
