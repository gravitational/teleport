package tagv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	tagv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/tag/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the Tag gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.Tags
	Reader     Reader
	Emitter    apievents.Emitter
	// TODO: add resource-specific dependencies
}

// CheckAndSetDefaults checks required fields.
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if s.Reader == nil {
		return trace.BadParameter("reader is required")
	}
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}
	return nil
}

// Service implements the teleport.tag.v1.TagService gRPC service.
type Service struct {
	tagv1pb.UnimplementedTagServiceServer
	authorizer authz.Authorizer
	backend    services.Tags
	reader     Reader
	emitter    apievents.Emitter
	// TODO: add resource-specific fields
}

// NewService returns a new Tag gRPC service.
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

// authorize checks that the caller has the given verbs on the resource kind.
func (s *Service) authorize(ctx context.Context, verb string, verbs ...string) (*authz.Context, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindTag, verb, verbs...); err != nil {
		return nil, trace.Wrap(err)
	}
	return authCtx, nil
}

// authorizeMutation checks verbs and additionally validates admin action MFA.
func (s *Service) authorizeMutation(ctx context.Context, verb string, verbs ...string) (*authz.Context, error) {
	authCtx, err := s.authorize(ctx, verb, verbs...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	return authCtx, nil
}

func (s *Service) emitCreateAuditEvent(ctx context.Context, tag *tagv1pb.Tag, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.TagCreate{
		Metadata: apievents.Metadata{
			Type: libevents.TagCreateEvent,
			Code: libevents.TagCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      tag.GetMetadata().GetName(),
			Expires:   getExpires(tag.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		// FIXME: add resource-specific event fields here.
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit tag create event.", "error", auditErr)
	}
}

func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *tagv1pb.Tag, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.TagUpdate{
		Metadata: apievents.Metadata{
			Type: libevents.TagUpdateEvent,
			Code: libevents.TagUpdateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      new.GetMetadata().GetName(),
			Expires:   getExpires(new.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		// FIXME: add resource-specific event fields here.
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit tag update event.", "error", auditErr)
	}
}

func (s *Service) emitDeleteAuditEvent(ctx context.Context, name string, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.TagDelete{
		Metadata: apievents.Metadata{
			Type: libevents.TagDeleteEvent,
			Code: libevents.TagDeleteCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      name,
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit tag delete event.", "error", auditErr)
	}
}
