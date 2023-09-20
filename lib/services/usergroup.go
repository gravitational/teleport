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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// UserGroups defines an interface for managing UserGroups.
type UserGroups interface {
	// ListUserGroups returns a paginated list of all user group resources.
	ListUserGroups(context.Context, int, string) ([]types.UserGroup, string, error)
	// GetUserGroup returns the specified user group resources.
	GetUserGroup(ctx context.Context, name string) (types.UserGroup, error)
	// CreateUserGroup creates a new user group resource.
	CreateUserGroup(context.Context, types.UserGroup) error
	// UpdateUserGroup updates an existing user group resource.
	UpdateUserGroup(context.Context, types.UserGroup) error
	// DeleteUserGroup removes the specified user group resource.
	DeleteUserGroup(ctx context.Context, name string) error
	// DeleteAllUserGroups removes all user groups.
	DeleteAllUserGroups(context.Context) error
}

// MarshalUserGroup marshals the user group resource to JSON.
func MarshalUserGroup(group types.UserGroup, opts ...MarshalOption) ([]byte, error) {
	if err := group.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch g := group.(type) {
	case *types.UserGroupV1:
		if !cfg.PreserveResourceID {
			copy := *g
			copy.SetResourceID(0)
			g = &copy
		}
		return utils.FastMarshal(g)
	default:
		return nil, trace.BadParameter("unsupported user group resource %T", g)
	}
}

// UnmarshalUserGroup unmarshals user group resource from JSON.
func UnmarshalUserGroup(data []byte, opts ...MarshalOption) (types.UserGroup, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing group data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var g types.UserGroupV1
		if err := utils.FastUnmarshal(data, &g); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := g.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			g.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			g.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			g.SetExpiry(cfg.Expires)
		}
		return &g, nil
	}
	return nil, trace.BadParameter("unsupported user group resource version %q", h.Version)
}
