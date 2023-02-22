package loginrulev1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/defaults"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the login rule gRPC service.
type ServiceConfig struct {
	Storage    *storage.S
	Authorizer auth.Authorizer
	Emitter    apievents.Emitter
}

// Service implements the login rule gRPC service.
type Service struct {
	loginrulepb.UnimplementedLoginRuleServiceServer

	logger *logrus.Entry

	storage    *storage.S
	authorizer auth.Authorizer
	emitter    apievents.Emitter
}

// NewService returns a new login rule gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Storage == nil:
		return nil, trace.BadParameter("storage is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}
	return &Service{
		logger:     logrus.WithField(trace.Component, "loginrule.service"),
		storage:    cfg.Storage,
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
	}, nil
}

// CreateLoginRule creates a login rule if one with the same name does not
// already exist, else it returns an error.
func (s *Service) CreateLoginRule(ctx context.Context, req *loginrulepb.CreateLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := loginrule.Validate(req.LoginRule); err != nil {
		return nil, trace.Wrap(err, "failed to validate login rule")
	}

	if err := s.emitCreateEvent(ctx, req.LoginRule); err != nil {
		return nil, trace.Wrap(err)
	}

	rule, err := s.storage.CreateLoginRule(ctx, req.LoginRule)
	return rule, trace.Wrap(err)
}

// UpsertLoginRule creates a login rule if one with the same name does not
// already exist, else it replaces the existing login rule.
func (s *Service) UpsertLoginRule(ctx context.Context, req *loginrulepb.UpsertLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := loginrule.Validate(req.LoginRule); err != nil {
		return nil, trace.Wrap(err, "failed to validate login rule")
	}

	if err := s.emitCreateEvent(ctx, req.LoginRule); err != nil {
		return nil, trace.Wrap(err)
	}

	rule, err := s.storage.UpsertLoginRule(ctx, req.LoginRule)
	return rule, trace.Wrap(err)
}

// GetLoginRule retrieves a login rule described by the given request.
func (s *Service) GetLoginRule(ctx context.Context, req *loginrulepb.GetLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rule, err := s.storage.GetLoginRule(ctx, req.Name)
	return rule, trace.Wrap(err)
}

// ListLoginRules lists all login rules.
func (s *Service) ListLoginRules(ctx context.Context, req *loginrulepb.ListLoginRulesRequest) (*loginrulepb.ListLoginRulesResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rules, nextPageToken, err := s.storage.ListLoginRules(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &loginrulepb.ListLoginRulesResponse{
		LoginRules:    rules,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteLoginRule deletes an existing login rule.
func (s *Service) DeleteLoginRule(ctx context.Context, req *loginrulepb.DeleteLoginRuleRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitDeleteEvent(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	err := s.storage.DeleteLoginRule(ctx, req.Name)
	return &emptypb.Empty{}, trace.Wrap(err)
}

func (s *Service) authorizeVerbs(ctx context.Context, verbs ...string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}
	errs := make([]error, len(verbs))
	for i, verb := range verbs {
		errs[i] = authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, types.KindLoginRule, verb, false /* silent */)
	}
	// Convert generic aggregate error to AccessDenied (auth_with_roles also does this).
	if err := trace.NewAggregate(errs...); err != nil {
		return trace.AccessDenied(err.Error())
	}
	return nil
}

func (s *Service) emitCreateEvent(ctx context.Context, rule *loginrulepb.LoginRule) error {
	e := &apievents.LoginRuleCreate{
		Metadata: apievents.Metadata{
			Type: events.LoginRuleCreateEvent,
			Code: events.LoginRuleCreateCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: rule.Metadata.Name,
		},
		UserMetadata: auth.ClientUserMetadata(ctx),
	}
	if expires := rule.Metadata.Expires; expires != nil {
		e.ResourceMetadata.Expires = *expires
	}
	return trace.Wrap(s.emitAuditEvent(ctx, e))
}

func (s *Service) emitDeleteEvent(ctx context.Context, name string) error {
	e := &apievents.LoginRuleDelete{
		Metadata: apievents.Metadata{
			Type: events.LoginRuleDeleteEvent,
			Code: events.LoginRuleDeleteCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
		UserMetadata: auth.ClientUserMetadata(ctx),
	}
	return trace.Wrap(s.emitAuditEvent(ctx, e))
}

func (s *Service) emitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	err := s.emitter.EmitAuditEvent(ctx, e)
	if err != nil {
		userMeta := auth.ClientUserMetadata(ctx)
		s.logger.WithError(err).WithFields(logrus.Fields{
			"type":         e.GetType(),
			"code":         e.GetCode(),
			"user":         userMeta.User,
			"impersonator": userMeta.Impersonator,
		}).Warn("Failed to emit audit event")
	}
	return trace.Wrap(err)
}
