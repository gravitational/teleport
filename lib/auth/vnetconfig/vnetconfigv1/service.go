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
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for VNet service.
type ServiceConfig struct {
	// ScopedAuthorizer is the scoped authorizer used to check access to resources.
	ScopedAuthorizer authz.ScopedAuthorizer
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

	storage          services.VnetConfigService
	scopedAuthorizer authz.ScopedAuthorizer
	emitter          apievents.Emitter
	logger           *slog.Logger
}

// NewService returns a new VnetConfig API service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.ScopedAuthorizer == nil {
		return nil, trace.BadParameter("scoped authorizer is required for vnet config service")
	}
	if cfg.Storage == nil {
		return nil, trace.BadParameter("storage is required for vnet config service")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required for vnet config service")
	}
	return &Service{
		scopedAuthorizer: cfg.ScopedAuthorizer,
		storage:          cfg.Storage,
		emitter:          cfg.Emitter,
		logger:           cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// GetVnetConfig returns the singleton VnetConfig resource.
func (s *Service) GetVnetConfig(ctx context.Context, _ *vnet.GetVnetConfigRequest) (*vnet.VnetConfig, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.RiskyAuthorizeUnpinnedRead(ctx, services.UnpinnedReadVnetConfig, &ruleCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	vnetConfig, err := s.storage.GetVnetConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	typesvnet.SetDefaultsVnetConfig(vnetConfig)
	return vnetConfig, nil
}

// CreateVnetConfig creates a VnetConfig resource.
func (s *Service) CreateVnetConfig(ctx context.Context, req *vnet.CreateVnetConfigRequest) (*vnet.VnetConfig, error) {
	if err := s.authorizeWrite(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetVnetConfig() == nil {
		req.VnetConfig = &vnet.VnetConfig{}
	}
	typesvnet.SetDefaultsVnetConfig(req.VnetConfig)
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
	if err := s.authorizeWrite(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetVnetConfig() == nil {
		req.VnetConfig = &vnet.VnetConfig{}
	}
	typesvnet.SetDefaultsVnetConfig(req.VnetConfig)
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
	if err := s.authorizeWrite(ctx, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetVnetConfig() == nil {
		req.VnetConfig = &vnet.VnetConfig{}
	}
	typesvnet.SetDefaultsVnetConfig(req.VnetConfig)
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
	if err := s.authorizeWrite(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err := s.storage.DeleteVnetConfig(ctx)
	status := eventStatus(err)
	s.emitAuditEvent(ctx, newDeleteAuditEvent(ctx, status))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) authorizeWrite(ctx context.Context, verbs ...string) error {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// VnetConfig is unscoped, which is effectively equivalent to the root scope for authz checks.
	const resourceScope = scopes.Root

	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.Decision(ctx, resourceScope, func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, types.KindVnetConfig, verbs...)
	}); err != nil {
		return trace.Wrap(err)
	}

	if unscopedAuthCtx, isUnscoped := authCtx.UnscopedContext(); isUnscoped {
		return unscopedAuthCtx.AuthorizeAdminActionAllowReusedMFA()
	}
	return trace.AccessDenied("cannot perform admin action as scoped identity")
}
