// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package access

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// TestEmptyRoleConverts verifies that a scoped role that is empty except for
// scoping/metadata converts to a reasonable default/unprivileged classic role.
func TestEmptyRoleConverts(t *testing.T) {
	t.Parallel()

	role, err := ScopedRoleToRole(&scopedaccessv1.ScopedRole{
		Kind: KindScopedRole,
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Scope: "/foo",
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{"/foo/bar"},
		},
		Version: types.V1,
	}, "/foo/bar")
	require.NoError(t, err)
	require.NotNil(t, role)
	require.Equal(t, "test@/foo/bar", role.GetName())
}

// TestSSHConversion verifies the various SSH-related scoped role rule conversion scenarios.
func TestSSHConversion(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name       string
		conditions *scopedaccessv1.ScopedRoleConditions
		expect     types.RoleConditions
	}{
		{
			name:       "empty conditions",
			conditions: &scopedaccessv1.ScopedRoleConditions{},
			expect:     types.RoleConditions{},
		},
		{
			name: "sparse",
			conditions: &scopedaccessv1.ScopedRoleConditions{
				Logins: []string{"root"},
				NodeLabels: []*labelv1.Label{
					{
						Name:   "team",
						Values: []string{"red"},
					},
				},
			},
			expect: types.RoleConditions{
				Logins: []string{"root"},
				NodeLabels: types.Labels{
					"team": apiutils.Strings{"red"},
				},
			},
		},
		{
			name: "full",
			conditions: &scopedaccessv1.ScopedRoleConditions{
				Logins: []string{"root", "admin"},
				NodeLabels: []*labelv1.Label{
					{
						Name:   "env",
						Values: []string{"prod", "staging"},
					},
					{
						Name:   "team",
						Values: []string{"blue"},
					},
				},
			},
			expect: types.RoleConditions{
				Logins: []string{"root", "admin"},
				NodeLabels: types.Labels{
					"env":  apiutils.Strings{"prod", "staging"},
					"team": apiutils.Strings{"blue"},
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			role, err := ScopedRoleToRole(&scopedaccessv1.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "/foo",
				Spec: &scopedaccessv1.ScopedRoleSpec{
					AssignableScopes: []string{"/foo/bar"},
					Allow:            tt.conditions,
				},
				Version: types.V1,
			}, "/foo/bar")
			require.NoError(t, err)
			tt.expect.Namespaces = []string{"default"}
			require.Empty(t, cmp.Diff(tt.expect, role.GetRoleConditions(types.Allow)))
		})
	}
}

// TestRulesConversion verifies various scoped role rule conversion scenarios.
func TestRulesConversion(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		rules  []*scopedaccessv1.ScopedRule
		expect []types.Rule
	}{
		{
			name: "basic",
			rules: []*scopedaccessv1.ScopedRule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
				{
					Resources: []string{KindScopedRoleAssignment},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
				},
			},
			expect: []types.Rule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
				{
					Resources: []string{KindScopedRoleAssignment},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
				},
			},
		},
		{
			name: "unsupported verb",
			rules: []*scopedaccessv1.ScopedRule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbRead},
				},
			},
			expect: []types.Rule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList},
				},
			},
		},
		{
			name: "unsupported resource",
			rules: []*scopedaccessv1.ScopedRule{
				{
					Resources: []string{types.KindCertAuthority},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
			},
			expect: []types.Rule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			role, err := ScopedRoleToRole(&scopedaccessv1.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "/foo",
				Spec: &scopedaccessv1.ScopedRoleSpec{
					AssignableScopes: []string{"/foo/bar"},
					Allow: &scopedaccessv1.ScopedRoleConditions{
						Rules: tt.rules,
					},
				},
				Version: types.V1,
			}, "/foo/bar")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.expect, role.GetRules(types.Allow)))
		})
	}
}
