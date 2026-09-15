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

package appresource

import (
	"cmp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// futureField encodes AppResource field 99, far above the allocated field
// numbers, as a newer version would send it over gRPC. The bytes are the
// two-byte tag 0x9A 0x06 (field 99, wire type 2), length 1, and the value
// "x".
var futureField = []byte{0x9A, 0x06, 0x01, 0x78}

func newRole(name, version string) *types.RoleV6 {
	return &types.RoleV6{Version: version, Metadata: types.Metadata{Name: name}}
}

func v9Role(name string, allow types.RoleConditions) *types.RoleV6 {
	role := newRole(name, types.V9)
	role.Spec.Allow = allow
	return role
}

func TestNewRole(t *testing.T) {
	tests := []struct {
		name string
		role *types.RoleV6
		want Role
	}{
		{
			name: "no app rules",
			role: v9Role("plain", types.RoleConditions{}),
			want: Role{Name: "plain"},
		},
		{
			name: "all rule fields",
			role: v9Role("dev", types.RoleConditions{
				AppResources: []types.AppResource{{
					Paths:          []string{"/api/v4/projects/{project}/**"},
					Methods:        []string{"GET", "HEAD"},
					Where:          `contains(user.traits["allowed_projects"], vars.project)`,
					AllowEncoded:   []string{"/"},
					AllowCode:      "repo_read",
					AllowReason:    "Read access to the repository API",
					DenyCodeHint:   "project_not_allowed",
					DenyReasonHint: "Project is not in the caller's allowlist",
				}},
			}),
			want: Role{
				Name: "dev",
				Resources: []types.AppResource{{
					Paths:          []string{"/api/v4/projects/{project}/**"},
					Methods:        []string{"GET", "HEAD"},
					Where:          `contains(user.traits["allowed_projects"], vars.project)`,
					AllowEncoded:   []string{"/"},
					AllowCode:      "repo_read",
					AllowReason:    "Read access to the repository API",
					DenyCodeHint:   "project_not_allowed",
					DenyReasonHint: "Project is not in the caller's allowlist",
				}},
			},
		},
		{
			name: "allow_all rule",
			role: v9Role("legacy", types.RoleConditions{
				AppResources: []types.AppResource{{AllowAll: true}},
			}),
			want: Role{
				Name:      "legacy",
				Resources: []types.AppResource{{AllowAll: true}},
			},
		},
		{
			name: "expressions",
			role: v9Role("expr", types.RoleConditions{
				AppResourcesExpressions: []string{`path.match(literal("health"))`},
			}),
			want: Role{
				Name:        "expr",
				Expressions: []string{`path.match(literal("health"))`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRole(tt.role)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewRoleError(t *testing.T) {
	denyRules := newRole("denier", types.V9)
	denyRules.Spec.Deny.AppResources = []types.AppResource{{AllowAll: true}}
	denyRulesNewer := newRole("denier", "v10")
	denyRulesNewer.Spec.Deny.AppResources = []types.AppResource{{AllowAll: true}}
	denyExpressions := newRole("denier", types.V9)
	denyExpressions.Spec.Deny.AppResourcesExpressions = []string{`path.match(literal("health"))`}

	tests := []struct {
		name        string
		role        *types.RoleV6
		wantErr     error
		wantMessage string
	}{
		{
			name:    "v8 role predates v9",
			role:    newRole("old", types.V8),
			wantErr: ErrRolePredatesV9,
		},
		{
			name:    "version above v9",
			role:    newRole("future", "v10"),
			wantErr: ErrRoleNotEvaluable,
		},
		{
			name:        "deny resources above v9",
			role:        denyRulesNewer,
			wantErr:     ErrRoleHasDenyRules,
			wantMessage: `role "denier" sets app_resources under deny`,
		},
		{
			name:        "deny resources",
			role:        denyRules,
			wantErr:     ErrRoleHasDenyRules,
			wantMessage: `role "denier" sets app_resources under deny`,
		},
		{
			name:        "deny expressions",
			role:        denyExpressions,
			wantErr:     ErrRoleHasDenyRules,
			wantMessage: `role "denier" sets app_resources_expressions under deny`,
		},
		{
			name: "unrecognized field",
			role: v9Role("newer", types.RoleConditions{
				AppResources: []types.AppResource{{Paths: []string{"/health"}}, {Paths: []string{"/api/**"}, XXX_unrecognized: futureField}},
			}),
			wantErr:     ErrRoleNotEvaluable,
			wantMessage: `role "newer" app_resources 1 has an unrecognized field`,
		},
		{
			name: "allow_all with another field",
			role: v9Role("combined", types.RoleConditions{
				AppResources: []types.AppResource{{AllowAll: true, Paths: []string{"/admin/**"}}},
			}),
			wantErr:     ErrRoleNotEvaluable,
			wantMessage: `role "combined" app_resources 0 combines allow_all with another field`,
		},
		{
			name: "allow_all with another rule",
			role: v9Role("mixed", types.RoleConditions{
				AppResources: []types.AppResource{{AllowAll: true}, {Paths: []string{"/api/**"}}},
			}),
			wantErr:     ErrRoleNotEvaluable,
			wantMessage: `role "mixed" app_resources 0 sets allow_all with another rule`,
		},
		{
			name: "allow_all with an expression",
			role: v9Role("mixedexpr", types.RoleConditions{
				AppResources:            []types.AppResource{{AllowAll: true}},
				AppResourcesExpressions: []string{`path.match(literal("health"))`},
			}),
			wantErr:     ErrRoleNotEvaluable,
			wantMessage: `role "mixedexpr" app_resources 0 sets allow_all with an app_resources_expressions entry`,
		},
		{
			name: "empty rule",
			role: v9Role("emptied", types.RoleConditions{
				AppResources: []types.AppResource{{}},
			}),
			wantErr:     ErrRoleNotEvaluable,
			wantMessage: `role "emptied" app_resources 0 is blank`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRole(tt.role)
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantMessage != "" {
				require.ErrorContains(t, err, tt.wantMessage)
			}
		})
	}
}

func TestRoleVersionPredatesV9(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: types.V1, want: true},
		{version: types.V2, want: true},
		{version: types.V3, want: true},
		{version: types.V4, want: true},
		{version: types.V5, want: true},
		{version: types.V6, want: true},
		{version: types.V7, want: true},
		{version: types.V8, want: true},
		{version: types.V9, want: false},
		{version: "v10", want: false},
		{version: "", want: false},
		{version: "V8", want: false},
	}
	for _, tt := range tests {
		t.Run(cmp.Or(tt.version, "empty"), func(t *testing.T) {
			require.Equal(t, tt.want, RoleVersionPredatesV9(tt.version))
		})
	}
}

// TestErrRoleHasDenyRulesWrapsNotEvaluable checks that a caller matching
// only ErrRoleNotEvaluable still matches a role with rules under deny.
func TestErrRoleHasDenyRulesWrapsNotEvaluable(t *testing.T) {
	require.ErrorIs(t, ErrRoleHasDenyRules, ErrRoleNotEvaluable)
}

func TestPartitionRoles(t *testing.T) {
	opener := v9Role("opener", types.RoleConditions{AppResources: []types.AppResource{{AllowAll: true}}})
	reader := v9Role("reader", types.RoleConditions{AppResources: []types.AppResource{{Paths: []string{"/x"}}}})
	newer := newRole("newer", "v10")
	older := newRole("older", types.V8)
	oldest := newRole("oldest", types.V6)

	tests := []struct {
		name                 string
		roles                []types.Role
		wantNames            []string
		wantIgnoredRoleNames []string
		wantErrCount         int
	}{
		{
			name:                 "v9 roles keep their order",
			roles:                []types.Role{reader, opener},
			wantNames:            []string{"reader", "opener"},
			wantIgnoredRoleNames: nil,
			wantErrCount:         0,
		},
		{
			name:                 "pre-v9 names are sorted",
			roles:                []types.Role{older, oldest},
			wantNames:            nil,
			wantIgnoredRoleNames: []string{"older", "oldest"},
			wantErrCount:         0,
		},
		{
			name:                 "unknown version becomes an error",
			roles:                []types.Role{newer},
			wantNames:            nil,
			wantIgnoredRoleNames: nil,
			wantErrCount:         1,
		},
		{
			name:                 "all three at once",
			roles:                []types.Role{opener, older, newer},
			wantNames:            []string{"opener"},
			wantIgnoredRoleNames: []string{"older"},
			wantErrCount:         1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PartitionRoles(tt.roles)
			var names []string
			for _, role := range got.Roles {
				names = append(names, role.Name)
			}
			require.Equal(t, tt.wantNames, names)
			require.Equal(t, tt.wantIgnoredRoleNames, got.IgnoredRoleNames)
			require.Len(t, got.Errors, tt.wantErrCount)
			for _, err := range got.Errors {
				require.ErrorIs(t, err, ErrRoleNotEvaluable)
			}
		})
	}
}
