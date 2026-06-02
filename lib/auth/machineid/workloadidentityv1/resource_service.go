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
	"iter"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type workloadIdentityReader interface {
	GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error)
	ListWorkloadIdentities(ctx context.Context, pageSize int, token string, options *services.ListWorkloadIdentitiesRequestOptions) ([]*workloadidentityv1pb.WorkloadIdentity, string, error)
}

type workloadIdentityReadWriter interface {
	workloadIdentityReader

	CreateWorkloadIdentity(ctx context.Context, identity *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, error)
	UpdateWorkloadIdentity(ctx context.Context, identity *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, error)
	DeleteWorkloadIdentity(ctx context.Context, name string) error
	UpsertWorkloadIdentity(ctx context.Context, identity *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, error)
}

// workloadIdentityCache is the read interface used for listing. In addition to
// single-resource reads it exposes ranged iteration (used to build scoped,
// per-resource authorized listing) and the matching pagination cursor helper.
type workloadIdentityCache interface {
	GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error)
	RangeWorkloadIdentities(ctx context.Context, start, end, sortField string, desc bool) iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error]
}

// ResourceServiceConfig holds configuration options for the ResourceService.
type ResourceServiceConfig struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Backend          workloadIdentityReadWriter
	Cache            workloadIdentityCache
	Clock            clockwork.Clock
	Emitter          apievents.Emitter
	Logger           *slog.Logger
}

// ResourceService is the gRPC service for managing workload identity resources.
// It implements the workloadidentityv1pb.WorkloadIdentityResourceServiceServer
type ResourceService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityResourceServiceServer

	scopedAuthorizer authz.ScopedAuthorizer
	backend          workloadIdentityReadWriter
	cache            workloadIdentityCache
	clock            clockwork.Clock
	emitter          apievents.Emitter
	logger           *slog.Logger
}

// NewResourceService returns a new instance of the ResourceService.
func NewResourceService(cfg *ResourceServiceConfig) (*ResourceService, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
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
		scopedAuthorizer: cfg.ScopedAuthorizer,
		backend:          cfg.Backend,
		cache:            cfg.Cache,
		clock:            cfg.Clock,
		emitter:          cfg.Emitter,
		logger:           cfg.Logger,
	}, nil
}

// GetWorkloadIdentity returns a WorkloadIdentity by name.
// Implements teleport.workloadidentity.v1.ResourceService/GetWorkloadIdentity
func (s *ResourceService) GetWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if it's feasible that the caller has access before hitting the
	// cluster state backend.
	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindWorkloadIdentity, types.VerbReadNoSecrets,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	resource, err := s.cache.GetWorkloadIdentity(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx.Resource153 = resource
	if err := authCtx.CheckerContext.Decision(
		ctx, resource.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&ruleCtx, types.KindWorkloadIdentity, types.VerbReadNoSecrets)
		},
	); err != nil {
		// Return NotFound rather than Forbidden to avoid leaking the existence
		// of the WorkloadIdentity to callers without access to its scope.
		return nil, trace.NotFound("workload_identity %q not found", req.Name)
	}

	return resource, nil
}

// ListWorkloadIdentities returns a list of WorkloadIdentity resources. It
// follows the Google API design guidelines for list pagination.
// Implements teleport.workloadidentity.v1.ResourceService/ListWorkloadIdentities
func (s *ResourceService) ListWorkloadIdentities(
	ctx context.Context, req *workloadidentityv1pb.ListWorkloadIdentitiesRequest,
) (*workloadidentityv1pb.ListWorkloadIdentitiesResponse, error) {
	return s.ListWorkloadIdentitiesV2(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesV2Request{
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
}

// ListWorkloadIdentitiesV2 returns a list of WorkloadIdentity resources. It
// follows the Google API design guidelines for list pagination. It supports
// sorting and filtering.
// Implements teleport.workloadidentity.v1.ResourceService/ListWorkloadIdentitiesV2
func (s *ResourceService) ListWorkloadIdentitiesV2(
	ctx context.Context, req *workloadidentityv1pb.ListWorkloadIdentitiesV2Request,
) (*workloadidentityv1pb.ListWorkloadIdentitiesResponse, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check generally whether the caller may list WorkloadIdentities at all
	// (ignoring per-resource where conditions) before iterating cluster state.
	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindWorkloadIdentity, types.VerbList,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	var (
		out       []*workloadidentityv1pb.WorkloadIdentity
		nextToken string
	)
	
	for wi, err := range s.cache.RangeWorkloadIdentities(
		ctx, req.GetPageToken(), "", req.GetSortField(), req.GetSortDesc(),
	) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Apply the request's search-term filter.
		if !services.MatchWorkloadIdentity(wi, req.GetFilterSearchTerm()) {
			continue
		}

		// Filter by the caller's access to this specific resource's scope.
		ruleCtx := authCtx.RuleContext()
		ruleCtx.Resource153 = wi
		if err := authCtx.CheckerContext.Decision(
			ctx, wi.GetScope(), func(checker *services.ScopedAccessChecker) error {
				return checker.CheckAccessToRules(&ruleCtx, types.KindWorkloadIdentity, types.VerbList)
			},
		); err != nil {
			// Silently omit resources the caller cannot access.
			continue
		}

		// The page is full: this item begins the next page, so emit its key as
		// the continuation token and stop.
		if len(out) == pageSize {
			nextToken, err = services.WorkloadIdentitySortKey(wi, req.GetSortField())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			break
		}
		out = append(out, wi)
	}

	return &workloadidentityv1pb.ListWorkloadIdentitiesResponse{
		WorkloadIdentities: out,
		NextPageToken:      nextToken,
	}, nil
}

// DeleteWorkloadIdentity deletes a WorkloadIdentity by name.
// Implements teleport.workloadidentity.v1.ResourceService/DeleteWorkloadIdentity
func (s *ResourceService) DeleteWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.DeleteWorkloadIdentityRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	// Perform a maybe-check before we hit the backend.
	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindWorkloadIdentity, types.VerbDelete,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the existing resource to determine its scope for authorization.
	existing, err := s.backend.GetWorkloadIdentity(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx.Resource153 = existing
	if err := authCtx.CheckerContext.Decision(
		ctx, existing.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&ruleCtx, types.KindWorkloadIdentity, types.VerbDelete)
		},
	); err != nil {
		// Return NotFound rather than Forbidden to avoid leaking the existence
		// of the WorkloadIdentity to callers without access to its scope.
		return nil, trace.NotFound("workload_identity %q not found", req.Name)
	}
	// Admin action MFA can only be enforced on unscoped identities.
	// TODO(strideynet): When scoped identities support MFA, enforce it here.
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		if err := unscoped.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := s.backend.DeleteWorkloadIdentity(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityDelete{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityDeleteCode,
			Type: events.WorkloadIdentityDeleteEvent,
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
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Scoped resources require the scopes feature to be enabled. Structural
	// validation of the scope and SPIFFE ID is performed by the backend via
	// services.ValidateWorkloadIdentity.
	if req.WorkloadIdentity.GetScope() != "" {
		if err := scopes.AssertFeatureEnabled(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = req.WorkloadIdentity
	if err := authCtx.CheckerContext.Decision(
		ctx, req.WorkloadIdentity.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&ruleCtx, types.KindWorkloadIdentity, types.VerbCreate)
		},
	); err != nil {
		return nil, trace.Wrap(err)
	}
	// Admin action MFA can only be enforced on unscoped identities.
	// TODO(strideynet): When scoped identities support MFA, enforce it here.
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		if err := unscoped.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	created, err := s.backend.CreateWorkloadIdentity(ctx, req.WorkloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityCreate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityCreateCode,
			Type: events.WorkloadIdentityCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.WorkloadIdentity.Metadata.Name,
		},
	}
	evt.WorkloadIdentityData, err = resourceToStruct(created)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to convert WorkloadIdentity to struct for audit log",
			"error", err,
		)
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
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
	ctx context.Context, req *workloadidentityv1pb.UpdateWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Scoped resources require the scopes feature to be enabled. Structural
	// validation of the scope and SPIFFE ID is performed by the backend via
	// services.ValidateWorkloadIdentity.
	if req.WorkloadIdentity.GetScope() != "" {
		if err := scopes.AssertFeatureEnabled(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = req.WorkloadIdentity
	if err := authCtx.CheckerContext.Decision(
		ctx, req.WorkloadIdentity.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&ruleCtx, types.KindWorkloadIdentity, types.VerbUpdate)
		},
	); err != nil {
		return nil, trace.Wrap(err)
	}
	// Admin action MFA can only be enforced on unscoped identities.
	// TODO(strideynet): When scoped identities support MFA, enforce it here.
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		if err := unscoped.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := s.checkNoScopeTransition(ctx, req.WorkloadIdentity); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.UpdateWorkloadIdentity(ctx, req.WorkloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityUpdate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityUpdateCode,
			Type: events.WorkloadIdentityUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.WorkloadIdentity.Metadata.Name,
		},
	}
	evt.WorkloadIdentityData, err = resourceToStruct(created)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to convert WorkloadIdentity to struct for audit log",
			"error", err,
		)
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for updating of WorkloadIdentity",
			"error", err,
		)
	}

	return created, nil
}

// UpsertWorkloadIdentity updates or creates an existing WorkloadIdentity.
// Implements teleport.workloadidentity.v1.ResourceService/UpsertWorkloadIdentity
func (s *ResourceService) UpsertWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.UpsertWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Scoped resources require the scopes feature to be enabled. Structural
	// validation of the scope and SPIFFE ID is performed by the backend via
	// services.ValidateWorkloadIdentity.
	if req.WorkloadIdentity.GetScope() != "" {
		if err := scopes.AssertFeatureEnabled(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = req.WorkloadIdentity
	if err := authCtx.CheckerContext.Decision(
		ctx, req.WorkloadIdentity.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToRules(
				&ruleCtx, types.KindWorkloadIdentity, types.VerbCreate, types.VerbUpdate,
			)
		},
	); err != nil {
		return nil, trace.Wrap(err)
	}
	// Admin action MFA can only be enforced on unscoped identities.
	// TODO(strideynet): When scoped identities support MFA, enforce it here.
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		if err := unscoped.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := s.checkNoScopeTransition(ctx, req.WorkloadIdentity); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.UpsertWorkloadIdentity(ctx, req.WorkloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityCreate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityCreateCode,
			Type: events.WorkloadIdentityCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.WorkloadIdentity.Metadata.Name,
		},
	}
	evt.WorkloadIdentityData, err = resourceToStruct(created)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to convert WorkloadIdentity to struct for audit log",
			"error", err,
		)
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for upsertion of WorkloadIdentity",
			"error", err,
		)
	}

	return created, nil
}

// checkNoScopeTransition enforces that an update or upsert does not change the
// scope of an existing WorkloadIdentity (including transitioning to or from
// unscoped). To change scope, the resource must be deleted and recreated. This
// keeps authorization for update/upsert simple, as we do not need to evaluate
// authz against both the pre- and post-mutation scopes.
func (s *ResourceService) checkNoScopeTransition(
	ctx context.Context, incoming *workloadidentityv1pb.WorkloadIdentity,
) error {
	existing, err := s.backend.GetWorkloadIdentity(ctx, incoming.GetMetadata().GetName())
	if err != nil {
		if trace.IsNotFound(err) {
			// No existing resource (e.g. an upsert that creates a new resource);
			// there is nothing to transition from.
			return nil
		}
		return trace.Wrap(err)
	}
	if existing.GetScope() != incoming.GetScope() {
		return trace.BadParameter(
			"the scope of a workload_identity cannot be changed, delete and recreate the resource to change its scope",
		)
	}
	return nil
}

func resourceToStruct(in *workloadidentityv1pb.WorkloadIdentity) (*apievents.Struct, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(in)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling resource for audit log")
	}
	out := &apievents.Struct{}
	if err := out.UnmarshalJSON(data); err != nil {
		return nil, trace.Wrap(err, "unmarshaling resource for audit log")
	}
	return out, nil
}
