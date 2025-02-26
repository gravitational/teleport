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
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
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
	// Emitter is the event emitter.
	Emitter apievents.Emitter
}

// Service implements the gRPC API layer for the AutoUpdate.
type Service struct {
	autoupdate.UnimplementedAutoUpdateServiceServer

	authorizer authz.Authorizer
	backend    services.AutoUpdateService
	emitter    apievents.Emitter
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
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("Emitter is required")
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
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

	config, err := s.backend.CreateAutoUpdateConfig(ctx, req.Config)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateConfigCreate{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateConfigCreateEvent,
			Code: events.AutoUpdateConfigCreateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateConfig,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
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

	config, err := s.backend.UpdateAutoUpdateConfig(ctx, req.Config)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateConfigUpdateEvent,
			Code: events.AutoUpdateConfigUpdateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateConfig,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
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

	config, err := s.backend.UpsertAutoUpdateConfig(ctx, req.Config)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateConfigUpdateEvent,
			Code: events.AutoUpdateConfigUpdateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateConfig,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
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

	err = s.backend.DeleteAutoUpdateConfig(ctx)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateConfigDelete{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateConfigDeleteEvent,
			Code: events.AutoUpdateConfigDeleteCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateConfig,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
	return &emptypb.Empty{}, trace.Wrap(err)
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

	if err := checkAdminCloudAccess(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateVersion, err := s.backend.CreateAutoUpdateVersion(ctx, req.Version)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateVersionCreate{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateVersionCreateEvent,
			Code: events.AutoUpdateVersionCreateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateVersion,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})

	return autoUpdateVersion, trace.Wrap(err)
}

// UpdateAutoUpdateVersion updates AutoUpdateVersion singleton.
func (s *Service) UpdateAutoUpdateVersion(ctx context.Context, req *autoupdate.UpdateAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAdminCloudAccess(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateVersion, err := s.backend.UpdateAutoUpdateVersion(ctx, req.Version)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateVersionUpdate{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateVersionUpdateEvent,
			Code: events.AutoUpdateVersionUpdateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateVersion,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})

	return autoUpdateVersion, trace.Wrap(err)
}

// UpsertAutoUpdateVersion updates or creates AutoUpdateVersion singleton.
func (s *Service) UpsertAutoUpdateVersion(ctx context.Context, req *autoupdate.UpsertAutoUpdateVersionRequest) (*autoupdate.AutoUpdateVersion, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAdminCloudAccess(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateVersion, err := s.backend.UpsertAutoUpdateVersion(ctx, req.Version)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateVersionUpdate{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateVersionUpdateEvent,
			Code: events.AutoUpdateVersionUpdateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateVersion,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})

	return autoUpdateVersion, trace.Wrap(err)
}

// DeleteAutoUpdateVersion deletes AutoUpdateVersion singleton.
func (s *Service) DeleteAutoUpdateVersion(ctx context.Context, req *autoupdate.DeleteAutoUpdateVersionRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAdminCloudAccess(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateVersion, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteAutoUpdateVersion(ctx)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.AutoUpdateVersionDelete{
		Metadata: apievents.Metadata{
			Type: events.AutoUpdateVersionDeleteEvent,
			Code: events.AutoUpdateVersionDeleteCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameAutoUpdateVersion,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
	return &emptypb.Empty{}, trace.Wrap(err)
}

func (s *Service) emitEvent(ctx context.Context, e apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, e); err != nil {
		slog.WarnContext(ctx, "Failed to emit audit event",
			"type", e.GetType(),
			"error", err,
		)
	}
}

// checkAdminCloudAccess validates if the given context has the builtin admin role if cloud feature is enabled.
func checkAdminCloudAccess(authCtx *authz.Context) error {
	if modules.GetModules().Features().Cloud && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return trace.AccessDenied("This Teleport instance is running on Teleport Cloud. "+
			"The %q resource is managed by the Teleport Cloud team. You can use the %q resource to opt-in, "+
			"opt-out or configure update schedules.",
			types.KindAutoUpdateVersion, types.KindAutoUpdateConfig)
	}
	return nil
}
