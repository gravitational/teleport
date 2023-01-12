package loginrulev1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/defaults"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the login rule gRPC service.
type ServiceConfig struct {
	Storage    *storage.S
	Authorizer auth.Authorizer
}

// Service implements the login rule gRPC service.
type Service struct {
	loginrulepb.UnimplementedLoginRuleServiceServer

	storage    *storage.S
	authorizer auth.Authorizer
}

// NewService returns a new login rule gRPC service.
func NewService(cfg *ServiceConfig) *Service {
	return &Service{
		storage:    cfg.Storage,
		authorizer: cfg.Authorizer,
	}
}

// CreateLoginRule creates a login rule if one with the same name does not
// already exist, else it returns an error.
func (s *Service) CreateLoginRule(ctx context.Context, req *loginrulepb.CreateLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): add audit event

	rule, err := s.storage.CreateLoginRule(ctx, req.LoginRule)
	return rule, trace.Wrap(err)
}

// UpsertLoginRule creates a login rule if one with the same name does not
// already exist, else it replaces the existing login rule.
func (s *Service) UpsertLoginRule(ctx context.Context, req *loginrulepb.UpsertLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): add audit event

	rule, err := s.storage.UpsertLoginRule(ctx, req.LoginRule)
	return rule, trace.Wrap(err)
}

// GetLoginRule retrieves a login rule described by the given request.
func (s *Service) GetLoginRule(ctx context.Context, req *loginrulepb.GetLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): add audit event

	rule, err := s.storage.GetLoginRule(ctx, req.Name)
	return rule, trace.Wrap(err)
}

// ListLoginRules lists all login rules.
func (s *Service) ListLoginRules(ctx context.Context, req *loginrulepb.ListLoginRulesRequest) (*loginrulepb.ListLoginRulesResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen): add audit event

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

	// TODO(nklaassen): add audit event

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
