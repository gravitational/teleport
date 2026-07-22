// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
)

// newScopedAgentServerWithRoles builds a ScopedServerWithRoles whose identity is a scoped agent pinned
// at the given scope with wildcard permissions. Used for exercising the scoped watch authz logic.
func newScopedAgentServerWithRoles(t *testing.T, pinScope string) *ScopedServerWithRoles {
	t.Helper()

	pin := scopesv1.Pin_builder{
		Kind:        scopesv1.PinKind_PIN_KIND_AGENT,
		Scope:       pinScope,
		SystemRoles: scopesv1.SystemRoles_builder{Primary: string(types.RoleNode)}.Build(),
	}.Build()

	roleSet, err := services.RoleSetFromSpec("agent-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{types.NewRule(types.Wildcard, []string{types.Wildcard})},
		},
	})
	require.NoError(t, err)

	checker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "test-cluster", roleSet)
	checkerCtx, err := services.NewScopedAccessCheckerContextForAgentPin(pin, map[string]*services.ScopedAccessChecker{
		string(types.RoleNode): services.NewScopedAccessCheckerForSystemRole(string(types.RoleNode), checker),
	})
	require.NoError(t, err)

	return &ScopedServerWithRoles{
		scopedContext: &authz.ScopedContext{
			User:           &types.UserV2{Metadata: types.Metadata{Name: "agent"}},
			Identity:       authz.ScopedBuiltinRole{ScopePin: pin},
			CheckerContext: checkerCtx,
		},
	}
}

// TestScopedWatchKindAuthz verifies the per-mode authorization, pin enforcement, and scoped kind whitelist
// applied to watch kinds requested by a scoped identity.
func TestScopedWatchKindAuthz(t *testing.T) {
	ctx := context.Background()
	const pinScope = "/foo"
	a := newScopedAgentServerWithRoles(t, pinScope)

	filter := func(mode scopesv1.Mode, scope string) *types.ScopeFilter {
		return types.ScopeFilterFromProto(scopesv1.Filter_builder{Mode: mode, Scope: scope}.Build())
	}

	tests := []struct {
		name        string
		kind        string
		filter      *types.ScopeFilter
		loadSecrets bool
		assertErr   require.ErrorAssertionFunc
		wantMode    scopesv1.Mode
		wantScope   string
	}{
		{
			name:      "scoped_role default resolves to EXACT at pin",
			kind:      scopedaccess.KindScopedRole,
			filter:    nil,
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_EXACT,
			wantScope: pinScope,
		},
		{
			name:      "scoped_role EXACT at pin",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, pinScope),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_EXACT,
			wantScope: pinScope,
		},
		{
			name:      "scoped_role EXACT at descendant of pin",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, "/foo/sub"),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_EXACT,
			wantScope: "/foo/sub",
		},
		{
			name:      "scoped_role DESCENDANTS at pin",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_DESCENDANTS, pinScope),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_DESCENDANTS,
			wantScope: pinScope,
		},
		{
			name:      "scoped_role ALL rewritten to DESCENDANTS at pin",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ALL, ""),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_DESCENDANTS,
			wantScope: pinScope,
		},
		{
			name:      "scoped_role EXACT at orthogonal scope denied by pin enforcement",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, "/bar"),
			assertErr: requireAccessDenied,
		},
		{
			name:      "scoped_role EXACT at ancestor of pin denied by pin enforcement",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, scopes.Root),
			assertErr: requireAccessDenied,
		},
		{
			name:      "scoped_role ANCESTORS at pin allowed for agent via ancestral exception",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ANCESTORS, pinScope),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_ANCESTORS,
			wantScope: pinScope,
		},
		{
			name:      "scoped_role RELATIVES at pin allowed for agent via ancestral exception",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_RELATIVES, pinScope),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_RELATIVES,
			wantScope: pinScope,
		},
		{
			name:      "scoped_role ANCESTORS at descendant of pin allowed for agent",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ANCESTORS, "/foo/sub"),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_ANCESTORS,
			wantScope: "/foo/sub",
		},
		{
			name:      "scoped_role ANCESTORS at orthogonal scope denied",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ANCESTORS, "/bar"),
			assertErr: requireAccessDenied,
		},
		{
			// the filter scope must be subject to the pin. while unpinned read exceptions permit reads
			// of parent scopes, a filter at a parent scope would also match orthogonal resources, which
			// unpinned read exceptions do not cover.
			name:      "scoped_role ANCESTORS at ancestor of pin denied",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ANCESTORS, scopes.Root),
			assertErr: requireAccessDenied,
		},
		{
			name:        "scoped_role ANCESTORS with secrets denied",
			kind:        scopedaccess.KindScopedRole,
			filter:      filter(scopesv1.Mode_MODE_ANCESTORS, pinScope),
			loadSecrets: true,
			assertErr:   requireAccessDenied,
		},
		{
			// the choice of scoped role assignment in this test-case is arbitrary, we're just varifying
			// that ancestral watch isn't permitted for a kind that we haven't written an exception for.
			name:      "scoped_role_assignment ANCESTORS denied",
			kind:      scopedaccess.KindScopedRoleAssignment,
			filter:    filter(scopesv1.Mode_MODE_ANCESTORS, pinScope),
			assertErr: requireAccessDenied,
		},
		{
			name:      "scoped_role UNSCOPED denied for scoped caller",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_UNSCOPED, ""),
			assertErr: requireAccessDenied,
		},
		{
			// scoped-kind-whitelist: this test case and associated behaviore can be removed once all scoped
			// kinds are namespaced.
			name:      "non-namespaced kind EXACT denied by whitelist",
			kind:      types.KindNode,
			filter:    filter(scopesv1.Mode_MODE_EXACT, pinScope),
			assertErr: requireAccessDenied,
		},
		{
			// scoped-kind-whitelist: this test case and associated behaviore can be removed once all scoped
			// kinds are namespaced.
			name:      "non-namespaced kind ALL denied by whitelist for scoped caller",
			kind:      types.KindNode,
			filter:    filter(scopesv1.Mode_MODE_ALL, ""),
			assertErr: requireAccessDenied,
		},
		{
			name:      "malformed filter (mode without scope) rejected",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, ""),
			assertErr: requireBadParameter,
		},
		{
			name:      "malformed filter (scope without mode) rejected",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_UNSPECIFIED, pinScope),
			assertErr: requireBadParameter,
		},
		{
			name:      "cert_authority UNSCOPED allowed via unscoped-kind exception",
			kind:      types.KindCertAuthority,
			filter:    filter(scopesv1.Mode_MODE_UNSCOPED, ""),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_UNSCOPED,
		},
		{
			name:      "spiffe_federation UNSCOPED allowed via unscoped-kind exception",
			kind:      types.KindSPIFFEFederation,
			filter:    filter(scopesv1.Mode_MODE_UNSCOPED, ""),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_UNSCOPED,
		},
		{
			name:        "cert_authority UNSCOPED with secrets denied",
			kind:        types.KindCertAuthority,
			filter:      filter(scopesv1.Mode_MODE_UNSCOPED, ""),
			loadSecrets: true,
			assertErr:   requireAccessDenied,
		},
		{
			// unscoped-kind exception translates missing filters to UNSCOPED even for scoped callers
			name:      "cert_authority default resolves to UNSCOPED via unscoped-kind exception",
			kind:      types.KindCertAuthority,
			filter:    nil,
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_UNSCOPED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.authorizeWatchKind(ctx, types.WatchKind{Kind: tt.kind, ScopeFilter: tt.filter, LoadSecrets: tt.loadSecrets})
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.wantMode, got.GetMode())
			require.Equal(t, tt.wantScope, got.GetScope())

			// authorizeWatchRequest writes the effective filter back onto the accepted kind.
			watch := types.Watch{Kinds: []types.WatchKind{{Kind: tt.kind, ScopeFilter: tt.filter, LoadSecrets: tt.loadSecrets}}}
			require.NoError(t, a.authorizeWatchRequest(ctx, &watch))
			require.Len(t, watch.Kinds, 1)
			require.Equal(t, tt.wantMode, watch.Kinds[0].ScopeFilter.ToProto().GetMode())
			require.Equal(t, tt.wantScope, watch.Kinds[0].ScopeFilter.ToProto().GetScope())
		})
	}
}

// TestAncestralWatchExceptionServiceSpecific verifies that the scoped-role ancestral watch exception is
// restricted to service identities. A user with equivalent permissions is denied.
func TestAncestralWatchExceptionServiceSpecific(t *testing.T) {
	ctx := context.Background()
	const pinScope = "/foo"
	a := newScopedAgentServerWithRoles(t, pinScope)

	// same checker context, but a non-service identity.
	userCtx := *a.scopedContext
	userCtx.Identity = authz.LocalUser{}
	u := &ScopedServerWithRoles{scopedContext: &userCtx}

	kind := types.WatchKind{
		Kind: scopedaccess.KindScopedRole,
		ScopeFilter: types.ScopeFilterFromProto(
			scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ANCESTORS, Scope: pinScope}.Build(),
		),
	}

	_, err := a.authorizeWatchKind(ctx, kind)
	require.NoError(t, err)

	_, err = u.authorizeWatchKind(ctx, kind)
	requireAccessDenied(t, err)
	require.ErrorContains(t, err, "non-service identities are not permitted to watch")
}

func requireAccessDenied(t require.TestingT, err error, _ ...any) {
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
}

func requireBadParameter(t require.TestingT, err error, _ ...any) {
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
}

// newUnscopedServerWithRoles builds a ScopedServerWithRoles whose identity is a classical unscoped
// caller with wildcard permissions. Used for exercising the unscoped side of the scoped watch authz.
func newUnscopedServerWithRoles(t *testing.T) *ScopedServerWithRoles {
	t.Helper()

	roleSet, err := services.RoleSetFromSpec("admin-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{types.NewRule(types.Wildcard, []string{types.Wildcard})},
		},
	})
	require.NoError(t, err)

	unscopedCtx := &authz.Context{
		User:    &types.UserV2{Metadata: types.Metadata{Name: "admin"}},
		Checker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "test-cluster", roleSet),
	}
	return &ScopedServerWithRoles{
		scopedContext: authz.ScopedContextFromUnscopedContext(unscopedCtx),
	}
}

// TestUnscopedWatchKindAuthz verifies the scoped watch authz for classical unscoped callers on the
// scoped-first path: they default to (and may only scope-target via) safe filters, and can opt into all
// instances of a not-yet-namespaced kind via MODE_ALL.
func TestUnscopedWatchKindAuthz(t *testing.T) {
	ctx := context.Background()
	a := newUnscopedServerWithRoles(t)

	filter := func(mode scopesv1.Mode, scope string) *types.ScopeFilter {
		return types.ScopeFilterFromProto(scopesv1.Filter_builder{Mode: mode, Scope: scope}.Build())
	}

	tests := []struct {
		name      string
		kind      string
		filter    *types.ScopeFilter
		assertErr require.ErrorAssertionFunc
		wantMode  scopesv1.Mode
	}{
		{
			name:      "non-namespaced kind defaults to UNSCOPED",
			kind:      types.KindNode,
			filter:    nil,
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_UNSCOPED,
		},
		{
			// scoped-kind-whitelist: this test case and associated behaviore can be removed once all scoped
			// kinds are namespaced.
			name:      "non-namespaced kind ALL is allowed for unscoped caller",
			kind:      types.KindNode,
			filter:    filter(scopesv1.Mode_MODE_ALL, ""),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_ALL,
		},
		{
			// scoped-kind-whitelist: this test case and associated behaviore can be removed once all scoped
			// kinds are namespaced.
			name:      "non-namespaced kind EXACT denied by whitelist",
			kind:      types.KindNode,
			filter:    filter(scopesv1.Mode_MODE_EXACT, "/foo"),
			assertErr: requireAccessDenied,
		},
		{
			// this may seem somewhat nonsensical in this specific example, but in general we aren't actually
			// attempting to prevent the creation of filters that will never match anything, so long as said
			// filters are sound from a cache-correctness point of view.
			name:      "scoped_role defaults to UNSCOPED for unscoped caller",
			kind:      scopedaccess.KindScopedRole,
			filter:    nil,
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_UNSCOPED,
		},
		{
			name:      "scoped_role ALL allowed for unscoped caller",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ALL, ""),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_ALL,
		},
		{
			name:      "scoped_role EXACT allowed for unscoped caller",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, "/foo"),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_EXACT,
		},
		{
			name:      "scoped_role ANCESTORS allowed for unscoped caller (ordinary decision at root)",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_ANCESTORS, "/foo"),
			assertErr: require.NoError,
			wantMode:  scopesv1.Mode_MODE_ANCESTORS,
		},
		{
			name:      "malformed filter (mode without scope) rejected",
			kind:      scopedaccess.KindScopedRole,
			filter:    filter(scopesv1.Mode_MODE_EXACT, ""),
			assertErr: requireBadParameter,
		},
		{
			// scoped-kind-whitelist: this test case and associated behaviore can be removed once all scoped
			// kinds are namespaced.
			name:      "cert_authority scope-targeting filter denied by whitelist",
			kind:      types.KindCertAuthority,
			filter:    filter(scopesv1.Mode_MODE_EXACT, "/foo"),
			assertErr: requireAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.authorizeWatchKind(ctx, types.WatchKind{Kind: tt.kind, ScopeFilter: tt.filter})
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.wantMode, got.GetMode())
		})
	}
}
