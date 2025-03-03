// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package userprovisioningv2

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the static host user gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Emitter is used to send audit events in response to processing requests.
	Emitter apievents.Emitter
	// Backend is the backend used to store static host users.
	Backend services.StaticHostUser
	// Cache is the cache used to store static host users.
	Cache Cache
}

// Cache is a subset of the service interface for reading items from the cache.
type Cache interface {
	// ListStaticHostUsers lists static host users.
	ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningpb.StaticHostUser, string, error)
	// GetStaticHostUser returns a static host user by name.
	GetStaticHostUser(ctx context.Context, name string) (*userprovisioningpb.StaticHostUser, error)
}

// Service implements the static host user RPC service.
type Service struct {
	userprovisioningpb.UnimplementedStaticHostUsersServiceServer

	authorizer authz.Authorizer
	emitter    apievents.Emitter
	backend    services.StaticHostUser
	cache      Cache
}

// NewService creates a new static host user gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
	}, nil
}

// ListStaticHostUsers lists static host users.
func (s *Service) ListStaticHostUsers(ctx context.Context, req *userprovisioningpb.ListStaticHostUsersRequest) (*userprovisioningpb.ListStaticHostUsersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	users, nextToken, err := s.cache.ListStaticHostUsers(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &userprovisioningpb.ListStaticHostUsersResponse{
		Users:         users,
		NextPageToken: nextToken,
	}, nil
}

// GetStaticHostUser returns a static host user by name.
func (s *Service) GetStaticHostUser(ctx context.Context, req *userprovisioningpb.GetStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("missing name")
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.cache.GetStaticHostUser(ctx, req.Name)
	return out, trace.Wrap(err)
}

func eventStatus(err error) apievents.Status {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return apievents.Status{
		Success:     err == nil,
		Error:       msg,
		UserMessage: msg,
	}
}

// CreateStaticHostUser creates a static host user.
func (s *Service) CreateStaticHostUser(ctx context.Context, req *userprovisioningpb.CreateStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.CreateStaticHostUser(ctx, req.User)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.StaticHostUserCreate{
		Metadata: apievents.Metadata{
			Type: events.StaticHostUserCreateEvent,
			Code: events.StaticHostUserCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.User.Metadata.Name,
		},
		Status: eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit static host user create event event.", "error", err)
	}

	return out, trace.Wrap(err)
}

// UpdateStaticHostUser updates a static host user.
func (s *Service) UpdateStaticHostUser(ctx context.Context, req *userprovisioningpb.UpdateStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.UpdateStaticHostUser(ctx, req.User)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.StaticHostUserUpdate{
		Metadata: apievents.Metadata{
			Type: events.StaticHostUserUpdateEvent,
			Code: events.StaticHostUserUpdateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.User.Metadata.Name,
		},
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit static host user update event event.", "error", err)
	}

	return out, trace.Wrap(err)
}

// UpsertStaticHostUser upserts a static host user.
func (s *Service) UpsertStaticHostUser(ctx context.Context, req *userprovisioningpb.UpsertStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.UpsertStaticHostUser(ctx, req.User)
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.StaticHostUserCreate{
		Metadata: apievents.Metadata{
			Type: events.StaticHostUserCreateEvent,
			Code: events.StaticHostUserCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.User.Metadata.Name,
		},
		Status: eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit static host user create event event.", "error", err)
	}

	return out, trace.Wrap(err)
}

// DeleteStaticHostUser deletes a static host user. Note that this does not
// remove any host users created on nodes from the resource.
func (s *Service) DeleteStaticHostUser(ctx context.Context, req *userprovisioningpb.DeleteStaticHostUserRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("missing name")
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteStaticHostUser(ctx, req.Name)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.StaticHostUserDelete{
		Metadata: apievents.Metadata{
			Type: events.StaticHostUserDeleteEvent,
			Code: events.StaticHostUserDeleteCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
		Status: eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit static host user delete event event.", "error", err)
	}

	return &emptypb.Empty{}, trace.Wrap(err)
}
