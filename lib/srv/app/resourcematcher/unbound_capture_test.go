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

package resourcematcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCaptureUnboundInOneBranchIsLoadError pins what happens when a where
// clause reads a capture that only one branch of a root() binds. The rule
// grants two paths through one path.match: api/v4/projects/{project}/... binds
// project, while api/v2/... binds nothing. The where reads vars.project.
//
// The engine rejects this at load, not at evaluation: a vars.<name> read is
// sound only where every matching path binds the name, and the api/v2 branch
// does not. So the capture cannot "fire on some branches only"; the rule never
// compiles. This is stricter and earlier than a runtime fail-closed, and it
// surfaces the mistake to the rule author at load.
func TestCaptureUnboundInOneBranchIsLoadError(t *testing.T) {
	_, err := compileExpression(`path.match(root(
			literal("api/v4/projects", capture("project", greedy())),
			literal("api/v2", greedy()))) &&
			contains(user.traits["allowed_projects"], vars.project)`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not every path in the rule binds")
}

// TestCaptureBoundInEveryBranch is the sound counterpart: when every branch of
// the root() binds project, the vars.project read passes load-time validation
// and the rule evaluates per branch. The capture carries the segment the
// matching branch bound, whichever branch that was.
func TestCaptureBoundInEveryBranch(t *testing.T) {
	compiled, err := compileExpression(`path.match(root(
			literal("api/v4/projects", capture("project", greedy())),
			literal("api/v2", capture("project", greedy())))) &&
			contains(user.traits["allowed_projects"], vars.project)`)
	require.NoError(t, err)

	identity := Identity{Traits: map[string][]string{
		"allowed_projects": {"allowed-one"},
	}}

	t.Run("first branch binds project", func(t *testing.T) {
		got, err := compiled.Evaluate(
			Request{Method: "GET", Path: "/api/v4/projects/allowed-one/tree"},
			identity)
		require.NoError(t, err)
		require.True(t, got.Allowed)
		require.Equal(t, "allowed-one", got.Allow.Vars["project"])
	})

	t.Run("second branch binds project", func(t *testing.T) {
		got, err := compiled.Evaluate(
			Request{Method: "GET", Path: "/api/v2/allowed-one"},
			identity)
		require.NoError(t, err)
		require.True(t, got.Allowed)
		require.Equal(t, "allowed-one", got.Allow.Vars["project"])
	})

	t.Run("a trait that does not list the project is denied", func(t *testing.T) {
		got, err := compiled.Evaluate(
			Request{Method: "GET", Path: "/api/v4/projects/secret/tree"},
			identity)
		require.NoError(t, err)
		require.False(t, got.Allowed)
	})
}
