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

package autoupdatev1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// Cache defines only read-only service methods.
type Cache interface {
	// GetAutoUpdateConfig gets the AutoUpdateConfig from the backend.
	GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the AutoUpdateVersion from the backend.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)
}

// ServiceConfig holds configuration options for the auto update gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Backend is the backend used to store AutoUpdate resources.
	Backend services.AutoUpdateService
	// Cache is the cache used to store AutoUpdate resources.
	Cache Cache
}

// Service implements the gRPC API layer for the AutoUpdate.
type Service struct {
	autoupdate.UnimplementedAutoUpdateServiceServer

	authorizer authz.Authorizer
	backend    services.AutoUpdateService
	cache      Cache
}

// NewService returns a new AutoUpdate API service using the given storage layer and authorizer.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
	}, nil
}

// GetAutoUpdateConfig gets the current AutoUpdateConfig singleton.
func (s *Service) GetAutoUpdateConfig(ctx context.Context, req *autoupdate.GetAutoUpdateConfigRequest) (*autoupdate.AutoUpdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.cache.GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// CreateAutoUpdateConfig creates AutoUpdateConfig singleton.
func (s *Service) CreateAutoUpdateConfig(ctx context.Context, req *autoupdate.CreateAutoUpdateConfigRequest) (*autoupdate.AutoUpdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateConfig, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.CreateAutoUpdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// UpdateAutoUpdateConfig updates AutoUpdateConfig singleton.
func (s *Service) UpdateAutoUpdateConfig(ctx context.Context, req *autoupdate.UpdateAutoUpdateConfigRequest) (*autoupdate.AutoUpdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.UpdateAutoUpdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// UpsertAutoUpdateConfig updates or creates AutoUpdateConfig singleton.
func (s *Service) UpsertAutoUpdateConfig(ctx context.Context, req *autoupdate.UpsertAutoUpdateConfigRequest) (*autoupdate.AutoUpdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.UpsertAutoUpdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// DeleteAutoUpdateConfig deletes AutoUpdateConfig singleton.
func (s *Service) DeleteAutoUpdateConfig(ctx context.Context, req *autoupdate.DeleteAutoUpdateConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAutoUpdateConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetAutoUpdateVersion gets the current AutoUpdateVersion singleton.
func (s *Service) GetAutoUpdateVersion(ctx context.Context, req *autoupdate.GetAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	version, err := s.cache.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// CreateAutoUpdateVersion creates AutoUpdateVersion singleton.
func (s *Service) CreateAutoUpdateVersion(ctx context.Context, req *autoupdate.CreateAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateVersion, err := s.backend.CreateAutoUpdateVersion(ctx, req.Version)
	return autoUpdateVersion, trace.Wrap(err)
}

// UpdateAutoUpdateVersion updates AutoUpdateVersion singleton.
func (s *Service) UpdateAutoUpdateVersion(ctx context.Context, req *autoupdate.UpdateAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateVersion, err := s.backend.UpdateAutoUpdateVersion(ctx, req.Version)
	return autoUpdateVersion, trace.Wrap(err)
}

// UpsertAutoUpdateVersion updates or creates AutoUpdateVersion singleton.
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

	autoUpdateVersion, err := s.backend.UpsertAutoUpdateVersion(ctx, req.Version)
	return autoUpdateVersion, trace.Wrap(err)
}

// DeleteAutoUpdateVersion deletes AutoUpdateVersion singleton.
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

	if err := s.backend.DeleteAutoUpdateVersion(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
