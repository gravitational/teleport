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

package services

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
)

func TestScopedACCGetPossibleLoginsForSSHServer(t *testing.T) {
	t.Parallel()

	scopedCheckers := []*ScopedAccessChecker{
		{
			role: &scopedaccessv1.ScopedRole{
				Metadata: &headerv1.Metadata{
					Name: "test-role-1",
				},
				Scope: "/test",
				Spec: &scopedaccessv1.ScopedRoleSpec{
					AssignableScopes: []string{"/test/foo"},
					Ssh: &scopedaccessv1.ScopedRoleSSH{
						Logins: []string{"test-login-1"},
						Labels: []*labelv1.Label{
							{
								Name:   types.Wildcard,
								Values: []string{types.Wildcard},
							},
						},
					},
				},
				Version: types.V1,
			},
		},
		{
			role: &scopedaccessv1.ScopedRole{
				Metadata: &headerv1.Metadata{
					Name: "test-role-2",
				},
				Scope: "/test",
				Spec: &scopedaccessv1.ScopedRoleSpec{
					AssignableScopes: []string{"/test/foo"},
					Ssh: &scopedaccessv1.ScopedRoleSSH{
						Logins: []string{"test-login-2"},
						Labels: []*labelv1.Label{
							{
								Name:   "foo",
								Values: []string{"bar"},
							},
						},
					},
				},
				Version: types.V1,
			},
		},
		{
			role: &scopedaccessv1.ScopedRole{
				Metadata: &headerv1.Metadata{
					Name: "test-role-3",
				},
				Scope: "/test",
				Spec: &scopedaccessv1.ScopedRoleSpec{
					AssignableScopes: []string{"/test"},
					Ssh: &scopedaccessv1.ScopedRoleSSH{
						Logins: []string{"test-login-3"},
						Labels: []*labelv1.Label{
							{
								Name:   types.Wildcard,
								Values: []string{types.Wildcard},
							},
						},
					},
				},
				Version: types.V1,
			},
		},
	}

	accessInfo := &AccessInfo{
		ScopePin: &scopesv1.Pin{
			Scope: "/test",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/test": {
					"/test/foo": {"test-role-1", "test-role-2"},
					"/test":     {"test-role-3"},
				},
			}),
		},
	}

	m := make(map[roleCheckerKey]*ScopedAccessChecker)
	for _, checker := range scopedCheckers {
		key := roleCheckerKey{
			scopeOfOrigin: checker.role.GetScope(),
			scopeOfEffect: checker.role.GetSpec().GetAssignableScopes()[0],
			roleName:      checker.role.GetMetadata().GetName(),
		}
		compatRole, err := access.ScopedRoleToRole(checker.role, checker.role.GetScope())
		require.NoError(t, err)
		checker.scopedCompatChecker = newAccessChecker(accessInfo, "test-cluster", NewRoleSet(compatRole))
		m[key] = checker
	}

	checkerCache := func(ctx context.Context, key roleCheckerKey) (*ScopedAccessChecker, error) {
		checker, ok := m[key]
		if !ok {
			return nil, trace.NotFound("checker not found")
		}
		return checker, nil
	}

	cases := []struct {
		name           string
		checkerCache   func(context.Context, roleCheckerKey) (*ScopedAccessChecker, error)
		pin            *scopesv1.Pin
		expectedLogins []string
		expectErr      bool
		server         types.Server
	}{
		{
			name:         "pinned to /test, all roles match",
			checkerCache: checkerCache,
			server: &types.ServerV2{
				Kind:  types.KindNode,
				Scope: "/test/foo",
				Metadata: types.Metadata{
					Name: "test-server",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expectedLogins: []string{"test-login-1", "test-login-2", "test-login-3"},
		},
		{
			name:         "pinned to /test, only wildcards match",
			checkerCache: checkerCache,
			server: &types.ServerV2{
				Kind:  types.KindNode,
				Scope: "/test/foo",
				Metadata: types.Metadata{
					Name: "test-server",
				},
			},
			expectedLogins: []string{"test-login-1", "test-login-3"},
		},
		{
			name:         "pinned to /test, orthogonal scope",
			checkerCache: checkerCache,
			server: &types.ServerV2{
				Kind:  types.KindNode,
				Scope: "/other",
				Metadata: types.Metadata{
					Name: "test-server",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expectErr: true,
		},
		{
			name:         "pinned to /test, server in /test/bar",
			checkerCache: checkerCache,
			server: &types.ServerV2{
				Kind:  types.KindNode,
				Scope: "/test/bar",
				Metadata: types.Metadata{
					Name: "test-server",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			// since role 3 is assigned at /test, its login should still be returned even though no roles
			// are explicitly assigned at /test/bar
			expectedLogins: []string{"test-login-3"},
		},
		{
			name:         "pinned to /test/foo, only test-role-1 matches",
			checkerCache: checkerCache,
			pin: &scopesv1.Pin{
				Scope: "/test/foo",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/test": {"/test/foo": {"test-role-1", "test-role-2"}},
				}),
			},
			server: &types.ServerV2{
				Kind:  types.KindNode,
				Scope: "/test/foo",
				Metadata: types.Metadata{
					Name: "test-server",
				},
			},
			expectedLogins: []string{"test-login-1"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			info := *accessInfo
			if c.pin != nil {
				info.ScopePin = c.pin
			}
			scopedACC := ScopedAccessCheckerContext{
				cachedCheckerForRole: c.checkerCache,
				builder: scopedAccessCheckerBuilder{
					info: &info,
				},
			}

			logins, err := scopedACC.GetPossibleLoginsForSSHServer(t.Context(), c.server)
			if c.expectErr {
				require.Error(t, err)
			} else {
				require.ElementsMatch(t, c.expectedLogins, logins)
			}
		})
	}
}
