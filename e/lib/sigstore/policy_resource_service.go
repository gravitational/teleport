package sigstore

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// PolicyResourceService implements the gRPC service for managing SigstorePolicy
// resources.
type PolicyResourceService struct {
	workloadidentityv1.UnimplementedSigstorePolicyResourceServiceServer

	backend    services.SigstorePolicies
	authorizer authz.Authorizer
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// PolicyResourceServiceConfig holds the configuration for the PolicyResourceService.
type PolicyResourceServiceConfig struct {
	Backend    services.SigstorePolicies
	Authorizer authz.Authorizer
	Emitter    apievents.Emitter
	Logger     *slog.Logger
}

func (cfg *PolicyResourceServiceConfig) CheckAndSetDefaults() error {
	switch {
	case cfg.Backend == nil:
		return trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return trace.BadParameter("emitter is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// NewPolicyResourceService returns the Enterprise Edition version of the
// gRPC service for managing SigstorePolicy resources.
func NewPolicyResourceService(cfg PolicyResourceServiceConfig) (*PolicyResourceService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &PolicyResourceService{
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
		logger:     cfg.Logger.With(teleport.ComponentKey, "sigstore"),
	}, nil
}

// CreateSigstorePolicy creates a new Sigstore policy.
func (s *PolicyResourceService) CreateSigstorePolicy(ctx context.Context, req *workloadidentityv1.CreateSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSigstorePolicy, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateSigstorePolicy(ctx, req.GetSigstorePolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.SigstorePolicyCreate{
		Metadata: apievents.Metadata{
			Code: events.SigstorePolicyCreateCode,
			Type: events.SigstorePolicyCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
	}); err != nil {
		s.logger.ErrorContext(ctx,
			"Failed to emit audit event for creation of SigstorePolicy",
			"error", err,
			"sigstore_policy_name", created.GetMetadata().GetName(),
		)
	}

	return created, nil
}

// GetSigstorePolicy reads a Sigstore policy by name.
func (s *PolicyResourceService) GetSigstorePolicy(ctx context.Context, req *workloadidentityv1.GetSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSigstorePolicy, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.GetName() == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	policy, err := s.backend.GetSigstorePolicy(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return policy, nil
}

// DeleteSigstorePolicy reads a Sigstore policy by name.
func (s *PolicyResourceService) DeleteSigstorePolicy(ctx context.Context, req *workloadidentityv1.DeleteSigstorePolicyRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSigstorePolicy, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	if err := s.backend.DeleteSigstorePolicy(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.SigstorePolicyDelete{
		Metadata: apievents.Metadata{
			Code: events.SigstorePolicyDeleteCode,
			Type: events.SigstorePolicyDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.GetName(),
		},
	}); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for deletion of SigstorePolicy",
			"error", err,
			"sigstore_policy_name", req.GetName(),
		)
	}

	return &emptypb.Empty{}, nil
}

// UpdateSigstorePolicy updates a Sigstore policy.
func (s *PolicyResourceService) UpdateSigstorePolicy(ctx context.Context, req *workloadidentityv1.UpdateSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSigstorePolicy, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpdateSigstorePolicy(ctx, req.GetSigstorePolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.SigstorePolicyUpdate{
		Metadata: apievents.Metadata{
			Code: events.SigstorePolicyUpdateCode,
			Type: events.SigstorePolicyUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: updated.GetMetadata().GetName(),
		},
	}); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for update of SigstorePolicy",
			"error", err,
			"sigstore_policy_name", updated.GetMetadata().GetName(),
		)
	}

	return updated, nil
}

// UpsertSigstorePolicy creates or updates an existing Sigstore policy.
func (s *PolicyResourceService) UpsertSigstorePolicy(ctx context.Context, req *workloadidentityv1.UpsertSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSigstorePolicy, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.UpsertSigstorePolicy(ctx, req.GetSigstorePolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.SigstorePolicyCreate{
		Metadata: apievents.Metadata{
			Code: events.SigstorePolicyCreateCode,
			Type: events.SigstorePolicyCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
	}); err != nil {
		s.logger.ErrorContext(ctx,
			"Failed to emit audit event for upsert of SigstorePolicy",
			"error", err,
			"sigstore_policy_name", created.GetMetadata().GetName(),
		)
	}

	return created, nil
}

// ListSigstorePolicies returns a list of Sigstore policies.
func (s *PolicyResourceService) ListSigstorePolicies(ctx context.Context, req *workloadidentityv1.ListSigstorePoliciesRequest) (*workloadidentityv1.ListSigstorePoliciesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSigstorePolicy, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	resources, nextToken, err := s.backend.ListSigstorePolicies(
		ctx,
		int(req.GetPageSize()),
		req.GetPageToken(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadidentityv1.ListSigstorePoliciesResponse_builder{
		SigstorePolicies: resources,
		NextPageToken:    nextToken,
	}.Build(), nil
}
