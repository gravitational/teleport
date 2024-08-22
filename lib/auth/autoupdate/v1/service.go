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

package autoupd

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// Service implements the gRPC API layer for the AutoUpdate.
type Service struct {
	// Opting out of forward compatibility, this service must implement all service methods.
	autoupdate.UnsafeAutoUpdateServiceServer

	storage    services.AutoUpdateService
	authorizer authz.Authorizer
}

// NewService returns a new AutoUpdate API service using the given storage layer and authorizer.
func NewService(storage services.AutoUpdateService, authorizer authz.Authorizer) *Service {
	return &Service{
		storage:    storage,
		authorizer: authorizer,
	}
}

// GetClusterAutoUpdateConfig gets the current autoupdate config singleton.
func (s *Service) GetClusterAutoUpdateConfig(ctx context.Context, req *autoupdate.GetClusterAutoUpdateConfigRequest) (*autoupdate.ClusterAutoUpdateConfig, error) {
	config, err := s.storage.GetClusterAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// UpsertClusterAutoUpdateConfig updates or creates autoupdate config singleton.
func (s *Service) UpsertClusterAutoUpdateConfig(ctx context.Context, req *autoupdate.UpsertClusterAutoUpdateConfigRequest) (*autoupdate.ClusterAutoUpdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindClusterAutoUpdateConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.storage.UpsertClusterAutoUpdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// DeleteClusterAutoUpdateConfig deletes autoupdate config singleton.
func (s *Service) DeleteClusterAutoUpdateConfig(ctx context.Context, req *autoupdate.DeleteClusterAutoUpdateConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindClusterAutoUpdateConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteClusterAutoUpdateConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetAutoUpdateVersion gets the current autoupdate version singleton.
func (s *Service) GetAutoUpdateVersion(ctx context.Context, req *autoupdate.GetAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.storage.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// UpsertAutoUpdateVersion updates or creates autoupdate version singleton.
func (s *Service) UpsertAutoUpdateVersion(ctx context.Context, req *autoupdate.UpsertAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateVersion, err := s.storage.UpsertAutoUpdateVersion(ctx, req.Version)
	return autoUpdateVersion, trace.Wrap(err)
}

// DeleteAutoUpdateVersion deletes autoupdate version singleton.
func (s *Service) DeleteAutoUpdateVersion(ctx context.Context, req *autoupdate.DeleteAutoUpdateVersionRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteAutoUpdateVersion(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
