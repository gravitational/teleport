/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package pinning

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		pin      *scopesv1.Pin
		strongOk bool
		weakOk   bool
	}{
		{
			name: "basic",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/foo": {
						Roles: []string{"r2"},
					},
					"/foo/bar": {
						Roles: []string{"r3"},
					},
				},
			},
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "missing scope",
			pin: &scopesv1.Pin{
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
				},
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing assignments",
			pin: &scopesv1.Pin{
				Scope: "/foo",
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "orthogonal assignment",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/bar": {
						Roles: []string{"r2"},
					},
				},
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "empty assignments",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/foo": {
						Roles: []string{},
					},
				},
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed assignment scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"invalid@scope": {
						Roles: []string{"r2"},
					},
				},
			},
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "malformed pin scope",
			pin: &scopesv1.Pin{
				Scope: "invalid@scope",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
				},
			},
			strongOk: false,
			weakOk:   false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidate(tt.pin)
			if tt.strongOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			err = WeakValidate(tt.pin)
			if tt.weakOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestAssignmentsForResourceScope verifies the expected behavior of the AssignmentsForResourceScope helper.
func TestAssignmentsForResourceScope(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		pin    *scopesv1.Pin
		scope  string
		ok     bool
		expect []string
	}{
		{
			name: "basic",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/foo": {
						Roles: []string{"r2"},
					},
					"/foo/bar": {
						Roles: []string{"r3"},
					},
				},
			},
			scope:  "/foo/bar",
			ok:     true,
			expect: []string{"/", "/foo", "/foo/bar"},
		},
		{
			name: "no assignments for scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/foo/bar": {
						Roles: []string{"r1"},
					},
				},
			},
			scope:  "/foo",
			ok:     true,
			expect: nil,
		},
		{
			name: "partial assignments",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/foo/bar": {
						Roles: []string{"r2"},
					},
					"/foo/bar/bin": {
						Roles: []string{"r3"},
					},
					"/foo/bin": {
						Roles: []string{"r4"},
					},
				},
			},
			scope:  "/foo/bar",
			ok:     true,
			expect: []string{"/", "/foo/bar"},
		},
		{
			name: "parent resource scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/foo": {
						Roles: []string{"r2"},
					},
				},
			},
			scope:  "/",
			ok:     false,
			expect: nil,
		},
		{
			name: "orthogonal resource scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/": {
						Roles: []string{"r1"},
					},
					"/bar": {
						Roles: []string{"r2"},
					},
				},
			},
			scope:  "/bar",
			ok:     false,
			expect: nil,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			var seen []string

			assignments, err := AssignmentsForResourceScope(tt.pin, tt.scope)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}

			for scope, assignment := range assignments {
				seen = append(seen, scope)
				require.Empty(t, cmp.Diff(tt.pin.GetAssignments()[scope], assignment, protocmp.Transform()))
			}
			require.Equal(t, tt.expect, seen)
		})
	}
}

// TestDescendAssignmentTree verifies the expected behavior of the DescendAssignmentTree helper. In particular, this test
// is used to validate the fact that the DescendAssignmentTree function yields the correct role assignments in the correct
// order for access-control decisions.
func TestDescendAssignmentTree(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		pin    *scopesv1.Pin
		scope  string
		ok     bool
		expect []RoleAssignment
	}{
		{
			name: "single-role",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/foo": {
						"/foo": {"r1"},
					},
				}),
			},
			scope: "/foo",
			ok:    true,
			expect: []RoleAssignment{
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "r1",
				},
			},
		},
		{
			name: "hierarchical multi",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/": {"r1"},
					},
					"/foo": {
						"/foo": {"r2"},
					},
					"/foo/bar": {
						"/foo/bar": {"r3"},
					},
				}),
			},
			scope: "/foo",
			ok:    true,
			expect: []RoleAssignment{
				{
					ScopeOfOrigin: "/",
					ScopeOfEffect: "/",
					RoleName:      "r1",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "r2",
				},
			},
		},
		{
			name: "single scope multi",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":        {"r1"},
						"/foo":     {"r2"},
						"/foo/bar": {"r3"},
					},
				}),
			},
			scope: "/foo",
			ok:    true,
			expect: []RoleAssignment{
				{
					ScopeOfOrigin: "/",
					ScopeOfEffect: "/foo",
					RoleName:      "r2",
				},
				{
					ScopeOfOrigin: "/",
					ScopeOfEffect: "/",
					RoleName:      "r1",
				},
			},
		},
		{
			name: "partially orthogonal",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/": {"r1"},
					},
					"/foo/bar": {
						"/foo/bar": {"r2"},
					},
					"/foo/baz": {
						"/foo/baz": {"r3"},
					},
				}),
			},
			scope: "/foo/bar",
			ok:    true,
			expect: []RoleAssignment{
				{
					ScopeOfOrigin: "/",
					ScopeOfEffect: "/",
					RoleName:      "r1",
				},
				{
					ScopeOfOrigin: "/foo/bar",
					ScopeOfEffect: "/foo/bar",
					RoleName:      "r2",
				},
			},
		},
		{
			name: "fully orthogonal",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/foo/bar": {
						"/foo/bar": {"r1"},
					},
				}),
			},
			scope:  "/foo/baz",
			ok:     true,
			expect: nil,
		},
		{
			name: "equivalent scoping",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/foo": {
						"/foo": {"b", "c", "a", "x", "q"},
					},
				}),
			},
			scope: "/foo",
			ok:    true,
			expect: []RoleAssignment{
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "a",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "b",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "c",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "q",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "x",
				},
			},
		},
		{
			name: "comprehensive",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":        {"rr1"},
						"/foo/bar": {"rr2"},
					},
					"/foo": {
						"/foo":         {"rf1"},
						"/foo/bar":     {"rf3", "rf2"},
						"/foo/bar/baz": {"rf4"},
					},
					"/foo/bar": {
						"/foo/bar":     {"rb1"},
						"/foo/bar/baz": {"rb2"},
					},
				}),
			},
			scope: "/foo/bar",
			ok:    true,
			expect: []RoleAssignment{
				{
					ScopeOfOrigin: "/",
					ScopeOfEffect: "/foo/bar",
					RoleName:      "rr2",
				},
				{
					ScopeOfOrigin: "/",
					ScopeOfEffect: "/",
					RoleName:      "rr1",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo/bar",
					RoleName:      "rf2",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo/bar",
					RoleName:      "rf3",
				},
				{
					ScopeOfOrigin: "/foo",
					ScopeOfEffect: "/foo",
					RoleName:      "rf1",
				},
				{
					ScopeOfOrigin: "/foo/bar",
					ScopeOfEffect: "/foo/bar",
					RoleName:      "rb1",
				},
			},
		},
		{
			name: "no assignments for scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/foo/bar": {
						"/foo/bar": {"r1"},
					},
				}),
			},
			scope:  "/foo",
			ok:     true,
			expect: nil,
		},
		{
			name: "orthogonal resource scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":    {"r1"},
						"/foo": {"r2"},
					},
				}),
			},
			scope: "/bar",
			ok:    false,
		},
		{
			name: "parent resource scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":    {"r1"},
						"/foo": {"r2"},
					},
				}),
			},
			scope: "/",
			ok:    false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// descend assignment tree
			assignments, err := DescendAssignmentTree(tt.pin, tt.scope)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}

			var gotAssignments []RoleAssignment
			for assignment := range assignments {
				gotAssignments = append(gotAssignments, assignment)
			}

			require.Equal(t, tt.expect, gotAssignments)
		})
	}
}
