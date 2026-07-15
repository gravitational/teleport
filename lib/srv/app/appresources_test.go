/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package app

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func newTestRole(t *testing.T, name, version string, allow types.RoleConditions) types.Role {
	t.Helper()
	role, err := types.NewRoleWithVersion(name, version, types.RoleSpecV6{Allow: allow})
	require.NoError(t, err)
	return role
}

func TestDecideMinimalV9(t *testing.T) {
	appMeta := types.Metadata{Name: "dev-app", Labels: map[string]string{"env": "dev"}}
	app, err := types.NewAppV3(appMeta, types.AppSpecV3{URI: "http://localhost"})
	require.NoError(t, err)

	devLabels := types.Labels{"env": []string{"dev"}}
	prodLabels := types.Labels{"env": []string{"prod"}}

	v8Grants := newTestRole(t, "v8-grants", types.V8, types.RoleConditions{AppLabels: devLabels})
	v8Other := newTestRole(t, "v8-other", types.V8, types.RoleConditions{AppLabels: prodLabels})
	v9AllowAll := newTestRole(t, "v9-allow-all", types.V9, types.RoleConditions{
		AppLabels:    devLabels,
		AppResources: []types.AppResource{{UnsafeAllowAll: true}},
	})
	// Built directly: write-time validation rejects paths in this release,
	// but the agent must still deny if such a role reaches it (for example
	// written by a later release during a rolling upgrade).
	v9Paths := &types.RoleV6{
		Metadata: types.Metadata{Name: "v9-paths"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{Allow: types.RoleConditions{
			Namespaces:   []string{apidefaults.Namespace},
			AppLabels:    devLabels,
			AppResources: []types.AppResource{{Paths: []string{"/health"}}},
		}},
	}
	// The deny stanza names a namespace that does not exist, so it must be
	// inert, exactly as in RoleSet.checkAccess: the role still grants the
	// app and v9 enforcement still applies.
	v9DenyOtherNamespace := &types.RoleV6{
		Metadata: types.Metadata{Name: "v9-deny-other-ns"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				AppLabels:  devLabels,
			},
			Deny: types.RoleConditions{
				Namespaces: []string{"other"},
				AppLabels:  devLabels,
			},
		},
	}
	v9DenyLabel := newTestRole(t, "v9-deny", types.V9, types.RoleConditions{
		AppLabels:    devLabels,
		AppResources: []types.AppResource{{UnsafeAllowAll: true}},
	})
	v9DenyLabel.SetAppLabels(types.Deny, devLabels)
	// A version above v9 must enforce default-deny exactly like v9, so a
	// role written by a later release fails closed rather than open.
	v10Grants := &types.RoleV6{
		Metadata: types.Metadata{Name: "v10-grants"},
		Version:  "v10",
		Spec: types.RoleSpecV6{Allow: types.RoleConditions{
			Namespaces: []string{apidefaults.Namespace},
			AppLabels:  devLabels,
		}},
	}
	// An unsafe_allow_all rule that also sets a companion field is invalid at
	// write time, but a role written by a later release could carry one. It is
	// not a pure escape hatch and must not open the app.
	v9ImpureUnsafe := &types.RoleV6{
		Metadata: types.Metadata{Name: "v9-impure-unsafe"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{Allow: types.RoleConditions{
			Namespaces:   []string{apidefaults.Namespace},
			AppLabels:    devLabels,
			AppResources: []types.AppResource{{UnsafeAllowAll: true, Where: "user.name == \"a\""}},
		}},
	}
	// Deny-side app rules are rejected at write time, but a role carrying
	// them is not plain unsafe_allow_all and must not open the app.
	v9UnsafeWithDenyRules := &types.RoleV6{
		Metadata: types.Metadata{Name: "v9-unsafe-deny-rules"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:   []string{apidefaults.Namespace},
				AppLabels:    devLabels,
				AppResources: []types.AppResource{{UnsafeAllowAll: true}},
			},
			Deny: types.RoleConditions{
				Namespaces:   []string{apidefaults.Namespace},
				AppResources: []types.AppResource{{Paths: []string{"/health"}}},
			},
		},
	}

	for _, tc := range []struct {
		desc  string
		roles []types.Role
		want  minimalV9Decision
	}{
		{
			desc:  "only v8 grants, no enforcement",
			roles: []types.Role{v8Grants},
			want:  minimalV9Decision{},
		},
		{
			desc:  "no role matches the app",
			roles: []types.Role{v8Other},
			want:  minimalV9Decision{},
		},
		{
			desc:  "v9 unsafe_allow_all forwards untouched",
			roles: []types.Role{v9AllowAll},
			want:  minimalV9Decision{enforced: true, allowed: true},
		},
		{
			desc:  "v9 paths rule denies without matcher",
			roles: []types.Role{v9Paths},
			want:  minimalV9Decision{enforced: true},
		},
		{
			desc:  "v9 drops a conflicting v8 role",
			roles: []types.Role{v9Paths, v8Grants},
			want:  minimalV9Decision{enforced: true, droppedRoles: []string{"v8-grants"}},
		},
		{
			desc:  "v9 unsafe_allow_all still drops the v8 role",
			roles: []types.Role{v9AllowAll, v8Grants},
			want:  minimalV9Decision{enforced: true, allowed: true, droppedRoles: []string{"v8-grants"}},
		},
		{
			desc:  "v9 role excluded by its own deny label",
			roles: []types.Role{v9DenyLabel},
			want:  minimalV9Decision{},
		},
		{
			desc:  "deny stanza in another namespace stays inert",
			roles: []types.Role{v9DenyOtherNamespace},
			want:  minimalV9Decision{enforced: true},
		},
		{
			desc:  "version above v9 enforces default-deny",
			roles: []types.Role{v10Grants},
			want:  minimalV9Decision{enforced: true},
		},
		{
			desc:  "deny app rules block unsafe_allow_all",
			roles: []types.Role{v9UnsafeWithDenyRules},
			want:  minimalV9Decision{enforced: true},
		},
		{
			desc:  "impure unsafe_allow_all does not open the app",
			roles: []types.Role{v9ImpureUnsafe},
			want:  minimalV9Decision{enforced: true},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := decideMinimalV9(tc.roles, app, "alice", nil)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsCORSPreflight(t *testing.T) {
	newRequest := func(method string, headers map[string]string) *http.Request {
		r := httptest.NewRequest(method, "http://app/", nil)
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}

	corsHeaders := map[string]string{
		"Origin":                        "http://origin",
		"Access-Control-Request-Method": http.MethodPost,
	}

	require.True(t, isCORSPreflight(newRequest(http.MethodOptions, corsHeaders)))
	require.False(t, isCORSPreflight(newRequest(http.MethodGet, corsHeaders)))
	require.False(t, isCORSPreflight(newRequest(http.MethodOptions, map[string]string{"Origin": "http://origin"})))
	require.False(t, isCORSPreflight(newRequest(http.MethodOptions, nil)))
}

func TestEnforceMinimalV9(t *testing.T) {
	appMeta := types.Metadata{Name: "dev-app", Labels: map[string]string{"env": "dev"}}
	app, err := types.NewAppV3(appMeta, types.AppSpecV3{URI: "http://localhost"})
	require.NoError(t, err)

	devLabels := types.Labels{"env": []string{"dev"}}

	// Built directly: write-time validation rejects a v9 rule with paths, but
	// the agent must still deny if such a role reaches it during a rolling
	// upgrade.
	denyAll := &types.RoleV6{
		Metadata: types.Metadata{Name: "deny-all"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{Allow: types.RoleConditions{
			Namespaces:   []string{apidefaults.Namespace},
			AppLabels:    devLabels,
			AppResources: []types.AppResource{{Paths: []string{"/health"}}},
		}},
	}
	allowAll := newTestRole(t, "allow-all", types.V9, types.RoleConditions{
		AppLabels:    devLabels,
		AppResources: []types.AppResource{{UnsafeAllowAll: true}},
	})

	authContext := func(role types.Role) *authz.Context {
		checker := services.NewAccessCheckerWithRoleSet(
			&services.AccessInfo{Username: "alice", Roles: []string{role.GetName()}},
			"cluster", services.NewRoleSet(role))
		return &authz.Context{
			Identity: authz.WrapIdentity(tlsca.Identity{Username: "alice"}),
			Checker:  checker,
		}
	}

	newHandler := func(logs *strings.Builder) *ConnectionsHandler {
		return &ConnectionsHandler{log: slog.New(slog.NewTextHandler(logs, nil))}
	}

	t.Run("denies a plain request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://dev-app/", nil)
		denied, err := newHandler(&strings.Builder{}).enforceMinimalV9(rec, req, authContext(denyAll), app)
		require.NoError(t, err)
		require.True(t, denied)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), denyKindRequestNotAllowed)
	})

	t.Run("allows unsafe_allow_all", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://dev-app/", nil)
		denied, err := newHandler(&strings.Builder{}).enforceMinimalV9(rec, req, authContext(allowAll), app)
		require.NoError(t, err)
		require.False(t, denied)
		require.Zero(t, rec.Body.Len(), "the handler must not write on the allow path")
	})

	t.Run("denies and warns on a CORS preflight", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "http://dev-app/", nil)
		req.Header.Set("Origin", "http://origin")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		logs := &strings.Builder{}
		denied, err := newHandler(logs).enforceMinimalV9(rec, req, authContext(denyAll), app)
		require.NoError(t, err)
		require.True(t, denied)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, logs.String(), "Denied CORS preflight")
	})
}
