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

package autoupdate

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// Service implements the gRPC API layer for the Autoupdate.
type Service struct {
	// Opting out of forward compatibility, this service must implement all service methods.
	autoupdate.UnsafeAutoupdateServiceServer

	storage    services.AutoupdateService
	authorizer authz.Authorizer
}

// NewService returns a new Autoupdate API service using the given storage layer and authorizer.
func NewService(storage services.AutoupdateService, authorizer authz.Authorizer) *Service {
	return &Service{
		storage:    storage,
		authorizer: authorizer,
	}
}

// GetAutoupdateConfig gets the current autoupdate config singleton.
func (s *Service) GetAutoupdateConfig(ctx context.Context, req *autoupdate.GetAutoupdateConfigRequest) (*autoupdate.AutoupdateConfig, error) {
	config, err := s.storage.GetAutoupdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// CreateAutoupdateConfig creates autoupdate config singleton.
func (s *Service) CreateAutoupdateConfig(ctx context.Context, req *autoupdate.CreateAutoupdateConfigRequest) (*autoupdate.AutoupdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateConfig, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.storage.CreateAutoupdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// UpdateAutoupdateConfig updates autoupdate config singleton.
func (s *Service) UpdateAutoupdateConfig(ctx context.Context, req *autoupdate.UpdateAutoupdateConfigRequest) (*autoupdate.AutoupdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.storage.UpdateAutoupdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// UpsertAutoupdateConfig updates or creates autoupdate config singleton.
func (s *Service) UpsertAutoupdateConfig(ctx context.Context, req *autoupdate.UpsertAutoupdateConfigRequest) (*autoupdate.AutoupdateConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.storage.UpsertAutoupdateConfig(ctx, req.Config)
	return config, trace.Wrap(err)
}

// DeleteAutoupdateConfig deletes autoupdate config singleton.
func (s *Service) DeleteAutoupdateConfig(ctx context.Context, req *autoupdate.DeleteAutoupdateConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteAutoupdateConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetAutoupdateVersion gets the current autoupdate version singleton.
func (s *Service) GetAutoupdateVersion(ctx context.Context, req *autoupdate.GetAutoupdateVersionRequest) (*autoupdate.AutoupdateVersion, error) {
	version, err := s.storage.GetAutoupdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// CreateAutoupdateVersion creates autoupdate version singleton.
func (s *Service) CreateAutoupdateVersion(ctx context.Context, req *autoupdate.CreateAutoupdateVersionRequest) (*autoupdate.AutoupdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateVersion, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoupdateVersion, err := s.storage.CreateAutoupdateVersion(ctx, req.Version)
	return autoupdateVersion, trace.Wrap(err)
}

// UpdateAutoupdateVersion updates autoupdate version singleton.
func (s *Service) UpdateAutoupdateVersion(ctx context.Context, req *autoupdate.UpdateAutoupdateVersionRequest) (*autoupdate.AutoupdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateVersion, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoupdateVersion, err := s.storage.UpdateAutoupdateVersion(ctx, req.Version)
	return autoupdateVersion, trace.Wrap(err)
}

// UpsertAutoupdateVersion updates or creates autoupdate version singleton.
func (s *Service) UpsertAutoupdateVersion(ctx context.Context, req *autoupdate.UpsertAutoupdateVersionRequest) (*autoupdate.AutoupdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateVersion, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoupdateVersion, err := s.storage.UpsertAutoupdateVersion(ctx, req.Version)
	return autoupdateVersion, trace.Wrap(err)
}

// DeleteAutoupdateVersion deletes autoupdate version singleton.
func (s *Service) DeleteAutoupdateVersion(ctx context.Context, req *autoupdate.DeleteAutoupdateVersionRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoupdateVersion, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteAutoupdateVersion(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
