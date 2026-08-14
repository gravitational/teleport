package loginrulev1

import (
	"context"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	oss "github.com/gravitational/teleport/lib/loginrule"
)

// ServiceConfig holds configuration options for the login rule gRPC service.
type ServiceConfig struct {
	Storage    *storage.S
	Authorizer authz.Authorizer
	Emitter    apievents.Emitter
}

// Service implements the login rule gRPC service.
type Service struct {
	loginrulepb.UnimplementedLoginRuleServiceServer

	logger     *slog.Logger
	storage    *storage.S
	authorizer authz.Authorizer
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
		logger:     slog.With(teleport.ComponentKey, teleport.Component("loginrule", "service")),
		storage:    cfg.Storage,
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
	}, nil
}

// CreateLoginRule creates a login rule if one with the same name does not
// already exist, else it returns an error.
func (s *Service) CreateLoginRule(ctx context.Context, req *loginrulepb.CreateLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLoginRule, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := loginrule.Validate(req.GetLoginRule()); err != nil {
		return nil, trace.Wrap(err, "failed to validate login rule")
	}

	if err := s.emitCreateEvent(ctx, req.GetLoginRule()); err != nil {
		return nil, trace.Wrap(err)
	}

	rule, err := s.storage.CreateLoginRule(ctx, req.GetLoginRule())
	return rule, trace.Wrap(err)
}

// UpsertLoginRule creates a login rule if one with the same name does not
// already exist, else it replaces the existing login rule.
func (s *Service) UpsertLoginRule(ctx context.Context, req *loginrulepb.UpsertLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLoginRule, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := loginrule.Validate(req.GetLoginRule()); err != nil {
		return nil, trace.Wrap(err, "failed to validate login rule")
	}

	if err := s.emitCreateEvent(ctx, req.GetLoginRule()); err != nil {
		return nil, trace.Wrap(err)
	}

	rule, err := s.storage.UpsertLoginRule(ctx, req.GetLoginRule())
	return rule, trace.Wrap(err)
}

// GetLoginRule retrieves a login rule described by the given request.
func (s *Service) GetLoginRule(ctx context.Context, req *loginrulepb.GetLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLoginRule, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rule, err := s.storage.GetLoginRule(ctx, req.GetName())
	return rule, trace.Wrap(err)
}

// ListLoginRules lists all login rules.
func (s *Service) ListLoginRules(ctx context.Context, req *loginrulepb.ListLoginRulesRequest) (*loginrulepb.ListLoginRulesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLoginRule, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rules, nextPageToken, err := s.storage.ListLoginRules(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return loginrulepb.ListLoginRulesResponse_builder{
		LoginRules:    rules,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

// DeleteLoginRule deletes an existing login rule.
func (s *Service) DeleteLoginRule(ctx context.Context, req *loginrulepb.DeleteLoginRuleRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLoginRule, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitDeleteEvent(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.storage.DeleteLoginRule(ctx, req.GetName())
	return &emptypb.Empty{}, trace.Wrap(err)
}

// TestLoginRule evaluates login rules against provided user traits
// to test that the output matches expectations prior to them being enforced and
// potentially locking out users.
func (s *Service) TestLoginRule(ctx context.Context, req *loginrulepb.TestLoginRuleRequest) (*loginrulepb.TestLoginRuleResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLoginRule, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.GetTraits()) == 0 {
		return nil, trace.BadParameter("at least one trait must be provided")
	}

	rules := req.GetLoginRules()
	if req.GetLoadFromCluster() {
		for next := ""; ; {
			retrieved, token, err := s.storage.ListLoginRules(ctx, 100, next)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			next = token
			rules = append(rules, retrieved...)

			if token == "" {
				break
			}
		}
	}

	traits := make(map[string][]string, len(req.GetTraits()))
	for key, values := range req.GetTraits() {
		traits[key] = values.Values
	}

	output, err := loginrule.Evaluate(rules, &oss.EvaluationInput{Traits: traits})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make(map[string]*wrappers.StringValues, len(output.Traits))
	for key, values := range output.Traits {
		out[key] = &wrappers.StringValues{
			Values: slices.Clone(values),
		}
	}

	return loginrulepb.TestLoginRuleResponse_builder{Traits: out}.Build(), nil
}

func (s *Service) emitCreateEvent(ctx context.Context, rule *loginrulepb.LoginRule) error {
	e := &apievents.LoginRuleCreate{
		Metadata: apievents.Metadata{
			Type: events.LoginRuleCreateEvent,
			Code: events.LoginRuleCreateCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: rule.GetMetadata().Name,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if expires := rule.GetMetadata().Expires; expires != nil {
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
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	return trace.Wrap(s.emitAuditEvent(ctx, e))
}

func (s *Service) emitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	err := s.emitter.EmitAuditEvent(ctx, e)
	if err != nil {
		userMeta := authz.ClientUserMetadata(ctx)
		s.logger.WarnContext(ctx, "Failed to emit audit event",
			"type", e.GetType(),
			"code", e.GetCode(),
			"user", userMeta.User,
			"impersonator", userMeta.Impersonator,
			"error", err,
		)
	}
	return trace.Wrap(err)
}
