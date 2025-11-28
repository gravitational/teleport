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

package appauthconfigv1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for [Service].
type ServiceConfig struct {
	// Authorizer used by the service.
	Authorizer authz.Authorizer
	// Backend is the backend service.
	Backend services.AppAuthConfig
	// Cache is the cache used to store health check config resources.
	Cache services.AppAuthConfigReader
	// Emitter is an audit event emitter.
	Emitter apievents.Emitter
	// Logger is the slog logger.
	Logger *slog.Logger
}

// Service implements the teleport.appauthconfig.v1.AppAuthConfigServer gRPC
// API.
type Service struct {
	appauthconfigv1.UnimplementedAppAuthConfigServiceServer

	authorizer authz.Authorizer
	backend    services.AppAuthConfig
	cache      services.AppAuthConfigReader
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewService creates a new instance of [Service].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required for app auth config service")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required for app auth config service")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("cache is required for app auth config service")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required for app auth config service")
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// CreateAppAuthConfig implements appauthconfigv1.AppAuthConfigServiceServer.
func (s *Service) CreateAppAuthConfig(ctx context.Context, req *appauthconfigv1.CreateAppAuthConfigRequest) (*appauthconfigv1.AppAuthConfig, error) {
	if err := s.authorize(ctx, true, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateAppAuthConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newCreateAuditEvent(ctx, created))
	return created, nil
}

// DeleteAppAuthConfig implements appauthconfigv1.AppAuthConfigServiceServer.
func (s *Service) DeleteAppAuthConfig(ctx context.Context, req *appauthconfigv1.DeleteAppAuthConfigRequest) (*emptypb.Empty, error) {
	if err := s.authorize(ctx, true, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAppAuthConfig(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newDeleteAuditEvent(ctx, req.GetName()))
	return &emptypb.Empty{}, nil
}

// GetAppAuthConfig implements appauthconfigv1.AppAuthConfigServiceServer.
func (s *Service) GetAppAuthConfig(ctx context.Context, req *appauthconfigv1.GetAppAuthConfigRequest) (*appauthconfigv1.AppAuthConfig, error) {
	if err := s.authorize(ctx, false, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := s.cache.GetAppAuthConfig(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg, nil
}

// ListAppAuthConfigs implements appauthconfigv1.AppAuthConfigServiceServer.
func (s *Service) ListAppAuthConfigs(ctx context.Context, req *appauthconfigv1.ListAppAuthConfigsRequest) (*appauthconfigv1.ListAppAuthConfigsResponse, error) {
	if err := s.authorize(ctx, false, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	page, token, err := s.cache.ListAppAuthConfigs(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &appauthconfigv1.ListAppAuthConfigsResponse{
		Configs:       page,
		NextPageToken: token,
	}, nil
}

// UpdateAppAuthConfig implements appauthconfigv1.AppAuthConfigServiceServer.
func (s *Service) UpdateAppAuthConfig(ctx context.Context, req *appauthconfigv1.UpdateAppAuthConfigRequest) (*appauthconfigv1.AppAuthConfig, error) {
	if err := s.authorize(ctx, true, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpdateAppAuthConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newUpdateAuditEvent(ctx, updated))
	return updated, nil
}

// UpsertAppAuthConfig implements appauthconfigv1.AppAuthConfigServiceServer.
func (s *Service) UpsertAppAuthConfig(ctx context.Context, req *appauthconfigv1.UpsertAppAuthConfigRequest) (*appauthconfigv1.AppAuthConfig, error) {
	if err := s.authorize(ctx, true, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.backend.UpsertAppAuthConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newCreateAuditEvent(ctx, upserted))
	return upserted, nil
}

func (s *Service) authorize(ctx context.Context, adminAction bool, verb string, extraVerbs ...string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindAppAuthConfig, verb, extraVerbs...); err != nil {
		return trace.Wrap(err)
	}
	if adminAction {
		if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *Service) emitAuditEvent(ctx context.Context, evt apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(ctx, "Failed to emit audit event",
			"event_code", evt.GetCode(),
			"event_type", evt.GetType(),
			"error", err,
		)
	}
}
