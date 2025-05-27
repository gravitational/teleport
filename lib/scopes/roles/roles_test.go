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

package roles

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	srpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedrole/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestValidateRole verifies basic functionality of strong and weak role validation functions.
func TestValidateRole(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		role     *srpb.ScopedRole
		strongOk bool
		weakOk   bool
	}{
		{
			name: "basic",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "unknown sub_kind",
			role: &srpb.ScopedRole{
				Kind:    KindScopedRole,
				SubKind: "unknown",
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing name",
			role: &srpb.ScopedRole{
				Kind:     KindScopedRole,
				Metadata: &headerpb.Metadata{},
				Scope:    "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing kind",
			role: &srpb.ScopedRole{
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing scope",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing version",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "malformed name",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "foo/bar",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed kind",
			role: &srpb.ScopedRole{
				Kind: "role",
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "slightly malformed scope",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "foo/bar",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo/bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "siginifcantly malformed scope",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "foo@bar",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo/bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing assignable scopes",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope:   "/",
				Spec:    &srpb.ScopedRoleSpec{},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "slightly malformed assignable scope",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"foo/bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "siginifcantly malformed assignable scope",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"foo@bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true, // invalid assignable scopes are totally ignored, even if they don't pass weak validation rules
		},
		{
			name: "impermissable assignable scope",
			role: &srpb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/foo/bar",
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidateRole(tt.role)
			if tt.strongOk {
				require.NoError(t, err, "strong validation should not fail")
			} else {
				require.Error(t, err, "strong validation should fail")
			}

			err = WeakValidateRole(tt.role)
			if tt.weakOk {
				require.NoError(t, err, "weak validation should not fail")
			} else {
				require.Error(t, err, "weak validation should fail")
			}
		})
	}
}

func TestValidateAsssignment(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name       string
		assignment *srpb.ScopedRoleAssignment
		strongOk   bool
		weakOk     bool
	}{
		{
			name: "basic",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "unknown sub_kind",
			assignment: &srpb.ScopedRoleAssignment{
				Kind:    KindScopedRoleAssignment,
				SubKind: "unknown",
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing name",
			assignment: &srpb.ScopedRoleAssignment{
				Kind:     KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{},
				Scope:    "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing kind",
			assignment: &srpb.ScopedRoleAssignment{
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing scope",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing version",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "malformed name",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: "not-a-uuid",
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed kind",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: "not_scoped_role_assignment",
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "slightly malformed scope",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "foo",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "significantly malformed scope",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "foo@bar",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "impermissable assigned scope",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/foo/bar",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed assigned scope",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "basic",
			assignment: &srpb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &srpb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*srpb.Assignment{
						{
							Role:  "test",
							Scope: "/foo",
						},
					},
				},
				Version: types.V1,
			},
			strongOk: true,
			weakOk:   true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidateAssignment(tt.assignment)
			if tt.strongOk {
				require.NoError(t, err, "strong validation should not fail")
			} else {
				require.Error(t, err, "strong validation should fail")
			}

			err = WeakValidateAssignment(tt.assignment)
			if tt.weakOk {
				require.NoError(t, err, "weak validation should not fail")
			} else {
				require.Error(t, err, "weak validation should fail")
			}
		})
	}
}

func TestWeakValidatedAssignableScopes(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name             string
		scope            string
		assignableScopes []string
		expect           []string
	}{
		{
			name:             "basic",
			scope:            "/foo",
			assignableScopes: []string{"/foo/bar", "/foo/baz"},
			expect:           []string{"/foo/bar", "/foo/baz"},
		},
		{
			name:             "empty",
			scope:            "/foo",
			assignableScopes: nil,
			expect:           nil,
		},
		{
			name:             "mildly malformed",
			scope:            "/foo",
			assignableScopes: []string{"foo/bar", "foo/baz"},
			expect:           []string{"foo/bar", "foo/baz"},
		},
		{
			name:             "significantly malformed",
			scope:            "/foo",
			assignableScopes: []string{"foo@bar", "foo@baz"},
			expect:           nil,
		},
		{
			name:             "impermissible",
			scope:            "/foo/bar",
			assignableScopes: []string{"/foo", "/foo/bar"},
			expect:           []string{"/foo/bar"},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			result := slices.Collect(WeakValidatedAssignableScopes(&srpb.ScopedRole{
				Scope: tt.scope,
				Spec: &srpb.ScopedRoleSpec{
					AssignableScopes: tt.assignableScopes,
				},
			}))
			require.Equal(t, tt.expect, result)
		})
	}
}

func TestWeakValidatedSubAssignments(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name        string
		scope       string
		assignments []*srpb.Assignment
		expect      []*srpb.Assignment
	}{
		{
			name:  "basic",
			scope: "/foo",
			assignments: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
		},
		{
			name:  "mildly malformed scope",
			scope: "/foo",
			assignments: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
		},
		{
			name:  "significantly malformed scope",
			scope: "/foo",
			assignments: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "foo@bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
		},
		{
			name:  "impermissible scope",
			scope: "/foo/bar",
			assignments: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
			},
		},
		{
			name:  "missing scope",
			scope: "/foo",
			assignments: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
		},
		{
			name:  "missing role",
			scope: "/foo",
			assignments: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "",
					Scope: "/foo/baz",
				},
			},
			expect: []*srpb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			result := slices.Collect(WeakValidatedSubAssignments(&srpb.ScopedRoleAssignment{
				Scope: tt.scope,
				Spec: &srpb.ScopedRoleAssignmentSpec{
					Assignments: tt.assignments,
				},
			}))
			require.Equal(t, tt.expect, result)
		})
	}
}
