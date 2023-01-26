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

// GroupGetter defines interface for fetching groups.
type GroupGetter interface {
	// ListGroups returns a paginated list of all group resources.
	ListGroups(context.Context, int, string) ([]types.Group, string, error)
	// GetGroup returns the specified group resources.
	GetGroup(ctx context.Context, name string) (types.Group, error)
}

// Groups defines an interface for managing Groups.
type Groups interface {
	// GroupGetter provides methods for fetching groups.
	GroupGetter
	// CreateGroup creates a new group resource.
	CreateGroup(context.Context, types.Group) error
	// UpdateGroup updates an existing group resource.
	UpdateGroup(context.Context, types.Group) error
	// DeleteGroup removes the specified group resource.
	DeleteGroup(ctx context.Context, name string) error
	// DeleteAllGroups removes all groups.
	DeleteAllGroups(context.Context) error
}

// MarshalGroup marshals the group resource to JSON.
func MarshalGroup(group types.Group, opts ...MarshalOption) ([]byte, error) {
	if err := group.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch g := group.(type) {
	case *types.GroupV1:
		if !cfg.PreserveResourceID {
			copy := *g
			copy.SetResourceID(0)
			g = &copy
		}
		return utils.FastMarshal(g)
	default:
		return nil, trace.BadParameter("unsupported group resource %T", g)
	}
}

// UnmarshalGroup unmarshals group resource from JSON.
func UnmarshalGroup(data []byte, opts ...MarshalOption) (types.Group, error) {
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
		var g types.GroupV1
		if err := utils.FastUnmarshal(data, &g); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := g.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			g.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			g.SetExpiry(cfg.Expires)
		}
		return &g, nil
	}
	return nil, trace.BadParameter("unsupported group resource version %q", h.Version)
}
