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
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
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
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {"/": {"r1"}, "/foo": {"r2"}, "/foo/bar": {"r3"}},
				}),
			},
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "missing scope",
			pin: &scopesv1.Pin{
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {"/": {"r1"}},
				}),
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
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {"/": {"r1"}, "/bar": {"r2"}},
				}),
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "empty assignments",
			pin: &scopesv1.Pin{
				Scope: "/foo",
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed assignment scope",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/":             {"/": {"r1"}},
					"invalid@scope": {"invalid@scope": {"r2"}},
				}),
			},
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed pin scope",
			pin: &scopesv1.Pin{
				Scope: "invalid@scope",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {"/": {"r1"}},
				}),
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

// TestGetRolesAtEnforcementPoint verifies that GetRolesAtEnforcementPoint correctly fetches roles from the assignment tree
// at specific (ScopeOfOrigin, ScopeOfEffect) pairs.
func TestGetRolesAtEnforcementPoint(t *testing.T) {
	t.Parallel()

	// Build a test pin with a populated assignment tree
	pin := &scopesv1.Pin{
		Scope: "/staging/west",
		AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
			"/": {
				"/":             {"root-global"},
				"/staging":      {"root-staging"},
				"/staging/west": {"root-west"},
				"/staging/east": {"root-east"},
			},
			"/staging": {
				"/staging":      {"staging-staging"},
				"/staging/west": {"staging-west-a", "staging-west-b"},
			},
			"/staging/west": {
				"/staging/west": {"west-west-a", "west-west-b", "west-west-c"},
			},
		}),
	}

	tests := []struct {
		name          string
		pin           *scopesv1.Pin
		scopeOfOrigin string
		scopeOfEffect string
		expect        []string
	}{
		{
			name:          "root origin, root effect",
			pin:           pin,
			scopeOfOrigin: "/",
			scopeOfEffect: "/",
			expect:        []string{"root-global"},
		},
		{
			name:          "root origin, staging effect",
			pin:           pin,
			scopeOfOrigin: "/",
			scopeOfEffect: "/staging",
			expect:        []string{"root-staging"},
		},
		{
			name:          "root origin, west effect",
			pin:           pin,
			scopeOfOrigin: "/",
			scopeOfEffect: "/staging/west",
			expect:        []string{"root-west"},
		},
		{
			name:          "staging origin, staging effect",
			pin:           pin,
			scopeOfOrigin: "/staging",
			scopeOfEffect: "/staging",
			expect:        []string{"staging-staging"},
		},
		{
			name:          "staging origin, west effect (multiple roles)",
			pin:           pin,
			scopeOfOrigin: "/staging",
			scopeOfEffect: "/staging/west",
			expect:        []string{"staging-west-a", "staging-west-b"},
		},
		{
			name:          "west origin, west effect (multiple roles)",
			pin:           pin,
			scopeOfOrigin: "/staging/west",
			scopeOfEffect: "/staging/west",
			expect:        []string{"west-west-a", "west-west-b", "west-west-c"},
		},
		{
			name:          "no roles at this combination",
			pin:           pin,
			scopeOfOrigin: "/staging",
			scopeOfEffect: "/staging/east",
			expect:        nil,
		},
		{
			name:          "scope of origin doesn't exist",
			pin:           pin,
			scopeOfOrigin: "/nonexistent",
			scopeOfEffect: "/nonexistent",
			expect:        nil,
		},
		{
			name:          "nil pin",
			pin:           nil,
			scopeOfOrigin: "/",
			scopeOfEffect: "/",
			expect:        nil,
		},
		{
			name: "nil assignment tree",
			pin: &scopesv1.Pin{
				Scope:          "/foo",
				AssignmentTree: nil,
			},
			scopeOfOrigin: "/",
			scopeOfEffect: "/foo",
			expect:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for role := range GetRolesAtEnforcementPoint(tt.pin, scopes.EnforcementPoint{
				ScopeOfOrigin: tt.scopeOfOrigin,
				ScopeOfEffect: tt.scopeOfEffect,
			}) {
				got = append(got, role)
			}

			if tt.expect == nil {
				require.Nil(t, got, "should return empty iterator for non-existent scope combinations")
			} else {
				require.Equal(t, tt.expect, got, "roles should match expected list in sorted order")
			}
		})
	}
}

// TestGetRolesAtEnforcementPointComposition verifies that scopes.EnforcementPointsForResourceScope and
// GetRolesAtEnforcementPoint compose correctly to produce the same ordering as DescendAssignmentTree.
func TestRolesAtEnforcementPointComposition(t *testing.T) {
	t.Parallel()

	pin := &scopesv1.Pin{
		Scope: "/staging/west",
		AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
			"/": {
				"/staging/west": {"root-west"},
				"/staging":      {"root-staging"},
				"/":             {"root-root"},
			},
			"/staging": {
				"/staging/west": {"staging-west"},
				"/staging":      {"staging-staging"},
			},
			"/staging/west": {
				"/staging/west": {"west-west-a", "west-west-b"},
			},
		}),
	}

	resourceScope := "/staging/west"

	// Collect assignments using DescendAssignmentTree
	var expectedAssignments []RoleAssignment
	iter, err := DescendAssignmentTree(pin, resourceScope)
	require.NoError(t, err)
	for assignment := range iter {
		expectedAssignments = append(expectedAssignments, assignment)
	}

	// Collect assignments using EnforcementPointsForResourceScope + GetRolesAtEnforcementPoint
	var gotAssignments []RoleAssignment
	for point := range scopes.EnforcementPointsForResourceScope(resourceScope) {
		for role := range GetRolesAtEnforcementPoint(pin, point) {
			gotAssignments = append(gotAssignments, RoleAssignment{
				ScopeOfOrigin: point.ScopeOfOrigin,
				ScopeOfEffect: point.ScopeOfEffect,
				RoleName:      role,
			})
		}
	}

	require.Equal(t, expectedAssignments, gotAssignments,
		"composition of EnforcementPointsForResourceScope + GetRolesAtEnforcementPoint should produce same result as DescendAssignmentTree")
}

// TestEnumerateAllAssignments verifies that EnumerateAllAssignments correctly yields all role assignments
// in the entire assignment tree, regardless of any specific resource scope.
func TestEnumerateAllAssignments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		pin    *scopesv1.Pin
		expect []RoleAssignment
	}{
		{
			name: "empty pin",
			pin: &scopesv1.Pin{
				Scope:          "/foo",
				AssignmentTree: nil,
			},
			expect: []RoleAssignment{},
		},
		{
			name: "single assignment at root",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/": {"role1"},
					},
				}),
			},
			expect: []RoleAssignment{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/", RoleName: "role1"},
			},
		},
		{
			name: "multiple assignments at different origins",
			pin: &scopesv1.Pin{
				Scope: "/staging/west",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":             {"root-root"},
						"/staging":      {"root-staging"},
						"/staging/west": {"root-west"},
					},
					"/staging": {
						"/staging":      {"staging-staging"},
						"/staging/west": {"staging-west"},
					},
					"/staging/west": {
						"/staging/west": {"west-west"},
					},
				}),
			},
			expect: []RoleAssignment{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/", RoleName: "root-root"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging", RoleName: "root-staging"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging/west", RoleName: "root-west"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging", RoleName: "staging-staging"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging/west", RoleName: "staging-west"},
				{ScopeOfOrigin: "/staging/west", ScopeOfEffect: "/staging/west", RoleName: "west-west"},
			},
		},
		{
			name: "assignments at scopes beyond pin scope",
			pin: &scopesv1.Pin{
				Scope: "/staging",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/staging":           {"root-staging"},
						"/staging/west":      {"root-west"},
						"/staging/west/rack": {"root-rack"},
					},
					"/staging": {
						"/staging/west":      {"staging-west"},
						"/staging/west/rack": {"staging-rack"},
					},
					"/staging/west": {
						"/staging/west/rack": {"west-rack"},
					},
				}),
			},
			expect: []RoleAssignment{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging", RoleName: "root-staging"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging/west", RoleName: "root-west"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging/west/rack", RoleName: "root-rack"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging/west", RoleName: "staging-west"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging/west/rack", RoleName: "staging-rack"},
				{ScopeOfOrigin: "/staging/west", ScopeOfEffect: "/staging/west/rack", RoleName: "west-rack"},
			},
		},
		{
			name: "multiple roles at same scope combination",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/foo": {"admin", "developer", "viewer"},
					},
					"/foo": {
						"/foo": {"owner", "user"},
					},
				}),
			},
			expect: []RoleAssignment{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/foo", RoleName: "admin"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/foo", RoleName: "developer"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/foo", RoleName: "viewer"},
				{ScopeOfOrigin: "/foo", ScopeOfEffect: "/foo", RoleName: "owner"},
				{ScopeOfOrigin: "/foo", ScopeOfEffect: "/foo", RoleName: "user"},
			},
		},
		{
			name: "complex tree with multiple branches",
			pin: &scopesv1.Pin{
				Scope: "/",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":        {"global"},
						"/staging": {"staging-policy"},
						"/prod":    {"prod-policy"},
					},
					"/staging": {
						"/staging/west": {"west-admin"},
						"/staging/east": {"east-admin"},
					},
					"/prod": {
						"/prod/us": {"us-admin"},
						"/prod/eu": {"eu-admin"},
					},
				}),
			},
			expect: []RoleAssignment{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/", RoleName: "global"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/prod", RoleName: "prod-policy"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging", RoleName: "staging-policy"},
				{ScopeOfOrigin: "/prod", ScopeOfEffect: "/prod/eu", RoleName: "eu-admin"},
				{ScopeOfOrigin: "/prod", ScopeOfEffect: "/prod/us", RoleName: "us-admin"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging/east", RoleName: "east-admin"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging/west", RoleName: "west-admin"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []RoleAssignment
			for assignment := range EnumerateAllAssignments(tt.pin) {
				got = append(got, assignment)
			}

			if len(tt.expect) == 0 {
				require.Empty(t, got, "expected no assignments")
			} else {
				// Sort both slices for comparison since enumeration order is undefined
				sortAssignments := func(assignments []RoleAssignment) {
					slices.SortFunc(assignments, func(a, b RoleAssignment) int {
						if a.ScopeOfOrigin != b.ScopeOfOrigin {
							return scopes.Sort(a.ScopeOfOrigin, b.ScopeOfOrigin)
						}
						if a.ScopeOfEffect != b.ScopeOfEffect {
							return scopes.Sort(a.ScopeOfEffect, b.ScopeOfEffect)
						}
						if a.RoleName < b.RoleName {
							return -1
						} else if a.RoleName > b.RoleName {
							return 1
						}
						return 0
					})
				}

				sortAssignments(got)
				sortAssignments(tt.expect)

				require.Equal(t, tt.expect, got, "all assignments should be enumerated")
			}
		})
	}
}

// TestAssignmentTreeMapConversions verifies that AssignmentTreeFromMap and AssignmentTreeIntoMap correctly
// convert between the map and tree representations. This test validates both functions
// by ensuring that map -> tree -> map produces the same result as the original map.
func TestAssignmentTreeMapConversions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]map[string][]string
	}{
		{
			name:  "nil map",
			input: nil,
		},
		{
			name: "single role at root",
			input: map[string]map[string][]string{
				"/": {
					"/": {"role1"},
				},
			},
		},
		{
			name: "multiple roles at single scope combination",
			input: map[string]map[string][]string{
				"/foo": {
					"/foo": {"admin", "developer", "viewer"},
				},
			},
		},
		{
			name: "hierarchical assignments",
			input: map[string]map[string][]string{
				"/": {
					"/":        {"root-global"},
					"/staging": {"root-staging"},
					"/prod":    {"root-prod"},
				},
				"/staging": {
					"/staging":      {"staging-admin"},
					"/staging/west": {"staging-west"},
					"/staging/east": {"staging-east"},
				},
				"/staging/west": {
					"/staging/west": {"west-local"},
				},
			},
		},
		{
			name: "complex multi-branch tree",
			input: map[string]map[string][]string{
				"/": {
					"/":        {"global"},
					"/staging": {"staging-policy"},
					"/prod":    {"prod-policy"},
				},
				"/staging": {
					"/staging":      {"staging-base"},
					"/staging/west": {"west-admin", "west-user"},
					"/staging/east": {"east-admin", "east-user"},
				},
				"/prod": {
					"/prod":    {"prod-base"},
					"/prod/us": {"us-admin"},
					"/prod/eu": {"eu-admin", "eu-auditor"},
				},
				"/staging/west": {
					"/staging/west": {"west-dev", "west-ops"},
				},
			},
		},
		{
			name: "deep hierarchy",
			input: map[string]map[string][]string{
				"/": {
					"/":                      {"r1"},
					"/a":                     {"r2"},
					"/a/b":                   {"r3"},
					"/a/b/c":                 {"r4"},
					"/a/b/c/d":               {"r5"},
					"/a/b/c/d/e":             {"r6"},
					"/a/b/c/d/e/f":           {"r7"},
					"/a/b/c/d/e/f/g":         {"r8"},
					"/a/b/c/d/e/f/g/h":       {"r9"},
					"/a/b/c/d/e/f/g/h/i":     {"r10"},
					"/a/b/c/d/e/f/g/h/i/j":   {"r11"},
					"/a/b/c/d/e/f/g/h/i/j/k": {"r12"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := AssignmentTreeFromMap(tt.input)

			got := AssignmentTreeIntoMap(tree)

			if tt.input == nil {
				require.Nil(t, tree, "tree should be nil for nil input")
				require.Nil(t, got, "result should be nil for nil input")
				return
			}

			require.Equal(t, tt.input, got, "round-trip conversion should preserve all assignments")
		})
	}
}
