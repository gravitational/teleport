package loginrulev1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type testPack struct {
	clock clocki.FakeClock
	mem   *memory.Memory
	s     *storage.S
}

func newTestPack(t *testing.T) *testPack {
	t.Helper()

	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	s := storage.New(mem)

	return &testPack{
		clock: clock,
		mem:   mem,
		s:     s,
	}
}

type fakeAuthorizer struct {
	checker *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker:              f.checker,
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	allow  map[check]bool
	checks []check
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	c := check{rule, verb}
	f.checks = append(f.checks, c)
	if f.allow[c] {
		return nil
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", rule, verb)
}

type check struct {
	rule, verb string
}

func TestRBAC(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{}

	mockEmitter := &eventstest.MockRecorderEmitter{}

	cfg := &ServiceConfig{
		Storage:    p.s,
		Authorizer: authorizer,
		Emitter:    mockEmitter,
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	rule := loginrulepb.LoginRule_builder{
		Metadata: &types.Metadata{
			Name: "test_rule",
		},
		Version:          types.V1,
		TraitsExpression: "external",
	}.Build()

	for _, tc := range []struct {
		desc         string
		f            func() error
		allow        map[check]bool
		expectChecks []check
		expectEvents []apievents.AuditEvent
	}{
		{
			desc: "create",
			f: func() error {
				_, err := service.CreateLoginRule(ctx, loginrulepb.CreateLoginRuleRequest_builder{
					LoginRule: rule,
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbCreate}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbCreate},
			},
			expectEvents: []apievents.AuditEvent{
				&apievents.LoginRuleCreate{
					Metadata: apievents.Metadata{
						Type: events.LoginRuleCreateEvent,
						Code: events.LoginRuleCreateCode,
					},
					ResourceMetadata: apievents.ResourceMetadata{
						Name: rule.GetMetadata().Name,
					},
					UserMetadata: authz.ClientUserMetadata(ctx),
				},
			},
		},
		{
			desc: "upsert",
			f: func() error {
				_, err := service.UpsertLoginRule(ctx, loginrulepb.UpsertLoginRuleRequest_builder{
					LoginRule: rule,
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbCreate}: true,
				{types.KindLoginRule, types.VerbUpdate}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbCreate},
				{types.KindLoginRule, types.VerbUpdate},
			},
			expectEvents: []apievents.AuditEvent{
				&apievents.LoginRuleCreate{
					Metadata: apievents.Metadata{
						Type: events.LoginRuleCreateEvent,
						Code: events.LoginRuleCreateCode,
					},
					ResourceMetadata: apievents.ResourceMetadata{
						Name: rule.GetMetadata().Name,
					},
					UserMetadata: authz.ClientUserMetadata(ctx),
				},
			},
		},
		{
			desc: "get",
			f: func() error {
				_, err := service.GetLoginRule(ctx, loginrulepb.GetLoginRuleRequest_builder{
					Name: rule.GetMetadata().Name,
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbRead}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbRead},
			},
			expectEvents: []apievents.AuditEvent{},
		},
		{
			desc: "list",
			f: func() error {
				_, err := service.ListLoginRules(ctx, &loginrulepb.ListLoginRulesRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbRead}: true,
				{types.KindLoginRule, types.VerbList}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbRead},
				{types.KindLoginRule, types.VerbList},
			},
			expectEvents: []apievents.AuditEvent{},
		},
		{
			desc: "delete",
			f: func() error {
				_, err := service.DeleteLoginRule(ctx, loginrulepb.DeleteLoginRuleRequest_builder{
					Name: rule.GetMetadata().Name,
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbDelete}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbDelete},
			},
			expectEvents: []apievents.AuditEvent{
				&apievents.LoginRuleDelete{
					Metadata: apievents.Metadata{
						Type: events.LoginRuleDeleteEvent,
						Code: events.LoginRuleDeleteCode,
					},
					ResourceMetadata: apievents.ResourceMetadata{
						Name: rule.GetMetadata().Name,
					},
					UserMetadata: authz.ClientUserMetadata(ctx),
				},
			},
		},
		{
			desc: "test",
			f: func() error {
				_, err := service.TestLoginRule(ctx, loginrulepb.TestLoginRuleRequest_builder{Traits: map[string]*wrappers.StringValues{"test": {Values: []string{"test"}}}}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbRead}: true,
				{types.KindLoginRule, types.VerbList}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbRead},
				{types.KindLoginRule, types.VerbList},
			},
			expectEvents: []apievents.AuditEvent{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// First check with nothing allowed.
			authorizer.checker = &fakeChecker{}
			err := tc.f()
			require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)

			// Check with allowed rule/verbs from testcase.
			authorizer.checker = &fakeChecker{
				allow: tc.allow,
			}
			err = tc.f()
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectChecks, authorizer.checker.checks)

			require.Equal(t, tc.expectEvents, mockEmitter.Events())
			mockEmitter.Reset()
		})
	}
}
