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

// TestPrefixMergeDesugar pins the desugared structure: paths that share a
// prefix factor it into one chain and branch only where they diverge, never
// duplicating the prefix. Paths that share no first segment stay distinct under
// a root(), and a path that is a strict prefix of another stays a distinct
// alternative, since a node cannot both end a match and continue.
func TestPrefixMergeDesugar(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{
			name:  "shared prefix factored, diverge at leaf",
			paths: []string{"/api/v4/health", "/api/v4/status"},
			want: "path.match(\n" +
				"  literal(\"api/v4\",\n" +
				"    literal(\"health\"),\n" +
				"    literal(\"status\")))",
		},
		{
			name:  "no common first segment keeps root",
			paths: []string{"/api/x", "/admin/y"},
			want: "path.match(\n" +
				"  root(\n" +
				"    literal(\"api/x\"),\n" +
				"    literal(\"admin/y\")))",
		},
		{
			name:  "prefix subset stays a distinct alternative",
			paths: []string{"/api", "/api/v4"},
			want: "path.match(\n" +
				"  root(\n" +
				"    literal(\"api\"),\n" +
				"    literal(\"api/v4\")))",
		},
		{
			name:  "three-way partial sharing",
			paths: []string{"/a/b/c", "/a/b/d", "/a/x"},
			want: "path.match(\n" +
				"  literal(\"a\",\n" +
				"    literal(\"b\",\n" +
				"      literal(\"c\"),\n" +
				"      literal(\"d\")),\n" +
				"    literal(\"x\")))",
		},
		{
			name:  "shared capture prefix factored once",
			paths: []string{"/p/{id}/issues", "/p/{id}/commits"},
			want: "path.match(\n" +
				"  literal(\"p\",\n" +
				"    capture(\"id\",\n" +
				"      literal(\"issues\"),\n" +
				"      literal(\"commits\"))))",
		},
		{
			name:  "shared glob prefix factored once",
			paths: []string{"/files/*/a", "/files/*/b"},
			want: "path.match(\n" +
				"  literal(\"files\",\n" +
				"    glob(\n" +
				"      literal(\"a\"),\n" +
				"      literal(\"b\"))))",
		},
		{
			name:  "greedy and literal as siblings under a shared prefix",
			paths: []string{"/logs/**", "/logs/app"},
			want: "path.match(\n" +
				"  literal(\"logs\",\n" +
				"    greedy(),\n" +
				"    literal(\"app\")))",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desugared, err := DesugarRoles([]Role{{Name: "r", Rules: []Rule{{Paths: tt.paths}}}})
			require.NoError(t, err)
			require.Equal(t, tt.want, desugared[0].Rules[0].Pred)
		})
	}
}

// TestPrefixMergeMatching pins that merging changes only the structure, never
// which requests match. The merged tree admits exactly the union of its paths:
// each path matches, the shared prefix alone does not unless a path ends there,
// and a sibling or deeper segment is rejected.
func TestPrefixMergeMatching(t *testing.T) {
	tests := []struct {
		name   string
		paths  []string
		probes map[string]bool
	}{
		{
			name:  "diverge at leaf",
			paths: []string{"/api/v4/health", "/api/v4/status"},
			probes: map[string]bool{
				"/api/v4/health":   true,
				"/api/v4/status":   true,
				"/api/v4":          false, // the shared prefix alone is no path
				"/api/v4/other":    false,
				"/api/v4/health/x": false, // a leaf literal is terminal
			},
		},
		{
			name:  "prefix subset, both terminal alternatives match",
			paths: []string{"/api", "/api/v4"},
			probes: map[string]bool{
				"/api":      true,
				"/api/v4":   true,
				"/api/v5":   false,
				"/api/v4/x": false,
				"/apixx":    false,
			},
		},
		{
			name:  "no common prefix",
			paths: []string{"/api/x", "/admin/y"},
			probes: map[string]bool{
				"/api/x":   true,
				"/admin/y": true,
				"/api/y":   false,
				"/other":   false,
			},
		},
		{
			name:  "three-way partial sharing",
			paths: []string{"/a/b/c", "/a/b/d", "/a/x"},
			probes: map[string]bool{
				"/a/b/c": true,
				"/a/b/d": true,
				"/a/x":   true,
				"/a/b/e": false,
				"/a/y":   false,
				"/a/b":   false,
			},
		},
		{
			name:  "shared glob prefix",
			paths: []string{"/files/*/a", "/files/*/b"},
			probes: map[string]bool{
				"/files/z/a": true,
				"/files/z/b": true,
				"/files/z/c": false,
				"/files/a":   false,
			},
		},
		{
			name:  "greedy and literal siblings",
			paths: []string{"/logs/**", "/logs/app"},
			probes: map[string]bool{
				"/logs":     true, // greedy matches zero
				"/logs/x/y": true, // greedy matches many
				"/logs/app": true,
				"/other":    false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := Rule{Paths: tt.paths}.Compile()
			require.NoError(t, err)
			for path, want := range tt.probes {
				got, err := rule.Evaluate(Request{Method: "GET", Path: path}, Identity{})
				require.NoError(t, err)
				require.Equal(t, want, got.Allowed, path)
			}
		})
	}
}

// TestPrefixMergeEqualsRootUnion pins that the merged single-match form decides
// every request identically to the unmerged form, one rule per path OR-ed in a
// role set. This is the invariant the merge must preserve: sharing a prefix is
// a structural rewrite, not a semantic change.
func TestPrefixMergeEqualsRootUnion(t *testing.T) {
	paths := []string{"/api/v4/health", "/api/v4/status", "/api/v4/projects/x", "/admin"}
	merged, err := Rule{Paths: paths}.Compile()
	require.NoError(t, err)

	unmergedRules := make([]Rule, 0, len(paths))
	for _, p := range paths {
		unmergedRules = append(unmergedRules, Rule{Paths: []string{p}})
	}
	unmerged, err := CompileRoles([]Role{{Name: "r", Rules: unmergedRules}})
	require.NoError(t, err)

	for _, path := range []string{
		"/api/v4/health", "/api/v4/status", "/api/v4/projects/x", "/admin",
		"/api/v4", "/api/v4/projects", "/api/v4/projects/x/y", "/admin/x", "/nope",
	} {
		req := Request{Method: "GET", Path: path}
		gotMerged, err := merged.Evaluate(req, Identity{})
		require.NoError(t, err)
		gotUnmerged, err := unmerged.Evaluate(req, Identity{})
		require.NoError(t, err)
		require.Equal(t, gotUnmerged.Allowed, gotMerged.Allowed, path)
	}
}

// TestPrefixMergeCaptureGuarantee pins that the capture-guarantee check stays
// correct after the merge introduces sibling children. A capture on the shared
// prefix is bound on every branch and may be read; captures that differ between
// branches are bound on only one, so reading either is a load error, even though
// the merged tree holds both under one parent.
func TestPrefixMergeCaptureGuarantee(t *testing.T) {
	// Shared capture in the factored prefix is guaranteed on every branch.
	ok := Rule{
		Paths: []string{"/p/{id}/issues", "/p/{id}/commits"},
		Where: "vars.id == user.name",
	}
	_, err := ok.Compile()
	require.NoError(t, err, "a capture on the shared prefix is bound on every branch")

	// Divergent captures under a shared literal prefix are each bound on only
	// one branch, so neither is guaranteed and reading one is a load error.
	bad := Rule{
		Paths: []string{"/api/{x}", "/api/{y}"},
		Where: "vars.x == user.name",
	}
	_, err = bad.Compile()
	require.Error(t, err, "a capture on only one merged branch is not guaranteed")
}

// TestPrefixMergeCaptureBinding pins that a capture on the shared prefix binds
// the same value on whichever branch matches.
func TestPrefixMergeCaptureBinding(t *testing.T) {
	rule, err := Rule{Paths: []string{"/p/{id}/issues", "/p/{id}/commits"}}.Compile()
	require.NoError(t, err)
	for _, path := range []string{"/p/42/issues", "/p/42/commits"} {
		got, err := rule.Evaluate(Request{Method: "GET", Path: path}, Identity{})
		require.NoError(t, err)
		require.True(t, got.Allowed, path)
		require.Equal(t, "42", got.Allow.Vars["id"], path)
	}
}
