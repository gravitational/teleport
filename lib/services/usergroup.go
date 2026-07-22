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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch g := group.(type) {
	case *types.UserGroupV1:
		if err := g.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, g))
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
			return nil, trace.BadParameter("%s", err)
		}
		if err := g.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
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
