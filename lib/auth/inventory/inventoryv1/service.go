/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package inventoryv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	inventorypb "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// InventoryCache is the subset of the inventory cache that the Service uses.
type InventoryCache interface {
	ListUnifiedInstances(ctx context.Context, req *inventorypb.ListUnifiedInstancesRequest) (*inventorypb.ListUnifiedInstancesResponse, error)
}

// ServiceConfig holds configuration options for the inventory gRPC service.
type ServiceConfig struct {
	Authorizer     authz.Authorizer
	InventoryCache InventoryCache
	Logger         *slog.Logger
}

// Service implements the teleport.inventory.v1.InventoryService RPC service.
type Service struct {
	inventorypb.UnimplementedInventoryServiceServer

	authorizer     authz.Authorizer
	inventoryCache InventoryCache
	logger         *slog.Logger
}

// NewService returns a new inventory gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.InventoryCache == nil:
		return nil, trace.BadParameter("inventory cache is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "inventory.service")
	}

	return &Service{
		logger:         cfg.Logger,
		authorizer:     cfg.Authorizer,
		inventoryCache: cfg.InventoryCache,
	}, nil
}

// ListUnifiedInstances returns a page of teleport instances and bot_instances.
// This API will refuse any requests when the cache is unhealthy or not yet fully initialized.
func (s *Service) ListUnifiedInstances(
	ctx context.Context, req *inventorypb.ListUnifiedInstancesRequest,
) (*inventorypb.ListUnifiedInstancesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If no instance types are specified, default to all instance types
	instanceTypes := req.GetFilter().GetInstanceTypes()
	if len(instanceTypes) == 0 {
		instanceTypes = []inventorypb.InstanceType{
			inventorypb.InstanceType_INSTANCE_TYPE_INSTANCE,
			inventorypb.InstanceType_INSTANCE_TYPE_BOT_INSTANCE,
		}
	}

	// Ensure that the instance types requested align with the user's permissions
	for _, instanceType := range instanceTypes {
		switch instanceType {
		case inventorypb.InstanceType_INSTANCE_TYPE_INSTANCE:
			if err := authCtx.CheckAccessToKind(types.KindInstance, types.VerbList, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case inventorypb.InstanceType_INSTANCE_TYPE_BOT_INSTANCE:
			if err := authCtx.CheckAccessToKind(types.KindBotInstance, types.VerbList, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			return nil, trace.NotImplemented("instance type %v is not supported", instanceType)
		}
	}

	resp, err := s.inventoryCache.ListUnifiedInstances(ctx, req)
	return resp, trace.Wrap(err)
}
