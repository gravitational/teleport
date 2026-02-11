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

package vnetconfigv1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for VNet service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Emitter is used to send audit events in response to processing requests.
	Emitter apievents.Emitter
	// Storage is used to store VNet configs.
	Storage services.VnetConfigService
	// Logger is the slog logger.
	Logger *slog.Logger
}

// Service implements the gRPC API layer for the singleton VnetConfig resource.
type Service struct {
	// Opting out of forward compatibility, this service must implement all service methods.
	vnet.UnsafeVnetConfigServiceServer

	storage    services.VnetConfigService
	authorizer authz.Authorizer
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewService returns a new VnetConfig API service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required for vnet config service")
	}
	if cfg.Storage == nil {
		return nil, trace.BadParameter("storage is required for vnet config service")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required for vnet config service")
	}
	return &Service{
		authorizer: cfg.Authorizer,
		storage:    cfg.Storage,
		emitter:    cfg.Emitter,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// GetVnetConfig returns the singleton VnetConfig resource.
func (s *Service) GetVnetConfig(ctx context.Context, _ *vnet.GetVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.GetVnetConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return vnetConfig, nil
}

// CreateVnetConfig creates a VnetConfig resource.
func (s *Service) CreateVnetConfig(ctx context.Context, req *vnet.CreateVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.CreateVnetConfig(ctx, req.VnetConfig)
	status := eventStatus(err)
	s.emitAuditEvent(ctx, newCreateAuditEvent(ctx, status))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return vnetConfig, nil
}

// UpdateVnetConfig updates a VnetConfig resource.
func (s *Service) UpdateVnetConfig(ctx context.Context, req *vnet.UpdateVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.UpdateVnetConfig(ctx, req.VnetConfig)
	status := eventStatus(err)
	s.emitAuditEvent(ctx, newUpdateAuditEvent(ctx, status))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return vnetConfig, nil
}

// UpsertVnetConfig does basic validation and upserts a VnetConfig resource.
func (s *Service) UpsertVnetConfig(ctx context.Context, req *vnet.UpsertVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.UpsertVnetConfig(ctx, req.VnetConfig)
	status := eventStatus(err)
	s.emitAuditEvent(ctx, newCreateAuditEvent(ctx, status))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return vnetConfig, nil
}

// DeleteVnetConfig deletes the singleton VnetConfig resource.
func (s *Service) DeleteVnetConfig(ctx context.Context, _ *vnet.DeleteVnetConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindVnetConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.storage.DeleteVnetConfig(ctx)
	status := eventStatus(err)
	s.emitAuditEvent(ctx, newDeleteAuditEvent(ctx, status))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
