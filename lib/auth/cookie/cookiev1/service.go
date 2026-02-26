package cookiev1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	cookiev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/cookie/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the Cookie gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.Cookies
	Reader     Reader
	Emitter    apievents.Emitter
	Hooks      *Hooks
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

// Service implements the teleport.cookie.v1.CookieService gRPC service.
type Service struct {
	cookiev1pb.UnimplementedCookieServiceServer
	authorizer authz.Authorizer
	backend    services.Cookies
	reader     Reader
	emitter    apievents.Emitter
	hooks      *Hooks
	// TODO: add resource-specific fields
}

// NewService returns a new Cookie gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		reader:     cfg.Reader,
		emitter:    cfg.Emitter,
		hooks:      cfg.Hooks,
	}, nil
}

// authorize checks that the caller has the given verbs on the resource kind.
func (s *Service) authorize(ctx context.Context, verb string, verbs ...string) (*authz.Context, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindCookie, verb, verbs...); err != nil {
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

func (s *Service) emitCreateAuditEvent(ctx context.Context, cookie *cookiev1pb.Cookie, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.CookieCreate{
		Metadata: apievents.Metadata{
			Type: libevents.CookieCreateEvent,
			Code: libevents.CookieCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      cookie.GetMetadata().GetName(),
			Expires:   getExpires(cookie.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		// FIXME: add resource-specific event fields here.
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit cookie create event.", "error", auditErr)
	}
}

func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *cookiev1pb.Cookie, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.CookieUpdate{
		Metadata: apievents.Metadata{
			Type: libevents.CookieUpdateEvent,
			Code: libevents.CookieUpdateCode,
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
		slog.WarnContext(ctx, "Failed to emit cookie update event.", "error", auditErr)
	}
}

func (s *Service) emitUpsertAuditEvent(ctx context.Context, old, new *cookiev1pb.Cookie, authCtx *authz.Context, err error) {
	if old == nil {
		s.emitCreateAuditEvent(ctx, new, authCtx, err)
		return
	}
	s.emitUpdateAuditEvent(ctx, old, new, authCtx, err)
}

func (s *Service) emitDeleteAuditEvent(ctx context.Context, name string, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.CookieDelete{
		Metadata: apievents.Metadata{
			Type: libevents.CookieDeleteEvent,
			Code: libevents.CookieDeleteCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      name,
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit cookie delete event.", "error", auditErr)
	}
}

// DefaultHooks returns the standard lifecycle hooks for cookie resources.
// Customize this to add logging, metrics, or side effects on mutations.
func DefaultHooks() *Hooks {
	return &Hooks{}
}
