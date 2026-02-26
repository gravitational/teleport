package webhookv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	webhookv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/webhook/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the Webhook gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.Webhooks
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

// Service implements the teleport.webhook.v1.WebhookService gRPC service.
type Service struct {
	webhookv1pb.UnimplementedWebhookServiceServer
	authorizer authz.Authorizer
	backend    services.Webhooks
	reader     Reader
	emitter    apievents.Emitter
	hooks      *Hooks
	// TODO: add resource-specific fields
}

// NewService returns a new Webhook gRPC service.
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
	if err := authCtx.CheckAccessToKind(types.KindWebhook, verb, verbs...); err != nil {
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

func (s *Service) emitCreateAuditEvent(ctx context.Context, webhook *webhookv1pb.Webhook, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.WebhookCreate{
		Metadata: apievents.Metadata{
			Type: libevents.WebhookCreateEvent,
			Code: libevents.WebhookCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      webhook.GetMetadata().GetName(),
			Expires:   getExpires(webhook.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		// FIXME: add resource-specific event fields here.
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit webhook create event.", "error", auditErr)
	}
}

func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *webhookv1pb.Webhook, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.WebhookUpdate{
		Metadata: apievents.Metadata{
			Type: libevents.WebhookUpdateEvent,
			Code: libevents.WebhookUpdateCode,
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
		slog.WarnContext(ctx, "Failed to emit webhook update event.", "error", auditErr)
	}
}

func (s *Service) emitUpsertAuditEvent(ctx context.Context, old, new *webhookv1pb.Webhook, authCtx *authz.Context, err error) {
	if old == nil {
		s.emitCreateAuditEvent(ctx, new, authCtx, err)
		return
	}
	s.emitUpdateAuditEvent(ctx, old, new, authCtx, err)
}

func (s *Service) emitDeleteAuditEvent(ctx context.Context, name string, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.WebhookDelete{
		Metadata: apievents.Metadata{
			Type: libevents.WebhookDeleteEvent,
			Code: libevents.WebhookDeleteCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      name,
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit webhook delete event.", "error", auditErr)
	}
}

// DefaultHooks returns the standard lifecycle hooks for webhook resources.
// Customize this to add logging, metrics, or side effects on mutations.
func DefaultHooks() *Hooks {
	return &Hooks{}
}
