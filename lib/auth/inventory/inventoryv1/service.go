// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package inventoryv1

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cache"
)

type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// InventoryCache is the inventory cache containing instances.
	InventoryCache *cache.InventoryCache
}

func (c *ServiceConfig) CheckAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer")
	}
	if c.InventoryCache == nil {
		return trace.BadParameter("missing InventoryCache")
	}
	return nil
}

// Service implements the teleport.inventory.v1.InventoryService RPC service.
type Service struct {
	inventoryv1.UnimplementedInventoryServiceServer

	authorizer     authz.Authorizer
	inventoryCache *cache.InventoryCache
}

// NewService returns a new inventory service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer:     cfg.Authorizer,
		inventoryCache: cfg.InventoryCache,
	}, nil
}

// ListUnifiedInstances returns a page of teleport instances and bot_instances. This API will refuse any requests when the cache is unhealthy or not yet
// fully initialized.
func (s *Service) ListUnifiedInstances(ctx context.Context, req *inventoryv1.ListUnifiedInstancesRequest) (*inventoryv1.ListUnifiedInstancesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Determine which kinds to include based on permissions.
	includeInstances := true
	includeBotInstances := true

	if err := authCtx.CheckAccessToKind(types.KindInstance, types.VerbList, types.VerbRead); err != nil {
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		includeInstances = false
	}

	if err := authCtx.CheckAccessToKind(types.KindBotInstance, types.VerbList, types.VerbRead); err != nil {
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		includeBotInstances = false
	}

	// If the user doesn't have access to either, return an error.
	if !includeInstances && !includeBotInstances {
		return nil, trace.AccessDenied("user does not have permission to list instances or bot instances")
	}

	// Ensure that the kinds requested in the filter (if any) align with the user's permissions.
	// This is a last line of defense because the client should already prevent the user from requesting
	// kinds they don't have access to (eg. the UI should grey out those options).
	if req.Filter != nil && len(req.Filter.Kinds) > 0 {

		// Deduplicate
		slices.Sort(req.Filter.Kinds)
		req.Filter.Kinds = slices.Compact(req.Filter.Kinds)

		var allowedKinds []string
		for _, kind := range req.Filter.Kinds {
			if (kind == types.KindInstance) && includeInstances {
				allowedKinds = append(allowedKinds, kind)
			} else if (kind == types.KindBotInstance) && includeBotInstances {
				allowedKinds = append(allowedKinds, kind)
			}
		}
		if len(allowedKinds) == 0 {
			return nil, trace.AccessDenied("user doesn't have permission to list the requested instance kinds")
		}
		req.Filter.Kinds = allowedKinds
	} else {
		if req.Filter == nil {
			req.Filter = &inventoryv1.ListUnifiedInstancesFilter{}
		}
		var kinds []string
		if includeInstances {
			kinds = append(kinds, types.KindInstance)
		}
		if includeBotInstances {
			kinds = append(kinds, types.KindBotInstance)
		}
		req.Filter.Kinds = kinds
	}

	resp, err := s.inventoryCache.ListUnifiedInstances(ctx, req)
	return resp, trace.Wrap(err)
}
