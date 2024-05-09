/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package vnetconfig

import (
	"context"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type VnetConfigService struct {
	// Opting out of forward compatibility, this service must implement all service methods.
	vnet.UnsafeVnetConfigServiceServer

	storage    *local.VnetConfigService
	authorizer authz.Authorizer
}

func NewVnetConfigService(storage *local.VnetConfigService, authorizer authz.Authorizer) *VnetConfigService {
	return &VnetConfigService{
		storage:    storage,
		authorizer: authorizer,
	}
}

func (s *VnetConfigService) GetVnetConfig(ctx context.Context, _ *vnet.GetVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.GetVnetConfig(ctx)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	if err := checkAccessToResource(authCtx, vnetConfig, types.VerbRead); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	return vnetConfig, nil
}

func (s *VnetConfigService) CreateVnetConfig(ctx context.Context, req *vnet.CreateVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAccessToResource(authCtx, req.VnetConfig, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.CreateVnetConfig(ctx, req.VnetConfig)
	return vnetConfig, trace.Wrap(err)
}

func (s *VnetConfigService) UpdateVnetConfig(ctx context.Context, req *vnet.UpdateVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAccessToResource(authCtx, req.VnetConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	oldVnetConfig, err := s.storage.GetVnetConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAccessToResource(authCtx, oldVnetConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.UpdateVnetConfig(ctx, req.VnetConfig)
	return vnetConfig, trace.Wrap(err)
}

func (s *VnetConfigService) UpsertVnetConfig(ctx context.Context, req *vnet.UpsertVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// To upsert you must be allowed to Create and Update the new resource.
	if err := checkAccessToResource(authCtx, req.VnetConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Try a few times to Create or Update the resource, in case we race with another Create or Update.
	for i := 0; i < 5; i++ {
		// To upsert you must be allowed to Update the existing resource, if there is one.
		existingVnetConfig, err := s.storage.GetVnetConfig(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if err == nil {
			// There is an existing resource, make sure the user is allowed to Update it.
			if err := checkAccessToResource(authCtx, existingVnetConfig, types.VerbUpdate); err != nil {
				return nil, trace.Wrap(err)
			}

			// Make sure the resource revision doesn't change between the authz check and the write.
			newVnetConfig := proto.Clone(req.VnetConfig).(*vnet.VnetConfig)
			newVnetConfig.Metadata.Revision = existingVnetConfig.Metadata.Revision
			vnetConfig, err := s.storage.UpdateVnetConfig(ctx, newVnetConfig)
			if trace.IsCompareFailed(err) {
				continue
			}
			return vnetConfig, trace.Wrap(err)
		}

		// There is no existing resource, just Create the new one.
		vnetConfig, err := s.storage.CreateVnetConfig(ctx, req.VnetConfig)
		if trace.IsAlreadyExists(err) {
			continue
		}
		return vnetConfig, trace.Wrap(err)
	}

	return nil, trace.CompareFailed("failed to upsert vnet_config within 5 attempts")
}

func (s *VnetConfigService) DeleteVnetConfig(ctx context.Context, _ *vnet.DeleteVnetConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	existingVnetConfig, err := s.storage.GetVnetConfig(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			// Nothing to delete
			return &emptypb.Empty{}, nil
		}
		return nil, trace.Wrap(err)
	}

	if err := checkAccessToResource(authCtx, existingVnetConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteVnetConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func checkAccessToResource(authCtx *authz.Context, vnetConfig *vnet.VnetConfig, verb string, additionalVerbs ...string) error {
	return trace.Wrap(authCtx.CheckAccessToResource(types.Resource153ToLegacy(vnetConfig), verb, additionalVerbs...))
}
