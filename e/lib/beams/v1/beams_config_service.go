package beamsv1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// BeamsConfigServiceConfig holds configuration for the BeamsConfigService gRPC service.
type BeamsConfigServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      services.BeamsConfigGetter
	Backend    services.BeamsConfigService
	Emitter    apievents.Emitter
	Logger     *slog.Logger
}

// BeamsConfigService implements the gRPC API for the BeamsConfig singleton resource.
type BeamsConfigService struct {
	beamsv1pb.UnimplementedBeamsConfigServiceServer

	authorizer authz.Authorizer
	cache      services.BeamsConfigGetter
	backend    services.BeamsConfigService
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewBeamsConfigService creates a new BeamsConfigService gRPC service.
func NewBeamsConfigService(cfg BeamsConfigServiceConfig) (*BeamsConfigService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("cache is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}
	return &BeamsConfigService{
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		emitter:    cfg.Emitter,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// GetBeamsConfig returns the singleton BeamsConfig resource.
func (s *BeamsConfigService) GetBeamsConfig(ctx context.Context, _ *beamsv1pb.GetBeamsConfigRequest) (*beamsv1pb.GetBeamsConfigResponse, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.cache.GetBeamsConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1pb.GetBeamsConfigResponse_builder{
		BeamsConfig: config,
	}.Build(), nil
}

// CreateBeamsConfig creates a new BeamsConfig resource.
func (s *BeamsConfigService) CreateBeamsConfig(ctx context.Context, req *beamsv1pb.CreateBeamsConfigRequest) (*beamsv1pb.CreateBeamsConfigResponse, error) {
	if err := s.authorizeWrite(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.CreateBeamsConfig(ctx, req.GetBeamsConfig())
	s.emitAuditEvent(ctx, newBeamsConfigCreateAuditEvent(ctx, eventStatusFromError(err)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1pb.CreateBeamsConfigResponse_builder{
		BeamsConfig: config,
	}.Build(), nil
}

// UpdateBeamsConfig updates an existing BeamsConfig resource.
func (s *BeamsConfigService) UpdateBeamsConfig(ctx context.Context, req *beamsv1pb.UpdateBeamsConfigRequest) (*beamsv1pb.UpdateBeamsConfigResponse, error) {
	if err := s.authorizeWrite(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.UpdateBeamsConfig(ctx, req.GetBeamsConfig())
	s.emitAuditEvent(ctx, newBeamsConfigUpdateAuditEvent(ctx, eventStatusFromError(err)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1pb.UpdateBeamsConfigResponse_builder{
		BeamsConfig: config,
	}.Build(), nil
}

// DeleteBeamsConfig deletes the BeamsConfig singleton.
func (s *BeamsConfigService) DeleteBeamsConfig(ctx context.Context, _ *beamsv1pb.DeleteBeamsConfigRequest) (*beamsv1pb.DeleteBeamsConfigResponse, error) {
	if err := s.authorizeWrite(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err := s.backend.DeleteBeamsConfig(ctx)
	s.emitAuditEvent(ctx, newBeamsConfigDeleteAuditEvent(ctx, eventStatusFromError(err)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1pb.DeleteBeamsConfigResponse_builder{}.Build(), nil
}

func (s *BeamsConfigService) authorizeRead(ctx context.Context) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(authCtx.CheckAccessToKind(types.KindBeamsConfig, types.VerbRead))
}

func (s *BeamsConfigService) authorizeWrite(ctx context.Context, verb string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindBeamsConfig, verb); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(authCtx.AuthorizeAdminActionAllowReusedMFA())
}

func (s *BeamsConfigService) emitAuditEvent(ctx context.Context, evt apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(ctx, "Failed to emit audit event",
			"event_code", evt.GetCode(),
			"event_type", evt.GetType(),
			"error", err,
		)
	}
}

func newBeamsConfigCreateAuditEvent(ctx context.Context, status apievents.Status) apievents.AuditEvent {
	return &apievents.BeamsConfigCreate{
		Metadata: apievents.Metadata{
			Code: libevents.BeamsConfigCreateCode,
			Type: libevents.BeamsConfigCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             status,
	}
}

func newBeamsConfigUpdateAuditEvent(ctx context.Context, status apievents.Status) apievents.AuditEvent {
	return &apievents.BeamsConfigUpdate{
		Metadata: apievents.Metadata{
			Code: libevents.BeamsConfigUpdateCode,
			Type: libevents.BeamsConfigUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             status,
	}
}

func newBeamsConfigDeleteAuditEvent(ctx context.Context, status apievents.Status) apievents.AuditEvent {
	return &apievents.BeamsConfigDelete{
		Metadata: apievents.Metadata{
			Code: libevents.BeamsConfigDeleteCode,
			Type: libevents.BeamsConfigDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             status,
	}
}

func eventStatusFromError(err error) apievents.Status {
	if err == nil {
		return apievents.Status{
			Success: true,
		}
	}
	return apievents.Status{
		Error:       err.Error(),
		UserMessage: trace.UserMessage(err),
	}
}
