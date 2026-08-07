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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// evalPath tokenizes a request path and walks the tree, failing the test on
// a path Tokenize rejects, so each case states its path in wire form.
func evalPath(t *testing.T, root *Node, path string) (bool, map[string]string) {
	t.Helper()
	tokens, err := Tokenize(path)
	require.NoError(t, err)
	return Eval(tokens, root)
}

func TestEvalLiteralChain(t *testing.T) {
	tree := Literal("api", Literal("v4"))
	for path, want := range map[string]bool{
		"/api/v4":       true,
		"/api":          false,
		"/api/v4/extra": false,
		"/api/v5":       false,
		"/apix/v4":      false,
		"/api/v4/":      false,
	} {
		matched, _ := evalPath(t, tree, path)
		require.Equal(t, want, matched, "path %q", path)
	}
}

// TestEvalLiteralSplits pins that Literal splits on "/", so the joined and
// nested spellings build equal trees.
func TestEvalLiteralSplits(t *testing.T) {
	require.Equal(t, Literal("a", Literal("b", Literal("c"))), Literal("a/b/c"))
}

func TestEvalGlob(t *testing.T) {
	tree := Literal("files", Glob())
	matched, _ := evalPath(t, tree, "/files/report.pdf")
	require.True(t, matched)
	matched, _ = evalPath(t, tree, "/files")
	require.False(t, matched, "glob requires its one segment")
	matched, _ = evalPath(t, tree, "/files/a/b")
	require.False(t, matched, "glob matches exactly one segment")
	matched, _ = evalPath(t, tree, "/files/")
	require.False(t, matched, "glob does not match the trailing empty segment")
	matched, _ = evalPath(t, tree, "/files/group%2Fproject")
	require.False(t, matched, "glob never matches an encoded separator")
	matched, _ = evalPath(t, tree, "/files/My%20Report")
	require.True(t, matched, "an encoded space is content")
}

func TestEvalCaptureBindsContentView(t *testing.T) {
	tree := Literal("projects", Capture("project", Greedy()))
	matched, caps := evalPath(t, tree, "/projects/My%20Project/jobs")
	require.True(t, matched)
	require.Equal(t, map[string]string{"project": "My Project"}, caps)

	matched, caps = evalPath(t, tree, "/projects/caf%C3%A9")
	require.True(t, matched)
	require.Equal(t, map[string]string{"project": "café"}, caps)

	matched, _ = evalPath(t, tree, "/projects/group%2Fproject")
	require.False(t, matched, "capture never matches an encoded separator")
}

func TestEvalGreedy(t *testing.T) {
	tree := Literal("api", Greedy())
	for path, want := range map[string]bool{
		"/api":                 true,
		"/api/v4":              true,
		"/api/v4/projects/1":   true,
		"/api/v4/group%2Fproj": false,
		"/other":               false,
	} {
		matched, _ := evalPath(t, tree, path)
		require.Equal(t, want, matched, "path %q", path)
	}
}

func TestEvalSlash(t *testing.T) {
	files := Literal("files", Slash())
	matched, _ := evalPath(t, files, "/files/")
	require.True(t, matched)
	matched, _ = evalPath(t, files, "/files")
	require.False(t, matched)

	matched, _ = evalPath(t, Slash(), "/")
	require.True(t, matched, "a bare slash node matches the root path")
}

func TestEvalOptional(t *testing.T) {
	opt, err := Optional(Slash(), Literal("reports"))
	require.NoError(t, err)
	tree := Literal("files", opt)
	for path, want := range map[string]bool{
		"/files":         true,
		"/files/":        true,
		"/files/reports": true,
		"/files/other":   false,
	} {
		matched, _ := evalPath(t, tree, path)
		require.Equal(t, want, matched, "path %q", path)
	}

	_, err = Optional()
	require.ErrorContains(t, err, "at least one child")
}

func TestEvalRoot(t *testing.T) {
	root, err := Root(Literal("api", Greedy()), Literal("health"))
	require.NoError(t, err)
	for path, want := range map[string]bool{
		"/api/v4": true,
		"/health": true,
		"/admin":  false,
	} {
		matched, _ := evalPath(t, root, path)
		require.Equal(t, want, matched, "path %q", path)
	}

	_, err = Root()
	require.ErrorContains(t, err, "at least one alternative")
}

func TestEvalLiteralContentView(t *testing.T) {
	matched, _ := evalPath(t, Literal("My Project"), "/My%20Project")
	require.True(t, matched, "a literal written plain matches the encoded request form")
	matched, _ = evalPath(t, Literal("café"), "/caf%C3%A9")
	require.True(t, matched)
}

func TestValidateLiteral(t *testing.T) {
	require.NoError(t, validateLiteral("api/v4/My Project"))
	require.ErrorContains(t, validateLiteral(""), "cannot be empty")
	require.ErrorContains(t, validateLiteral("a//b"), "cannot be empty")
	require.ErrorContains(t, validateLiteral("a%2Fb"), "contains %")
	require.ErrorContains(t, validateLiteral("secret "), "space")
	require.ErrorContains(t, validateLiteral("secret."), "dot")
	require.ErrorContains(t, validateLiteral(".."), `"." or ".."`)
}

func TestLiteralPanicsOnInvalidSegment(t *testing.T) {
	require.Panics(t, func() { Literal("") })
	require.Panics(t, func() { Literal("a%2Fb") })
}

// compileMatch compiles a predicate that is expected to be valid and
// evaluates it against a GET request for the given path.
func compileMatch(t *testing.T, expr, path string) (bool, error) {
	t.Helper()
	where, err := CompileWhere(expr)
	require.NoError(t, err)
	return where.Evaluate(Env{Request: Request{Method: http.MethodGet, Path: path}})
}

func TestPathMatchPredicate(t *testing.T) {
	const expr = `path.match(root(literal("api", greedy()), literal("health", optional(slash()))))`
	for path, want := range map[string]bool{
		"/api/v4/projects": true,
		"/health":          true,
		"/health/":         true,
		"/admin":           false,
	} {
		matched, err := compileMatch(t, expr, path)
		require.NoError(t, err, "path %q", path)
		require.Equal(t, want, matched, "path %q", path)
	}
}

// TestPathMatchFailsClosed pins that an unmatchable path is an evaluation
// error rather than a false, so a negated path.match cannot invert it into
// an allow.
func TestPathMatchFailsClosed(t *testing.T) {
	for _, expr := range []string{
		`path.match(literal("api"))`,
		`!path.match(literal("api"))`,
	} {
		_, err := compileMatch(t, expr, "/a/../b")
		require.Error(t, err, "expr %q", expr)

		_, err = compileMatch(t, expr, "/group%2Fproject")
		require.ErrorContains(t, err, "encoded separator", "expr %q", expr)
	}
}

// TestPathMatchCombines pins that path.match composes with the method and
// identity conditions the where language already has.
func TestPathMatchCombines(t *testing.T) {
	where, err := CompileWhere(`path.match(literal("api", greedy())) && request.method == "GET"`)
	require.NoError(t, err)

	matched, err := where.Evaluate(Env{Request: Request{Method: http.MethodGet, Path: "/api/v4"}})
	require.NoError(t, err)
	require.True(t, matched)

	matched, err = where.Evaluate(Env{Request: Request{Method: http.MethodPost, Path: "/api/v4"}})
	require.NoError(t, err)
	require.False(t, matched)
}

// TestPathMatchTypeChecked pins that the constructors type-check at parse
// time: a string where a *Node belongs is a compile error, not a runtime
// surprise.
func TestPathMatchTypeChecked(t *testing.T) {
	_, err := CompileWhere(`path.match("admin")`)
	require.Error(t, err)
}

// TestPredicateLiteralValidates pins that the expression surface holds a
// literal to the same segment rules as the Literal constructor.
func TestPredicateLiteralValidates(t *testing.T) {
	_, err := compileMatch(t, `path.match(literal("a%2Fb"))`, "/api")
	require.ErrorContains(t, err, "contains %")
}

func TestContentView(t *testing.T) {
	for token, want := range map[string]string{
		"plain":        "plain",
		"My%20Project": "My Project",
		"caf%C3%A9":    "café",
		"group%2Fproj": "group%2Fproj",
		"group%2fproj": "group%2fproj",
		"a%20b%2Fc":    "a b%2Fc",
	} {
		require.Equal(t, want, contentView(token), "token %q", token)
	}
}

func TestHasEncodedSeparator(t *testing.T) {
	require.True(t, hasEncodedSeparator("group%2Fproj"))
	require.True(t, hasEncodedSeparator("group%2fproj"))
	require.False(t, hasEncodedSeparator("My%20Project"))
	require.False(t, hasEncodedSeparator("plain"))
	require.False(t, hasEncodedSeparator("caf%C3%A9"))
}
