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

package healthcheckconfigv1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds creation parameters for [Service].
type ServiceConfig struct {
	// Authorizer used by the service.
	Authorizer authz.Authorizer
	// Backend is the backend service.
	Backend services.HealthCheckConfig
	// Cache is the cache used to store health check config resources.
	Cache services.HealthCheckConfigReader
	// Emitter is an audit event emitter.
	Emitter apievents.Emitter
	// Logger is the slog logger.
	Logger *slog.Logger
}

// Service implements the teleport.healthcheck.v1.HealthCheckConfigService gRPC
// API.
type Service struct {
	healthcheckconfigv1.UnimplementedHealthCheckConfigServiceServer

	authorizer authz.Authorizer
	backend    services.HealthCheckConfig
	cache      services.HealthCheckConfigReader
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewService creates a new [Service] instance.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required for health check config service")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required for health check config service")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("cache is required for health check config service")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required for health check config service")
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// CreateHealthCheckConfig creates a new HealthCheckConfig.
func (s *Service) CreateHealthCheckConfig(ctx context.Context, req *healthcheckconfigv1.CreateHealthCheckConfigRequest) (*healthcheckconfigv1.HealthCheckConfig, error) {
	if err := s.authorize(ctx, true, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateHealthCheckConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newCreateAuditEvent(ctx, created))
	return created, nil
}

// GetHealthCheckConfig returns the specified HealthCheckConfig.
func (s *Service) GetHealthCheckConfig(ctx context.Context, req *healthcheckconfigv1.GetHealthCheckConfigRequest) (*healthcheckconfigv1.HealthCheckConfig, error) {
	if err := s.authorize(ctx, false, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := s.cache.GetHealthCheckConfig(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg, nil
}

// ListHealthCheckConfigs lists HealthCheckConfig resources.
func (s *Service) ListHealthCheckConfigs(ctx context.Context, req *healthcheckconfigv1.ListHealthCheckConfigsRequest) (*healthcheckconfigv1.ListHealthCheckConfigsResponse, error) {
	if err := s.authorize(ctx, false, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	page, token, err := s.cache.ListHealthCheckConfigs(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &healthcheckconfigv1.ListHealthCheckConfigsResponse{
		Configs:       page,
		NextPageToken: token,
	}, nil
}

// UpdateHealthCheckConfig updates an existing HealthCheckConfig.
func (s *Service) UpdateHealthCheckConfig(ctx context.Context, req *healthcheckconfigv1.UpdateHealthCheckConfigRequest) (*healthcheckconfigv1.HealthCheckConfig, error) {
	if err := s.authorize(ctx, true, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpdateHealthCheckConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newUpdateAuditEvent(ctx, updated))
	return updated, nil
}

// UpsertHealthCheckConfig creates or replaces a HealthCheckConfig.
func (s *Service) UpsertHealthCheckConfig(ctx context.Context, req *healthcheckconfigv1.UpsertHealthCheckConfigRequest) (*healthcheckconfigv1.HealthCheckConfig, error) {
	if err := s.authorize(ctx, true, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.backend.UpsertHealthCheckConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newCreateAuditEvent(ctx, upserted))
	return upserted, nil
}

// DeleteHealthCheckConfig deletes the specified HealthCheckConfig.
func (s *Service) DeleteHealthCheckConfig(ctx context.Context, req *healthcheckconfigv1.DeleteHealthCheckConfigRequest) (*emptypb.Empty, error) {
	if err := s.authorize(ctx, true, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteHealthCheckConfig(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, newDeleteAuditEvent(ctx, req.GetName()))
	return &emptypb.Empty{}, nil
}

func (s *Service) authorize(ctx context.Context, adminAction bool, verb string, extraVerbs ...string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindHealthCheckConfig, verb, extraVerbs...); err != nil {
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
