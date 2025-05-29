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
	"maps"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/autoupdate/rollout"
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

	// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout from the backend.
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)

	// GetAutoUpdateAgentReport gets a AutoUpdateAgentReport from the backend.
	GetAutoUpdateAgentReport(ctx context.Context, name string) (*autoupdate.AutoUpdateAgentReport, error)

	// ListAutoUpdateAgentReports lists all AutoUpdateAgentReport from the backend.
	ListAutoUpdateAgentReports(ctx context.Context, pageSize int, pageToken string) ([]*autoupdate.AutoUpdateAgentReport, string, error)
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

// Backend interface for manipulating AutoUpdate resources.
type Backend interface {
	services.AutoUpdateService
}

// Service implements the gRPC API layer for the AutoUpdate.
type Service struct {
	autoupdate.UnimplementedAutoUpdateServiceServer

	authorizer authz.Authorizer
	backend    services.AutoUpdateService
	emitter    apievents.Emitter
	cache      Cache
	clock      clockwork.Clock
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
		clock:      clockwork.NewRealClock(),
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

	if err := validateServerSideAgentConfig(req.Config); err != nil {
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateServerSideAgentConfig(req.Config); err != nil {
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateServerSideAgentConfig(req.Config); err != nil {
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

// UpsertAutoUpdateConfig creates a new AutoUpdateConfig or forcefully updates an existing AutoUpdateConfig.
// This is a function rather than a method so that it can be used by the gRPC service
// and the auth server init code when dealing with resources to be applied at startup.
func UpsertAutoUpdateConfig(
	ctx context.Context,
	backend Backend,
	config *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	if err := validateServerSideAgentConfig(config); err != nil {
		return nil, trace.Wrap(err, "validating config")
	}
	out, err := backend.UpsertAutoUpdateConfig(ctx, config)
	return out, trace.Wrap(err)
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
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

// UpsertAutoUpdateVersion creates a new AutoUpdateVersion or forcefully updates an existing AutoUpdateVersion.
// This is a function rather than a method so that it can be used by the gRPC service
// and the auth server init code when dealing with resources to be applied at startup.
func UpsertAutoUpdateVersion(
	ctx context.Context,
	backend Backend,
	version *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	out, err := backend.UpsertAutoUpdateVersion(ctx, version)
	return out, trace.Wrap(err)
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

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
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

// GetAutoUpdateAgentRollout gets the current AutoUpdateAgentRollout singleton.
func (s *Service) GetAutoUpdateAgentRollout(ctx context.Context, req *autoupdate.GetAutoUpdateAgentRolloutRequest) (*autoupdate.AutoUpdateAgentRollout, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := s.cache.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return plan, nil
}

// CreateAutoUpdateAgentRollout creates AutoUpdateAgentRollout singleton.
func (s *Service) CreateAutoUpdateAgentRollout(ctx context.Context, req *autoupdate.CreateAutoUpdateAgentRolloutRequest) (*autoupdate.AutoUpdateAgentRollout, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing the AU agent plan is restricted to cluster administrators. As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	// a first class citizen, we'll want to update this permission check.
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateAgentRollout, err := s.backend.CreateAutoUpdateAgentRollout(ctx, req.Rollout)
	return autoUpdateAgentRollout, trace.Wrap(err)
}

// UpdateAutoUpdateAgentRollout updates AutoUpdateAgentRollout singleton.
func (s *Service) UpdateAutoUpdateAgentRollout(ctx context.Context, req *autoupdate.UpdateAutoUpdateAgentRolloutRequest) (*autoupdate.AutoUpdateAgentRollout, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing the AU agent plan is restricted to cluster administrators. As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	// a first class citizen, we'll want to update this permission check.
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateAgentRollout, err := s.backend.UpdateAutoUpdateAgentRollout(ctx, req.Rollout)
	return autoUpdateAgentRollout, trace.Wrap(err)
}

// UpsertAutoUpdateAgentRollout updates or creates AutoUpdateAgentRollout singleton.
func (s *Service) UpsertAutoUpdateAgentRollout(ctx context.Context, req *autoupdate.UpsertAutoUpdateAgentRolloutRequest) (*autoupdate.AutoUpdateAgentRollout, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing the AU agent plan is restricted to cluster administrators. As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	// a first class citizen, we'll want to update this permission check.
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateAgentRollout, err := s.backend.UpsertAutoUpdateAgentRollout(ctx, req.Rollout)
	return autoUpdateAgentRollout, trace.Wrap(err)
}

// DeleteAutoUpdateAgentRollout deletes AutoUpdateAgentRollout singleton.
func (s *Service) DeleteAutoUpdateAgentRollout(ctx context.Context, req *autoupdate.DeleteAutoUpdateAgentRolloutRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing the AU agent plan is restricted to cluster administrators. As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	// a first class citizen, we'll want to update this permission check.
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAutoUpdateAgentRollout(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) getAllReports(ctx context.Context) ([]*autoupdate.AutoUpdateAgentReport, error) {
	var reports []*autoupdate.AutoUpdateAgentReport

	// this is an in-memory client, we go for the default page size
	const pageSize = 0
	var pageToken string
	for {
		page, nextToken, err := s.cache.ListAutoUpdateAgentReports(ctx, pageSize, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, page...)
		if nextToken == "" {
			return reports, nil
		}
		pageToken = nextToken
	}
}

// TriggerAutoUpdateAgentGroup triggers automatic updates for one or many groups
// in the rollout.
func (s *Service) TriggerAutoUpdateAgentGroup(ctx context.Context, req *autoupdate.TriggerAutoUpdateAgentGroupRequest) (result *autoupdate.AutoUpdateAgentRollout, err error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.AutoUpdateAgentRolloutTrigger{
			Metadata: apievents.Metadata{
				Type: events.AutoUpdateAgentRolloutTriggerEvent,
				Code: events.AutoUpdateAgentRolloutTriggerCode,
			},
			UserMetadata:       userMetadata,
			Groups:             req.Groups,
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
		})
	}()

	const maxTries = 3

	var existingRollout, newRollout *autoupdate.AutoUpdateAgentRollout
	for range maxTries {
		existingRollout, err = s.backend.GetAutoUpdateAgentRollout(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "getting rollout")
		}
		reports, err := s.getAllReports(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "getting reports")
		}

		err = rollout.TriggerGroups(existingRollout, reports, rollout.GroupListToGroupSet(req.Groups), req.DesiredState, s.clock.Now())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		newRollout, err = s.backend.UpdateAutoUpdateAgentRollout(ctx, existingRollout)
		if err == nil {
			return newRollout, nil
		}
		if !trace.IsCompareFailed(err) {
			return nil, trace.Wrap(err)
		}
	}
	return nil, trace.LimitExceeded("max update tries exceeded")
}

// ForceAutoUpdateAgentGroup forces one or many groups of the agent rollout into
// a DONE state.
func (s *Service) ForceAutoUpdateAgentGroup(ctx context.Context, req *autoupdate.ForceAutoUpdateAgentGroupRequest) (result *autoupdate.AutoUpdateAgentRollout, err error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.AutoUpdateAgentRolloutForceDone{
			Metadata: apievents.Metadata{
				Type: events.AutoUpdateAgentRolloutForceDoneEvent,
				Code: events.AutoUpdateAgentRolloutForceDoneCode,
			},
			UserMetadata:       userMetadata,
			Groups:             req.Groups,
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
		})
	}()

	const maxTries = 3

	var existingRollout, newRollout *autoupdate.AutoUpdateAgentRollout
	for range maxTries {
		existingRollout, err = s.backend.GetAutoUpdateAgentRollout(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = rollout.ForceGroupsDone(existingRollout, rollout.GroupListToGroupSet(req.Groups), s.clock.Now())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		newRollout, err = s.backend.UpdateAutoUpdateAgentRollout(ctx, existingRollout)
		if err == nil {
			return newRollout, nil
		}
		if !trace.IsCompareFailed(err) {
			return nil, trace.Wrap(err)
		}
	}
	return nil, trace.LimitExceeded("max update tries exceeded")
}

// RollbackAutoUpdateAgentGroup forces one or many groups of the agent rollout into
// a DONE state.
func (s *Service) RollbackAutoUpdateAgentGroup(ctx context.Context, req *autoupdate.RollbackAutoUpdateAgentGroupRequest) (result *autoupdate.AutoUpdateAgentRollout, err error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentRollout, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.Groups) == 0 && !req.AllStartedGroups {
		return nil, trace.BadParameter("at least one group must be specified or the all_started_groups flag set")
	}

	defer func() {
		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		userMetadata := authz.ClientUserMetadata(ctx)
		s.emitEvent(ctx, &apievents.AutoUpdateAgentRolloutRollback{
			Metadata: apievents.Metadata{
				Type: events.AutoUpdateAgentRolloutRollbackEvent,
				Code: events.AutoUpdateAgentRolloutRollbackCode,
			},
			UserMetadata:       userMetadata,
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status: apievents.Status{
				Success: err == nil,
				Error:   errMsg,
			},
			Groups: req.Groups,
		})
	}()

	const maxTries = 3

	var existingRollout, newRollout *autoupdate.AutoUpdateAgentRollout
	for range maxTries {
		existingRollout, err = s.backend.GetAutoUpdateAgentRollout(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		groups := rollout.GroupListToGroupSet(req.Groups)
		if req.AllStartedGroups {
			startedGroups := rollout.GetStartedGroups(existingRollout)
			for group := range startedGroups {
				groups[group] = struct{}{}
			}
		}

		if len(groups) == 0 {
			return nil, trace.AlreadyExists("no groups to rollback")
		}

		err = rollout.RollbackGroups(existingRollout, groups, s.clock.Now())

		if err != nil {
			return nil, trace.Wrap(err)
		}

		newRollout, err = s.backend.UpdateAutoUpdateAgentRollout(ctx, existingRollout)
		if err == nil {
			return newRollout, nil
		}
		if !trace.IsCompareFailed(err) {
			return nil, trace.Wrap(err)
		}
	}
	return nil, trace.LimitExceeded("max update tries exceeded")
}

func (s *Service) ListAutoUpdateAgentReports(ctx context.Context, req *autoupdate.ListAutoUpdateAgentReportsRequest) (*autoupdate.ListAutoUpdateAgentReportsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentReport, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	reports, nextKey, err := s.cache.ListAutoUpdateAgentReports(ctx, int(req.GetPageSize()), req.GetNextToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &autoupdate.ListAutoUpdateAgentReportsResponse{
		AutoupdateAgentReports: reports,
		NextKey:                nextKey,
	}, nil

}

// GetAutoUpdateAgentReport gets an AutoUpdateAgentReport.
func (s *Service) GetAutoUpdateAgentReport(ctx context.Context, req *autoupdate.GetAutoUpdateAgentReportRequest) (*autoupdate.AutoUpdateAgentReport, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentReport, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	report, err := s.cache.GetAutoUpdateAgentReport(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return report, nil
}

// CreateAutoUpdateAgentReport creates an AutoUpdateAgentReport.
func (s *Service) CreateAutoUpdateAgentReport(ctx context.Context, req *autoupdate.CreateAutoUpdateAgentReportRequest) (*autoupdate.AutoUpdateAgentReport, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing an agent report is restricted to cluster administrators.
	// As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	// a first class citizen, we'll want to update this permission check.
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentReport, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateAgentReport, err := s.backend.CreateAutoUpdateAgentReport(ctx, req.GetAutoupdateAgentReport())
	return autoUpdateAgentReport, trace.Wrap(err)
}

// UpdateAutoUpdateAgentReport updates an AutoUpdateAgentReport.
func (s *Service) UpdateAutoUpdateAgentReport(ctx context.Context, req *autoupdate.UpdateAutoUpdateAgentReportRequest) (*autoupdate.AutoUpdateAgentReport, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing an agent report is restricted to cluster administrators.
	// As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentReport, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateAgentReport, err := s.backend.UpdateAutoUpdateAgentReport(ctx, req.GetAutoupdateAgentReport())
	return autoUpdateAgentReport, trace.Wrap(err)
}

// UpsertAutoUpdateAgentReport updates or creates an AutoUpdateAgentReport.
func (s *Service) UpsertAutoUpdateAgentReport(ctx context.Context, req *autoupdate.UpsertAutoUpdateAgentReportRequest) (*autoupdate.AutoUpdateAgentReport, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing an agent report is restricted to cluster administrators.
	// As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentReport, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	autoUpdateAgentReport, err := s.backend.UpsertAutoUpdateAgentReport(ctx, req.GetAutoupdateAgentReport())
	return autoUpdateAgentReport, trace.Wrap(err)
}

// DeleteAutoUpdateAgentReport deletes an AutoUpdateAgentReport.
func (s *Service) DeleteAutoUpdateAgentReport(ctx context.Context, req *autoupdate.DeleteAutoUpdateAgentReportRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Editing an agent report is restricted to cluster administrators.
	// As of today we don't have any way of having
	// resources that can only be edited by Teleport Cloud (when running cloud-hosted).
	// The workaround is to check if the caller has the auth/admin system role.
	// This is not ideal as it forces local tctl usage and can be bypassed if the user is very creative.
	// In the future, if we expand the permission system and make cloud
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAuth)) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := authCtx.CheckAccessToKind(types.KindAutoUpdateAgentReport, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAutoUpdateAgentReport(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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

// Those values are arbitrary, we will want to increase them as we test. We will also want to modulate them based on the
// cluster context. We don't want people to craft schedules that can't realistically finish within a week on Cloud as
// we usually do weekly updates. However, self-hosted users can craft more complex schedules, slower rollouts, and shoot
// themselves in the foot if they want.
const (
	maxGroupsTimeBasedStrategy        = 20
	maxGroupsHaltOnErrorStrategy      = 10
	maxGroupsHaltOnErrorStrategyCloud = 4
	maxRolloutDurationCloudHours      = 72
)

var (
	cloudGroupUpdateDays = []string{"Mon", "Tue", "Wed", "Thu"}
)

// validateServerSideAgentConfig validates that the autoupdate_config.agent spec meets the cluster rules.
// Rules may vary based on the cluster, and over time.
//
// This function should not be confused with api/types/autoupdate.ValidateAutoUpdateConfig which validates the integrity
// of the resource and does not enforce potentially changing rules.
func validateServerSideAgentConfig(config *autoupdate.AutoUpdateConfig) error {
	agentsSpec := config.GetSpec().GetAgents()
	if agentsSpec == nil {
		return nil
	}
	// We must check resource integrity before, because it makes no sense to try to enforce rules on an invalid resource.
	// The generic backend service will likely check integrity again, but it's not a large performance problem.
	err := update.ValidateAutoUpdateConfig(config)
	if err != nil {
		return trace.Wrap(err, "validating autoupdate config")
	}

	isLimitedCloud := modules.GetModules().Features().Cloud &&
		!modules.GetModules().Features().Entitlements[entitlements.UnrestrictedManagedUpdates].Enabled

	var maxGroups int
	switch {
	case isLimitedCloud && agentsSpec.GetStrategy() == update.AgentsStrategyHaltOnError:
		maxGroups = maxGroupsHaltOnErrorStrategyCloud
	case agentsSpec.GetStrategy() == update.AgentsStrategyHaltOnError:
		maxGroups = maxGroupsHaltOnErrorStrategy
	case agentsSpec.GetStrategy() == update.AgentsStrategyTimeBased:
		maxGroups = maxGroupsTimeBasedStrategy
	default:
		return trace.BadParameter("unknown max group for strategy %v", agentsSpec.GetStrategy())
	}

	if len(agentsSpec.GetSchedules().GetRegular()) > maxGroups {
		return trace.BadParameter("max groups (%d) exceeded for strategy %s, %s schedule contains %d groups", maxGroups, agentsSpec.GetStrategy(), update.AgentsScheduleRegular, len(agentsSpec.GetSchedules().GetRegular()))
	}

	if !isLimitedCloud {
		return nil
	}

	cloudWeekdays, err := types.ParseWeekdays(cloudGroupUpdateDays)
	if err != nil {
		return trace.Wrap(err, "parsing cloud weekdays")
	}

	for i, group := range agentsSpec.GetSchedules().GetRegular() {
		weekdays, err := types.ParseWeekdays(group.Days)
		if err != nil {
			return trace.Wrap(err, "parsing weekdays from group %d", i)
		}

		if !maps.Equal(cloudWeekdays, weekdays) {
			return trace.BadParameter("weekdays must be set to %v in cloud", cloudGroupUpdateDays)
		}
	}

	if duration := computeMinRolloutTime(agentsSpec.GetSchedules().GetRegular()); duration > maxRolloutDurationCloudHours {
		return trace.BadParameter("rollout takes more than %d hours to complete: estimated completion time is %d hours", maxRolloutDurationCloudHours, duration)
	}

	return nil
}

func computeMinRolloutTime(groups []*autoupdate.AgentAutoUpdateGroup) int {
	if len(groups) == 0 {
		return 0
	}

	// We start the rollout at the first group hour, and we wait for the group to update (1 hour).
	hours := groups[0].StartHour + 1

	for _, group := range groups[1:] {
		previousStartHour := (hours - 1) % 24
		previousEndHour := hours % 24

		// compute the difference between the current hour and the group start hour
		// we then check if it's less than the WaitHours, in this case we wait a day
		diff := hourDifference(previousStartHour, group.StartHour)
		if diff < group.WaitHours%24 {
			hours += 24 + hourDifference(previousEndHour, group.StartHour)
		} else {
			hours += hourDifference(previousEndHour, group.StartHour)
		}

		// Handle the case where WaitHours is > 24
		// This is an integer division
		waitDays := group.WaitHours / 24
		// There's a special case where the difference modulo 24 is zero, the
		// wait hours are non-null, but we already waited 23 hours.
		// To avoid double counting we reduce the number of wait days by 1 if
		// it's not zero already.
		if diff == 0 {
			waitDays = max(waitDays-1, 0)
		}
		hours += waitDays * 24

		// We assume the group took an hour to update
		hours += 1
	}

	// We remove the group start hour we added initially
	return int(hours - groups[0].StartHour)
}

// hourDifference computed the difference between two hours.
func hourDifference(a, b int32) int32 {
	diff := b - a
	if diff < 0 {
		diff = diff + 24
	}
	return diff
}
