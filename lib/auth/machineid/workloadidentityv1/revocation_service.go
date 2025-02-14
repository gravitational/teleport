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

package workloadidentityv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

type workloadIdentityX509RevocationReadWriter interface {
	GetWorkloadIdentityX509Revocation(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	ListWorkloadIdentityX509Revocations(ctx context.Context, pageSize int, token string) ([]*workloadidentityv1pb.WorkloadIdentityX509Revocation, string, error)
	CreateWorkloadIdentityX509Revocation(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	UpdateWorkloadIdentityX509Revocation(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	DeleteWorkloadIdentityX509Revocation(ctx context.Context, name string) error
	UpsertWorkloadIdentityX509Revocation(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
}

// RevocationServiceConfig holds configuration options for the RevocationService.
type RevocationServiceConfig struct {
	Authorizer authz.Authorizer
	Store      workloadIdentityX509RevocationReadWriter
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
	Logger     *slog.Logger
}

// RevocationService is the gRPC service for managing workload identity
// revocations.
// It implements the workloadidentityv1pb.WorkloadIdentityRevocationServiceServer
type RevocationService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityRevocationServiceServer

	authorizer authz.Authorizer
	store      workloadIdentityX509RevocationReadWriter
	clock      clockwork.Clock
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewRevocationService returns a new instance of the RevocationService.
func NewRevocationService(cfg *RevocationServiceConfig) (*RevocationService, error) {
	switch {
	case cfg.Store == nil:
		return nil, trace.BadParameter("store service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_revocation.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &RevocationService{
		authorizer: cfg.Authorizer,
		store:      cfg.Store,
		clock:      cfg.Clock,
		emitter:    cfg.Emitter,
		logger:     cfg.Logger,
	}, nil
}

// GetWorkloadIdentityX509Revocation returns a WorkloadIdentityX509Revocation
// by name. An error is returned if the resource does not exist.
// Implements teleport.workloadidentity.v1.RevocationService/GetWorkloadIdentityX509Revocation
func (s *RevocationService) GetWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	resource, err := s.store.GetWorkloadIdentityX509Revocation(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}

// ListWorkloadIdentityX509Revocations returns a list of
// WorkloadIdentityX509Revocation resources. It follows the Google API design
// guidelines for list pagination.
// Implements teleport.workloadidentity.v1.RevocationService/ListWorkloadIdentityX509Revocations
func (s *RevocationService) ListWorkloadIdentityX509Revocations(
	ctx context.Context, req *workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest,
) (*workloadidentityv1pb.ListWorkloadIdentityX509RevocationsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	resources, nextToken, err := s.store.ListWorkloadIdentityX509Revocations(
		ctx,
		int(req.PageSize),
		req.PageToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsResponse{
		WorkloadIdentityX509Revocations: resources,
		NextPageToken:                   nextToken,
	}, nil
}

// DeleteWorkloadIdentityX509Revocation deletes a WorkloadIdentityX509Revocation
// by name. An error is returned if the resource does not exist.
// Implements teleport.workloadidentity.v1.RevocationService/DeleteWorkloadIdentityX509Revocation
func (s *RevocationService) DeleteWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	if err := s.store.DeleteWorkloadIdentityX509Revocation(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityX509RevocationDelete{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationDeleteCode,
			Type: events.WorkloadIdentityX509RevocationDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for UpsertWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return &emptypb.Empty{}, nil
}

// CreateWorkloadIdentityX509Revocation creates a new WorkloadIdentityX509Revocation.
// Implements teleport.workloadidentity.v1.RevocationService/CreateWorkloadIdentityX509Revocation
func (s *RevocationService) CreateWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.CreateWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityX509RevocationCreate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationCreateCode,
			Type: events.WorkloadIdentityX509RevocationCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
		Reason: created.GetSpec().GetReason(),
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for CreateWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return created, nil
}

// UpdateWorkloadIdentityX509Revocation updates an existing
// WorkloadIdentityX509Revocation.
// Implements teleport.workloadidentity.v1.RevocationService/UpdateWorkloadIdentityX509Revocation
func (s *RevocationService) UpdateWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.UpdateWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityX509RevocationUpdate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationUpdateCode,
			Type: events.WorkloadIdentityX509RevocationUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
		Reason: created.GetSpec().GetReason(),
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for UpdateWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return created, nil
}

// UpsertWorkloadIdentityX509Revocation updates or creates an existing
// WorkloadIdentityX509Revocation.
// Implements teleport.workloadidentity.v1.RevocationService/UpsertWorkloadIdentityX509Revocation
func (s *RevocationService) UpsertWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(
		types.KindWorkloadIdentityX509Revocation, types.VerbCreate, types.VerbUpdate,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.UpsertWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityX509RevocationCreate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationCreateCode,
			Type: events.WorkloadIdentityX509RevocationCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
		Reason: created.GetSpec().GetReason(),
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for UpsertWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return created, nil
}
