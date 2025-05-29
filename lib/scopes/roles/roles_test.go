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
	accesspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestValidateRole verifies basic functionality of strong and weak role validation functions.
func TestValidateRole(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		role     *accesspb.ScopedRole
		strongOk bool
		weakOk   bool
	}{
		{
			name: "basic",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "unknown sub_kind",
			role: &accesspb.ScopedRole{
				Kind:    KindScopedRole,
				SubKind: "unknown",
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing name",
			role: &accesspb.ScopedRole{
				Kind:     KindScopedRole,
				Metadata: &headerpb.Metadata{},
				Scope:    "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing kind",
			role: &accesspb.ScopedRole{
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing scope",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing version",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "malformed name",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "foo/bar",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed kind",
			role: &accesspb.ScopedRole{
				Kind: "role",
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "slightly malformed scope",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "foo/bar",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo/bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "siginifcantly malformed scope",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "foo@bar",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"/foo/bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing assignable scopes",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope:   "/",
				Spec:    &accesspb.ScopedRoleSpec{},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "slightly malformed assignable scope",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"foo/bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "siginifcantly malformed assignable scope",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleSpec{
					AssignableScopes: []string{"foo@bar"},
				},
				Version: types.V1,
			},
			strongOk: false,
			weakOk:   true, // invalid assignable scopes are totally ignored, even if they don't pass weak validation rules
		},
		{
			name: "impermissable assignable scope",
			role: &accesspb.ScopedRole{
				Kind: KindScopedRole,
				Metadata: &headerpb.Metadata{
					Name: "test",
				},
				Scope: "/foo/bar",
				Spec: &accesspb.ScopedRoleSpec{
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
		assignment *accesspb.ScopedRoleAssignment
		strongOk   bool
		weakOk     bool
	}{
		{
			name: "basic",
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind:    KindScopedRoleAssignment,
				SubKind: "unknown",
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind:     KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{},
				Scope:    "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: "not-a-uuid",
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: "not_scoped_role_assignment",
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "foo",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "foo@bar",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/foo/bar",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			assignment: &accesspb.ScopedRoleAssignment{
				Kind: KindScopedRoleAssignment,
				Metadata: &headerpb.Metadata{
					Name: uuid.New().String(),
				},
				Scope: "/",
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					User: "alice",
					Assignments: []*accesspb.Assignment{
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
			result := slices.Collect(WeakValidatedAssignableScopes(&accesspb.ScopedRole{
				Scope: tt.scope,
				Spec: &accesspb.ScopedRoleSpec{
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
		assignments []*accesspb.Assignment
		expect      []*accesspb.Assignment
	}{
		{
			name:  "basic",
			scope: "/foo",
			assignments: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*accesspb.Assignment{
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
			assignments: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*accesspb.Assignment{
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
			assignments: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "foo@bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
		},
		{
			name:  "impermissible scope",
			scope: "/foo/bar",
			assignments: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
			},
		},
		{
			name:  "missing scope",
			scope: "/foo",
			assignments: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "",
				},
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
			expect: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/baz",
				},
			},
		},
		{
			name:  "missing role",
			scope: "/foo",
			assignments: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
				{
					Role:  "",
					Scope: "/foo/baz",
				},
			},
			expect: []*accesspb.Assignment{
				{
					Role:  "test",
					Scope: "/foo/bar",
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			result := slices.Collect(WeakValidatedSubAssignments(&accesspb.ScopedRoleAssignment{
				Scope: tt.scope,
				Spec: &accesspb.ScopedRoleAssignmentSpec{
					Assignments: tt.assignments,
				},
			}))
			require.Equal(t, tt.expect, result)
		})
	}
}
