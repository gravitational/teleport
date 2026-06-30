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

// TestLowerUpper pins the lower and upper string helpers, used to compare
// case-insensitively in a where clause. lower folds a value to lower case and
// upper to upper case, so a membership test matches regardless of the request's
// case.
func TestLowerUpper(t *testing.T) {
	lowered, err := compileExpression(
		`path.match(greedy()) && contains(set("get", "head"), lower(request.method))`)
	require.NoError(t, err)

	uppered, err := compileExpression(
		`path.match(greedy()) && contains(set("GET", "HEAD"), upper(request.method))`)
	require.NoError(t, err)

	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"get", true},
		{"GeT", true},
		{"DELETE", false},
		{"delete", false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, err := lowered.Evaluate(Request{Method: tt.method, Path: "/anything"}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)

			got, err = uppered.Evaluate(Request{Method: tt.method, Path: "/anything"}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)
		})
	}
}

// TestSubstringFuncs pins has_prefix, has_suffix, and has_substring, the string
// scopers for a where clause. The main use is group-prefix scoping on an
// encoded-slash capture, such as has_prefix(vars.project, "acme/") over a
// {project:/} that binds the decoded "acme/widgets".
func TestSubstringFuncs(t *testing.T) {
	const rule = `path.match(literal("api/v4/projects", capture_encoded("project", set("/"), greedy()))) && has_prefix(vars.project, "acme/")`
	pred, err := compileExpression(rule)
	require.NoError(t, err)

	tests := []struct {
		path string
		want bool
	}{
		{"/api/v4/projects/acme%2Fwidgets/issues", true},
		{"/api/v4/projects/acme%2Fwidgets%2Fsub/issues", true},
		{"/api/v4/projects/other%2Fwidgets/issues", false},
		{"/api/v4/projects/acme/issues", false}, // a plain "acme" is not "acme/..."
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := pred.Evaluate(Request{Method: "GET", Path: tt.path}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed, tt.path)
		})
	}

	// has_suffix and has_substring evaluate the same way against a captured name.
	suffix, err := compileExpression(`path.match(literal("f", capture("name"))) && has_suffix(vars.name, ".md")`)
	require.NoError(t, err)
	got, err := suffix.Evaluate(Request{Method: "GET", Path: "/f/readme.md"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	got, err = suffix.Evaluate(Request{Method: "GET", Path: "/f/readme.txt"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)

	substr, err := compileExpression(`path.match(literal("f", capture("name"))) && has_substring(vars.name, "report")`)
	require.NoError(t, err)
	got, err = substr.Evaluate(Request{Method: "GET", Path: "/f/q3-report-final"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
}

// TestRequestPathBindingRemoved pins that request.path is no longer a binding:
// path structure is matched by path.match and conditioned on through vars
// captures, so a raw request.path read is an unknown identifier and errors,
// rather than inviting a weaker string-prefix path check. request.method
// stays, so it is not collateral damage.
func TestRequestPathBindingRemoved(t *testing.T) {
	pred, err := compileExpression(`path.match(greedy()) && has_prefix(request.path, "/api")`)
	require.NoError(t, err) // unknown identifiers resolve at evaluation
	_, err = pred.Evaluate(Request{Method: "GET", Path: "/api/v4"}, Identity{})
	require.Error(t, err, "request.path is no longer a known binding")

	// request.method still resolves and evaluates.
	ok, err := compileExpression(`path.match(greedy()) && request.method == "GET"`)
	require.NoError(t, err)
	got, err := ok.Evaluate(Request{Method: "GET", Path: "/api/v4"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
}
