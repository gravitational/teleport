/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discoveryconfigv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	conv "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the DiscoveryConfig gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing DiscoveryConfigs.
	Backend services.DiscoveryConfigs

	// Clock is the clock.
	Clock clockwork.Clock

	// Emitter emits audit events.
	Emitter apievents.Emitter
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}

	if s.Logger == nil {
		s.Logger = logrus.New().WithField(trace.Component, "discoveryconfig_crud_service")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Service implements the teleport.DiscoveryConfig.v1.DiscoveryConfigService RPC service.
type Service struct {
	discoveryconfigv1.UnimplementedDiscoveryConfigServiceServer

	log        logrus.FieldLogger
	authorizer authz.Authorizer
	backend    services.DiscoveryConfigs
	clock      clockwork.Clock
	emitter    apievents.Emitter
}

// NewService returns a new DiscoveryConfigs gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:        cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		clock:      cfg.Clock,
		emitter:    cfg.Emitter,
	}, nil
}

// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
func (s *Service) ListDiscoveryConfigs(ctx context.Context, req *discoveryconfigv1.ListDiscoveryConfigsRequest) (*discoveryconfigv1.ListDiscoveryConfigsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextKey, err := s.backend.ListDiscoveryConfigs(ctx, int(req.GetPageSize()), req.GetNextToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dcs := make([]*discoveryconfigv1.DiscoveryConfig, len(results))
	for i, r := range results {
		dcs[i] = conv.ToProto(r)
	}

	return &discoveryconfigv1.ListDiscoveryConfigsResponse{
		DiscoveryConfigs: dcs,
		NextKey:          nextKey,
	}, nil
}

// GetDiscoveryConfig returns the specified DiscoveryConfig resource.
func (s *Service) GetDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.GetDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := s.backend.GetDiscoveryConfig(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(dc), nil
}

// CreateDiscoveryConfig creates a new DiscoveryConfig resource.
func (s *Service) CreateDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.CreateDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to an empty struct to clear any status that may have been set
	// in the request.
	dc.Status = discoveryconfig.Status{}

	resp, err := s.backend.CreateDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigCreate{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigCreateEvent,
			Code: events.DiscoveryConfigCreateCode,
		},
		UserMetadata: authCtx.Identity.GetIdentity().GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    resp.GetName(),
			Expires: resp.Expiry(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit discovery config create event.")
	}

	return conv.ToProto(resp), nil
}

// UpdateDiscoveryConfig updates an existing DiscoveryConfig.
func (s *Service) UpdateDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.UpdateDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to the existing status to ensure it is not cleared.
	oldDiscoveryConfig, err := s.backend.GetDiscoveryConfig(ctx, dc.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc.Status = oldDiscoveryConfig.Status

	resp, err := s.backend.UpdateDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigUpdateEvent,
			Code: events.DiscoveryConfigUpdateCode,
		},
		UserMetadata: authCtx.Identity.GetIdentity().GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    resp.GetName(),
			Expires: resp.Expiry(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit discovery config update event.")
	}

	return conv.ToProto(resp), nil
}

// UpsertDiscoveryConfig creates or updates a DiscoveryConfig.
func (s *Service) UpsertDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.UpsertDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to an empty struct to clear any status that may have been set
	// in the request.
	dc.Status = discoveryconfig.Status{}

	resp, err := s.backend.UpsertDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigCreate{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigCreateEvent,
			Code: events.DiscoveryConfigCreateCode,
		},
		UserMetadata: authCtx.Identity.GetIdentity().GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    resp.GetName(),
			Expires: resp.Expiry(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit discovery config create event.")
	}

	return conv.ToProto(resp), nil
}

// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
func (s *Service) DeleteDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.DeleteDiscoveryConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteDiscoveryConfig(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigDelete{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigDeleteEvent,
			Code: events.DiscoveryConfigDeleteCode,
		},
		UserMetadata: authCtx.Identity.GetIdentity().GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit discovery config delete event.")
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllDiscoveryConfigs removes all DiscoveryConfig resources.
func (s *Service) DeleteAllDiscoveryConfigs(ctx context.Context, _ *discoveryconfigv1.DeleteAllDiscoveryConfigsRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAllDiscoveryConfigs(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigDeleteAll{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigDeleteAllEvent,
			Code: events.DiscoveryConfigDeleteAllCode,
		},
		UserMetadata:       authCtx.Identity.GetIdentity().GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit discovery config delete all event.")
	}

	return &emptypb.Empty{}, nil
}

// UpdateDiscoveryConfigStatus updates the status of a DiscoveryConfig.
func (s *Service) UpdateDiscoveryConfigStatus(ctx context.Context, req *discoveryconfigv1.UpdateDiscoveryConfigStatusRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) {
		return nil, trace.AccessDenied("UpdateDiscoveryConfigStatus request can be only executed by a Discovery Service")
	}

	for {
		dc, err := s.backend.GetDiscoveryConfig(ctx, req.GetName())
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("discovery config %q not found", req.GetName())
		case err != nil:
			return nil, trace.Wrap(err)
		}

		dc.Status = conv.StatusFromProto(req.GetStatus())
		resp, err := s.backend.UpdateDiscoveryConfig(ctx, dc)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		return conv.ToProto(resp), nil
	}
}
