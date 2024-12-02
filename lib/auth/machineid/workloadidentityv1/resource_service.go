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

type workloadIdentityReader interface {
	GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error)
	ListWorkloadIdentities(ctx context.Context, pageSize int, token string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error)
}

type workloadIdentityReadWriter interface {
	workloadIdentityReader

	CreateWorkloadIdentity(ctx context.Context, identity *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, error)
	UpdateWorkloadIdentity(ctx context.Context, identity *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, error)
	DeleteWorkloadIdentity(ctx context.Context, name string) error
}

// ResourceServiceConfig holds configuration options for the ResourceService.
type ResourceServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    workloadIdentityReadWriter
	Cache      workloadIdentityReader
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
	Logger     *slog.Logger
}

// ResourceService is the gRPC service for managing workload identity resources.
// It implements the workloadidentityv1pb.WorkloadIdentityResourceServiceServer
type ResourceService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityResourceServiceServer

	authorizer authz.Authorizer
	backend    workloadIdentityReadWriter
	cache      workloadIdentityReader
	clock      clockwork.Clock
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewResourceService returns a new instance of the ResourceService.
func NewResourceService(cfg *ResourceServiceConfig) (*ResourceService, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_resource.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &ResourceService{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
		clock:      cfg.Clock,
		emitter:    cfg.Emitter,
		logger:     cfg.Logger,
	}, nil
}

// GetWorkloadIdentity returns a WorkloadIdentity by name.
// Implements teleport.workloadidentity.v1.ResourceService/GetWorkloadIdentity
func (s *ResourceService) GetWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	resource, err := s.cache.GetWorkloadIdentity(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}

// ListWorkloadIdentities returns a list of WorkloadIdentity resources. It
// follows the Google API design guidelines for list pagination.
// Implements teleport.workloadidentity.v1.ResourceService/ListWorkloadIdentities
func (s *ResourceService) ListWorkloadIdentities(
	ctx context.Context, req *workloadidentityv1pb.ListWorkloadIdentitiesRequest,
) (*workloadidentityv1pb.ListWorkloadIdentitiesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	resources, nextToken, err := s.cache.ListWorkloadIdentities(
		ctx,
		int(req.PageSize),
		req.PageToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.ListWorkloadIdentitiesResponse{
		WorkloadIdentities: resources,
		NextPageToken:      nextToken,
	}, nil
}

// DeleteWorkloadIdentity deletes a WorkloadIdentity by name.
// Implements teleport.workloadidentity.v1.ResourceService/DeleteWorkloadIdentity
func (s *ResourceService) DeleteWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.DeleteWorkloadIdentityRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	if err := s.backend.DeleteWorkloadIdentity(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityDelete{
		Metadata: apievents.Metadata{
			Code: events.SPIFFEFederationDeleteCode,
			Type: events.SPIFFEFederationDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
	}); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for deletion of WorkloadIdentity",
			"error", err,
		)
	}

	return &emptypb.Empty{}, nil
}

// CreateWorkloadIdentity creates a new WorkloadIdentity.
// Implements teleport.workloadidentity.v1.ResourceService/CreateWorkloadIdentity
func (s *ResourceService) CreateWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.CreateWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateWorkloadIdentity(ctx, req.WorkloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityCreate{
		Metadata: apievents.Metadata{
			Code: events.SPIFFEFederationCreateCode,
			Type: events.SPIFFEFederationCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.WorkloadIdentity.Metadata.Name,
		},
	}); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for creation of WorkloadIdentity",
			"error", err,
		)
	}

	return created, nil
}

// UpdateWorkloadIdentity updates an existing WorkloadIdentity.
// Implements teleport.workloadidentity.v1.ResourceService/UpdateWorkloadIdentity
func (s *ResourceService) UpdateWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.CreateWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateWorkloadIdentity(ctx, req.WorkloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityUpdate{
		Metadata: apievents.Metadata{
			Code: events.SPIFFEFederationCreateCode,
			Type: events.SPIFFEFederationCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.WorkloadIdentity.Metadata.Name,
		},
	}); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for creation of WorkloadIdentity",
			"error", err,
		)
	}

	return created, nil
}
