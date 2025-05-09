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

package crownjewelv1

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/crownjewel"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the CrownJewel gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing CrownJewel.
	Backend services.CrownJewels

	// Reader is the cache for storing CrownJewel.
	Reader Reader

	// Emitter is the event emitter.
	Emitter events.Emitter
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
	if s.Reader == nil {
		return trace.BadParameter("cache is required")
	}
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}

	return nil
}

type Reader interface {
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
	GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error)
}

// Service implements the teleport.CrownJewel.v1.CrownJewelService RPC service.
type Service struct {
	crownjewelv1.UnimplementedCrownJewelServiceServer

	authorizer authz.Authorizer
	backend    services.CrownJewels
	reader     Reader
	emitter    events.Emitter
}

// NewService returns a new CrownJewel gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		reader:     cfg.Reader,
		emitter:    cfg.Emitter,
	}, nil
}

// CreateCrownJewel creates crown jewel resource.
func (s *Service) CreateCrownJewel(ctx context.Context, req *crownjewelv1.CreateCrownJewelRequest) (rec *crownjewelv1.CrownJewel, err error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := crownjewel.ValidateCrownJewel(req.CrownJewel); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.CreateCrownJewel(ctx, req.CrownJewel)

	s.emitCreateAuditEvent(ctx, rsp, authCtx, err)

	return rsp, trace.Wrap(err)
}

func (s *Service) emitCreateAuditEvent(ctx context.Context, req *crownjewelv1.CrownJewel, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &events.CrownJewelCreate{
		Metadata: events.Metadata{
			Type: libevents.CrownJewelCreateEvent,
			Code: libevents.CrownJewelCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: events.ResourceMetadata{
			Name:      req.GetMetadata().GetName(),
			Expires:   getExpires(req.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		CrownJewelQuery: req.GetSpec().GetQuery(),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit crown jewel create event.", "error", auditErr)
	}
}

// ListCrownJewels returns a list of crown jewels.
func (s *Service) ListCrownJewels(ctx context.Context, req *crownjewelv1.ListCrownJewelsRequest) (*crownjewelv1.ListCrownJewelsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.reader.ListCrownJewels(ctx, req.PageSize, req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &crownjewelv1.ListCrownJewelsResponse{
		CrownJewels:   rsp,
		NextPageToken: nextToken,
	}, nil
}

// GetCrownJewel returns crown jewel resource.
func (s *Service) GetCrownJewel(ctx context.Context, req *crownjewelv1.GetCrownJewelRequest) (*crownjewelv1.CrownJewel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.reader.GetCrownJewel(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// UpdateCrownJewel updates crown jewel resource.
func (s *Service) UpdateCrownJewel(ctx context.Context, req *crownjewelv1.UpdateCrownJewelRequest) (*crownjewelv1.CrownJewel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := crownjewel.ValidateCrownJewel(req.CrownJewel); err != nil {
		return nil, trace.Wrap(err)
	}

	oldCrownJewel, err := s.reader.GetCrownJewel(ctx, req.GetCrownJewel().GetMetadata().GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateCrownJewel(ctx, req.CrownJewel)

	s.emitUpdateAuditEvent(ctx, oldCrownJewel, req.GetCrownJewel(), authCtx, err)

	return rsp, trace.Wrap(err)
}

func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *crownjewelv1.CrownJewel, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &events.CrownJewelUpdate{
		Metadata: events.Metadata{
			Type: libevents.CrownJewelUpdateEvent,
			Code: libevents.CrownJewelUpdateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: events.ResourceMetadata{
			Name:      new.GetMetadata().GetName(),
			Expires:   getExpires(new.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		CurrentCrownJewelQuery: old.GetSpec().GetQuery(),
		UpdatedCrownJewelQuery: new.GetSpec().GetQuery(),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit crown jewel update event.", "error", auditErr)
	}
}

// UpsertCrownJewel upserts crown jewel resource.
func (s *Service) UpsertCrownJewel(ctx context.Context, req *crownjewelv1.UpsertCrownJewelRequest) (*crownjewelv1.CrownJewel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbUpdate, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := crownjewel.ValidateCrownJewel(req.CrownJewel); err != nil {
		return nil, trace.Wrap(err)
	}

	oldCrownJewel, err := s.reader.GetCrownJewel(ctx, req.GetCrownJewel().GetMetadata().GetName())
	if err != nil && !trace.IsNotFound(err) { // ignore not found errors
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpsertCrownJewel(ctx, req.CrownJewel)

	s.emitUpsertAuditEvent(ctx, oldCrownJewel, req.GetCrownJewel(), authCtx, err)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

func (s *Service) emitUpsertAuditEvent(ctx context.Context, old, new *crownjewelv1.CrownJewel, authCtx *authz.Context, err error) {
	if old == nil {
		s.emitCreateAuditEvent(ctx, new, authCtx, err)
		return
	}
	s.emitUpdateAuditEvent(ctx, old, new, authCtx, err)
}

// DeleteCrownJewel deletes crown jewel resource.
func (s *Service) DeleteCrownJewel(ctx context.Context, req *crownjewelv1.DeleteCrownJewelRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteCrownJewel(ctx, req.GetName())

	if auditErr := s.emitter.EmitAuditEvent(ctx, &events.CrownJewelDelete{
		Metadata: events.Metadata{
			Type: libevents.CrownJewelDeleteEvent,
			Code: libevents.CrownJewelDeleteCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: events.ResourceMetadata{
			Name:      req.GetName(),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit crown jewel delete event.", "error", auditErr)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func eventStatus(err error) events.Status {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return events.Status{
		Success:     err == nil,
		Error:       msg,
		UserMessage: msg,
	}
}

func getExpires(cj *timestamppb.Timestamp) time.Time {
	if cj == nil {
		return time.Time{}
	}
	return cj.AsTime()
}
