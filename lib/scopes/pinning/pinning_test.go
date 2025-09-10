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
