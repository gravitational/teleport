package workloadidentityv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	eservices "github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

type X509OverridesServiceConfig struct {
	Authorizer authz.Authorizer
	Storage    services.WorkloadIdentityX509Overrides
	Emitter    apievents.Emitter

	ClusterName string
}

// NewX509OverridesService returns a fully featured implementation of
// [workloadidentityv1pb.X509OverridesServiceServer], unlike the OSS
// implementation in lib/auth/machineid/workloadidentityv1.
func NewX509OverridesService(cfg X509OverridesServiceConfig) (*X509OverridesService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Storage == nil {
		return nil, trace.BadParameter("storage is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("cluster name is required")
	}

	return &X509OverridesService{
		authorizer: cfg.Authorizer,
		storage:    cfg.Storage,
		emitter:    cfg.Emitter,

		clusterName: cfg.ClusterName,
	}, nil
}

// X509OverridesService is a fully featured implementation of
// [workloadidentityv1pb.X509OverridesServiceServer].
type X509OverridesService struct {
	workloadidentityv1pb.UnimplementedX509OverridesServiceServer

	authorizer authz.Authorizer
	storage    services.WorkloadIdentityX509Overrides
	emitter    apievents.Emitter

	clusterName string
}

var _ workloadidentityv1pb.X509OverridesServiceServer = (*X509OverridesService)(nil)

func (s *X509OverridesService) authorizeAccessToKind(ctx context.Context, kind string, verb string, additionalVerbs ...string) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return authzCtx.CheckAccessToKind(kind, verb, additionalVerbs...)
}

func (s *X509OverridesService) authorizeAccessToKindAdminReusedMFA(ctx context.Context, kind string, verb string, additionalVerbs ...string) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authzCtx.CheckAccessToKind(kind, verb, additionalVerbs...); err != nil {
		return trace.Wrap(err)
	}
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) GetX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.GetX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.GetX509IssuerOverride(ctx, req.GetName())
}

// ListX509IssuerOverrides implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) ListX509IssuerOverrides(ctx context.Context, req *workloadidentityv1pb.ListX509IssuerOverridesRequest) (*workloadidentityv1pb.ListX509IssuerOverridesResponse, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbList, apitypes.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	overrides, nextPageToken, err := s.storage.ListX509IssuerOverrides(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.ListX509IssuerOverridesResponse{
		X509IssuerOverrides: overrides,
		NextPageToken:       nextPageToken,
	}, nil
}

// CreateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) CreateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.CreateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKindAdminReusedMFA(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetX509IssuerOverride()
	if _, err := eservices.ParseWorkloadIdentityX509IssuerOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	newResource, err := s.storage.CreateX509IssuerOverride(ctx, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityX509IssuerOverrideCreate{
		Metadata: apievents.Metadata{
			Type: events.WorkloadIdentityX509IssuerOverrideCreateEvent,
			Code: events.WorkloadIdentityX509IssuerOverrideCreateCode,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: newResource.GetMetadata().GetName(),
		},
	})

	return newResource, nil
}

// UpdateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpdateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpdateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKindAdminReusedMFA(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetX509IssuerOverride()
	if _, err := eservices.ParseWorkloadIdentityX509IssuerOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	newResource, err := s.storage.UpdateX509IssuerOverride(ctx, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityX509IssuerOverrideCreate{
		Metadata: apievents.Metadata{
			Type: events.WorkloadIdentityX509IssuerOverrideCreateEvent,
			Code: events.WorkloadIdentityX509IssuerOverrideCreateCode,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: newResource.GetMetadata().GetName(),
		},
	})

	return newResource, nil
}

// UpsertX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpsertX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpsertX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKindAdminReusedMFA(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbCreate, apitypes.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetX509IssuerOverride()
	if _, err := eservices.ParseWorkloadIdentityX509IssuerOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	newResource, err := s.storage.UpsertX509IssuerOverride(ctx, req.GetX509IssuerOverride())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityX509IssuerOverrideCreate{
		Metadata: apievents.Metadata{
			Type: events.WorkloadIdentityX509IssuerOverrideCreateEvent,
			Code: events.WorkloadIdentityX509IssuerOverrideCreateCode,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: newResource.GetMetadata().GetName(),
		},
	})

	return newResource, nil
}

// DeleteX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) DeleteX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.DeleteX509IssuerOverrideRequest) (*emptypb.Empty, error) {
	if err := s.authorizeAccessToKindAdminReusedMFA(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteX509IssuerOverride(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityX509IssuerOverrideDelete{
		Metadata: apievents.Metadata{
			Type: events.WorkloadIdentityX509IssuerOverrideDeleteEvent,
			Code: events.WorkloadIdentityX509IssuerOverrideDeleteCode,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.GetName(),
		},
	})

	return &emptypb.Empty{}, nil
}
