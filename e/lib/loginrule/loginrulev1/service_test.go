package loginrulev1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

type testPack struct {
	clock clockwork.FakeClock
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

	s := storage.New(func() backend.Backend { return mem })

	return &testPack{
		clock: clock,
		mem:   mem,
		s:     s,
	}
}

type fakeAuthorizer struct {
	checker *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*auth.Context, error) {
	return &auth.Context{
		Checker: f.checker,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	allow  map[check]bool
	checks []check
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string, silent bool) error {
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

	cfg := &ServiceConfig{
		Storage:    p.s,
		Authorizer: authorizer,
	}

	service := NewService(cfg)

	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: "test_rule",
		},
		TraitsExpression: "test",
	}

	for _, tc := range []struct {
		desc         string
		f            func() error
		allow        map[check]bool
		expectChecks []check
	}{
		{
			desc: "create",
			f: func() error {
				_, err := service.CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
					LoginRule: rule,
				})
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbCreate}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbCreate},
			},
		},
		{
			desc: "upsert",
			f: func() error {
				_, err := service.UpsertLoginRule(ctx, &loginrulepb.UpsertLoginRuleRequest{
					LoginRule: rule,
				})
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
		},
		{
			desc: "get",
			f: func() error {
				_, err := service.GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
					Name: rule.Metadata.Name,
				})
				return err
			},
			allow: map[check]bool{
				{types.KindLoginRule, types.VerbRead}: true,
			},
			expectChecks: []check{
				{types.KindLoginRule, types.VerbRead},
			},
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

			// TODO(nklaassen): check for audit events.
		})
	}
}
