// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

func TestScopedAccessCheckerContextRiskyAuthorizeUnpinnedRead(t *testing.T) {
	ctx := t.Context()
	checkerContext, err := NewScopedAccessCheckerContext(ctx, &AccessInfo{
		Username: "alice",
		ScopePin: scopesv1.Pin_builder{
			Kind:  scopesv1.PinKind_PIN_KIND_USER,
			Scope: "/test/scope",
		}.Build(),
	}, "test-cluster", emptyScopedRoleReader{})
	require.NoError(t, err)

	ruleCtx := &Context{}

	// A normal decision for a root-scoped resource is denied because the identity
	// is pinned away from root before any checker, including the default implicit
	// role checker, is evaluated.
	err = checkerContext.Decision(ctx, scopes.Root, func(checker *ScopedAccessChecker) error {
		return checker.CheckAccessToRules(ruleCtx, types.KindCertAuthority, types.VerbReadNoSecrets)
	})
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// RiskyAuthorizeUnpinnedRead bypasses pin enforcement but still requires the
	// underlying RBAC permission. The default implicit role grants CA
	// read_no_secrets, so this succeeds.
	err = checkerContext.RiskyAuthorizeUnpinnedRead(ctx, UnpinnedReadCertAuthority, ruleCtx)
	require.NoError(t, err)

	// Using an empty UnpinnedReadAuthorization is not allowed.
	err = checkerContext.RiskyAuthorizeUnpinnedRead(ctx, UnpinnedReadAuthorization{}, ruleCtx)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
}

// TestRiskyAuthorizeUnpinnedReadWithScope verifies that the per-call resourceScope
// overrides the authorization's scope and is used to determine the resulting access decision.
func TestRiskyAuthorizeUnpinnedReadWithScope(t *testing.T) {
	t.Parallel()

	const pinScope = "/test/scope"
	pin := scopesv1.Pin_builder{
		Kind:  scopesv1.PinKind_PIN_KIND_AGENT,
		Scope: pinScope,
		SystemRoles: scopesv1.SystemRoles_builder{
			Primary: types.RoleNode.String(),
		}.Build(),
	}.Build()
	checkerCtx := newAgentPinCheckerContext(t, pin)

	tests := []struct {
		name          string
		resourceScope string
		wantErr       string
	}{
		{
			name:          "override to pin scope is allowed",
			resourceScope: pinScope,
		},
		{
			name:          "override to descendant scope is allowed",
			resourceScope: pinScope + "/child",
		},
		{
			name:          "override to ancestor (root) scope is allowed",
			resourceScope: scopes.Root,
		},
		{
			name:          "override to orthogonal scope is denied",
			resourceScope: "/other",
			wantErr:       "scope pin \"/test/scope\" is orthogonal to resource scope \"/other\"",
		},
		{
			name:          "override to empty scope is rejected",
			resourceScope: "",
			wantErr:       "scope is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkerCtx.RiskyAuthorizeUnpinnedReadWithScope(t.Context(), UnpinnedReadScopedRole, &Context{}, tt.resourceScope)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

type emptyScopedRoleReader struct{}

func (emptyScopedRoleReader) GetScopedRole(context.Context, *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	return nil, trace.NotFound("scoped role not found")
}

func (emptyScopedRoleReader) ListScopedRoles(context.Context, *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	return &scopedaccessv1.ListScopedRolesResponse{}, nil
}

// newAgentPinCheckerContext is a test helper that builds a ScopedAccessCheckerContext for an agent pin
// using an allow-all role set for the given system role.
func newAgentPinCheckerContext(t *testing.T, pin *scopesv1.Pin) *ScopedAccessCheckerContext {
	t.Helper()
	roleName := pin.GetSystemRoles().GetPrimary()
	roleSet, err := RoleSetFromSpec("test-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{types.NewRule(types.Wildcard, RW())},
		},
	})
	require.NoError(t, err)
	checker := NewAccessCheckerWithRoleSet(&AccessInfo{}, "local-cluster", roleSet)
	checkersByRole := map[string]*ScopedAccessChecker{
		roleName: NewScopedAccessCheckerForSystemRole(roleName, checker),
	}
	ctx, err := NewScopedAccessCheckerContextForAgentPin(pin, checkersByRole)
	require.NoError(t, err)
	return ctx
}

// newAgentPin is a test helper that builds a [*scopesv1.Pin] for an agent.
func newAgentPin(scope string, role types.SystemRole) *scopesv1.Pin {
	return scopesv1.Pin_builder{
		Kind:  scopesv1.PinKind_PIN_KIND_AGENT,
		Scope: scope,
		SystemRoles: scopesv1.SystemRoles_builder{
			Primary: role.String(),
		}.Build(),
	}.Build()
}

// TestScopedAccessCheckerContextAgentPin covers the agent-pin mode of ScopedAccessCheckerContext.
func TestScopedAccessCheckerContextAgentPin(t *testing.T) {
	t.Parallel()

	const pinScope = "/test/scope"

	t.Run("constructor rejects nil pin", func(t *testing.T) {
		t.Parallel()
		roleSet, err := RoleSetFromSpec("test-role", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{types.NewRule(types.Wildcard, RW())},
			},
		})
		require.NoError(t, err)
		checker := NewAccessCheckerWithRoleSet(&AccessInfo{}, "local-cluster", roleSet)
		scopedChecker := NewScopedAccessCheckerForSystemRole(types.RoleNode.String(), checker)

		_, err = NewScopedAccessCheckerContextForAgentPin(nil, map[string]*ScopedAccessChecker{types.RoleNode.String(): scopedChecker})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("constructor rejects empty checkers", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)

		_, err := NewScopedAccessCheckerContextForAgentPin(pin, nil)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("successful construction and accessors", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		// ScopePin returns the pin for agent pin identities.
		gotPin, ok := checkerCtx.ScopePin()
		require.True(t, ok, "ScopePin should return true for agent pin context")
		require.Equal(t, pin, gotPin)

		// Traits returns nil for agent pin identities.
		require.Nil(t, checkerCtx.Traits(), "Traits should be nil for agent pin context")
	})

	t.Run("Decision at pin scope allows access", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		called := false
		err := checkerCtx.Decision(t.Context(), pinScope, func(c *ScopedAccessChecker) error {
			called = true
			return c.CheckAccessToRules(&Context{}, types.KindNode, types.VerbRead)
		})
		require.NoError(t, err, "Decision at pin scope should succeed")
		require.True(t, called, "fn should have been called")
	})

	t.Run("Decision at child of pin scope allows access", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		childScope := pinScope + "/child"
		called := false
		err := checkerCtx.Decision(t.Context(), childScope, func(c *ScopedAccessChecker) error {
			called = true
			return c.CheckAccessToRules(&Context{}, types.KindNode, types.VerbRead)
		})
		require.NoError(t, err, "Decision at child scope should succeed")
		require.True(t, called, "fn should have been called for child scope")
	})

	t.Run("Decision at root scope is denied when pin is non-root", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		called := false
		err := checkerCtx.Decision(t.Context(), "/", func(c *ScopedAccessChecker) error {
			called = true
			return nil
		})
		require.Error(t, err, "Decision at root scope should be denied when pin is non-root")
		require.True(t, trace.IsAccessDenied(err))
		require.False(t, called, "fn should NOT have been called when pin excludes the scope")
	})

	t.Run("RiskyAuthorizeUnpinnedRead bypasses pin enforcement", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		err := checkerCtx.RiskyAuthorizeUnpinnedRead(t.Context(), UnpinnedReadAuthorization{
			resourceScope: scopes.Root,
			kind:          types.KindNode,
			verbs:         []string{types.VerbRead},
		}, &Context{})
		require.NoError(t, err, "RiskyAuthorizeUnpinnedRead should bypass pin enforcement and succeed")
	})

	t.Run("RiskyAuthorizeUnpinnedEmitEvent bypasses pin enforcement", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		// A normal decision for a root-scoped resource is denied because the identity
		// is pinned away from root before any checker, including the default implicit
		// role checker, is evaluated.
		err := checkerCtx.Decision(t.Context(), scopes.Root, func(checker *ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&Context{}, types.KindEvent, types.VerbCreate)
		})
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))

		err = checkerCtx.RiskyAuthorizeUnpinnedEmitEvent(t.Context(), &Context{})
		require.NoError(t, err, "RiskyAuthorizeUnpinnedEmitEvent should bypass pin enforcement and succeed")
	})
	t.Run("RiskyAuthorizeUnpinnedWriteEvent bypasses pin enforcement", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		// A normal decision for a root-scoped resource is denied because the identity
		// is pinned away from root before any checker, including the default implicit
		// role checker, is evaluated.
		err := checkerCtx.Decision(t.Context(), scopes.Root, func(checker *ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&Context{}, types.KindEvent, types.VerbCreate, types.VerbUpdate)
		})
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))

		err = checkerCtx.RiskyAuthorizeUnpinnedWriteEvent(t.Context(), &Context{})
		require.NoError(t, err, "RiskyAuthorizeUnpinnedWriteEvent should bypass pin enforcement and succeed")
	})

	t.Run("riskyEnumerateScopedCheckers panics for agent pin context", func(t *testing.T) {
		t.Parallel()
		pin := newAgentPin(pinScope, types.RoleNode)
		checkerCtx := newAgentPinCheckerContext(t, pin)

		require.Panics(t, func() {
			// Drain the stream to trigger the panic.
			for range checkerCtx.riskyEnumerateScopedCheckers(t.Context()) {
			}
		}, "riskyEnumerateScopedCheckers should panic for agent pin contexts")
	})
}

func TestScopedAccessCheckerContextUnpinnedUserEventWrites(t *testing.T) {
	ctx := t.Context()
	userCheckerContext, err := NewScopedAccessCheckerContext(ctx, &AccessInfo{
		Username: "alice",
		ScopePin: scopesv1.Pin_builder{
			Kind:  scopesv1.PinKind_PIN_KIND_USER,
			Scope: "/test/scope",
		}.Build(),
	}, "test-cluster", emptyScopedRoleReader{})
	require.NoError(t, err)

	ruleCtx := &Context{}

	// A normal decision for a root-scoped resource is denied because the identity
	// is pinned away from root before any checker, including the default implicit
	// role checker, is evaluated.
	err = userCheckerContext.Decision(ctx, scopes.Root, func(checker *ScopedAccessChecker) error {
		return checker.CheckAccessToRules(ruleCtx, types.KindEvent, types.VerbCreate)
	})
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// RiskyAuthorizeUnpinnedEmitEvent bypasses pin enforcement but should explicitly fail for scoped
	// user pins.
	err = userCheckerContext.RiskyAuthorizeUnpinnedEmitEvent(ctx, ruleCtx)
	require.ErrorContains(t, err, "unpinned authorization for audit event emission is only supported for agent pins")

	// RiskyAuthorizeUnpinneWriteEvent bypasses pin enforcement but should explicitly fail for scoped
	// user pins.
	err = userCheckerContext.RiskyAuthorizeUnpinnedWriteEvent(ctx, ruleCtx)
	require.ErrorContains(t, err, "unpinned authorization for audit event emission is only supported for agent pins")
}
