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
	"testing"

	"github.com/stretchr/testify/require"
)

// evalPath compiles the tree, tokenizes a request path, and walks it, failing
// the test on a tree Compile rejects or a path Tokenize rejects, so each case
// states its path in wire form.
func evalPath(t *testing.T, root Node, path string) (bool, map[string]string) {
	t.Helper()
	pattern, err := Compile(root)
	require.NoError(t, err)
	tokens, err := Tokenize(path)
	require.NoError(t, err)
	return Match(pattern, tokens)
}

func TestMatchLiteralChain(t *testing.T) {
	tree := Literal("api", Literal("v4"))
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v4", true},
		{"/api", false},
		{"/api/v4/extra", false},
		{"/api/v5", false},
		{"/apix/v4", false},
		{"/api/v4/", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matched, captures := evalPath(t, tree, tt.path)
			require.Equal(t, tt.want, matched)
			if !tt.want {
				require.Nil(t, captures, "no match returns a nil map")
			}
		})
	}
}

// TestMatchLiteralSplits checks that Literal splits on "/", so the joined and
// nested spellings create equal trees.
func TestMatchLiteralSplits(t *testing.T) {
	require.Equal(t, Literal("a", Literal("b", Literal("c"))), Literal("a/b/c"))
}

func TestMatchGlob(t *testing.T) {
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

func TestMatchGlobWithout(t *testing.T) {
	tree := Literal("files", GlobWithout([]string{"secret", "my x"}, Greedy()))
	tests := []struct {
		path string
		want bool
	}{
		{"/files/public/x", true},
		{"/files/secret/x", false},
		{"/files/Secret/x", true},
		{"/files/secret", false},
		{"/files/my%20x/y", false},
		{"/files/secrets/x", true},
		{"/files", false},
		{"/files/", false},
		{"/files/group%2Fproject", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matched, _ := evalPath(t, tree, tt.path)
			require.Equal(t, tt.want, matched)
		})
	}
}

func TestMatchCaptureBindsDecodedContent(t *testing.T) {
	tree := Literal("projects", Capture("project", Greedy()))
	matched, captures := evalPath(t, tree, "/projects/My%20Project/jobs")
	require.True(t, matched)
	require.Equal(t, map[string]string{"project": "My Project"}, captures)

	matched, captures = evalPath(t, tree, "/projects/caf%C3%A9")
	require.True(t, matched)
	require.Equal(t, map[string]string{"project": "café"}, captures)

	matched, _ = evalPath(t, tree, "/projects/group%2Fproject")
	require.False(t, matched, "capture never matches an encoded separator")

	matched, _ = evalPath(t, tree, "/projects/")
	require.False(t, matched, "capture does not match the trailing empty segment")
}

// TestMatchCaptureUndoneOnFailedBranch checks that a losing alternative leaves
// no binding: matching "/docs/intro" against /{project}/settings or
// /docs/{page} binds only page, never project.
func TestMatchCaptureUndoneOnFailedBranch(t *testing.T) {
	root := Root(Capture("project", Literal("settings")), Literal("docs", Capture("page")))
	matched, captures := evalPath(t, root, "/docs/intro")
	require.True(t, matched)
	require.Equal(t, map[string]string{"page": "intro"}, captures)
}

// TestMatchNilPattern checks that a nil pattern is a no-match, not a panic.
func TestMatchNilPattern(t *testing.T) {
	matched, captures := Match(nil, nil)
	require.False(t, matched)
	require.Nil(t, captures)
}

// TestMatchEmptyTokens checks that an empty tokens slice never matches, even
// for a pattern rooted at Greedy.
func TestMatchEmptyTokens(t *testing.T) {
	pattern, err := Compile(Greedy())
	require.NoError(t, err)
	matched, captures := Match(pattern, nil)
	require.False(t, matched)
	require.Nil(t, captures)
}

func TestMatchGreedy(t *testing.T) {
	tree := Literal("api", Greedy())
	tests := []struct {
		path string
		want bool
	}{
		{"/api", true},
		{"/api/", true},
		{"/api/v4", true},
		{"/api/v4/", true},
		{"/api/v4/projects/1", true},
		{"/api/v4/group%2Fproj", false},
		{"/other", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matched, _ := evalPath(t, tree, tt.path)
			require.Equal(t, tt.want, matched)
		})
	}
}

func TestMatchSlash(t *testing.T) {
	files := Literal("files", Slash())
	matched, _ := evalPath(t, files, "/files/")
	require.True(t, matched)
	matched, _ = evalPath(t, files, "/files")
	require.False(t, matched)

	matched, _ = evalPath(t, Slash(), "/")
	require.True(t, matched, "a bare slash node matches the root path")
}

func TestMatchOptional(t *testing.T) {
	tree := Literal("files", Optional(Slash(), Literal("reports")))
	tests := []struct {
		path string
		want bool
	}{
		{"/files", true},
		{"/files/", true},
		{"/files/reports", true},
		{"/files/other", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matched, _ := evalPath(t, tree, tt.path)
			require.Equal(t, tt.want, matched)
		})
	}
}

func TestMatchRoot(t *testing.T) {
	root := Root(Literal("api", Greedy()), Literal("health"))
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v4", true},
		{"/health", true},
		{"/admin", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matched, _ := evalPath(t, root, tt.path)
			require.Equal(t, tt.want, matched)
		})
	}
}

func TestMatchLiteralDecodedContent(t *testing.T) {
	matched, _ := evalPath(t, Literal("My Project"), "/My%20Project")
	require.True(t, matched, "a literal written plain matches the encoded request form")
	matched, _ = evalPath(t, Literal("café"), "/caf%C3%A9")
	require.True(t, matched)
}

func TestValidateSegment(t *testing.T) {
	require.NoError(t, validateSegment("My Project"))
	require.ErrorContains(t, validateSegment(""), "cannot be empty")
	require.ErrorContains(t, validateSegment("a%2Fb"), "contains %")
	require.ErrorContains(t, validateSegment("secret "), "space")
	require.ErrorContains(t, validateSegment("secret."), "dot")
	require.ErrorContains(t, validateSegment(".."), `"." or ".."`)
	require.NoError(t, validateSegment("*"))
	require.ErrorContains(t, validateSegment("cafe\u0301"), "NFKC")
	require.ErrorContains(t, validateSegment("a<b"), "illegal URL byte")
	require.ErrorContains(t, validateSegment("a\u200bb"), "disallowed character")
}

func TestCompileValidTree(t *testing.T) {
	pattern, err := Compile(Root(Literal("api", Capture("version", Greedy())), Literal("health", Optional(Slash()))))
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// The same capture name in sibling alternatives is valid.
	pattern, err = Compile(Root(Capture("v", Literal("a")), Capture("v", Literal("b"))))
	require.NoError(t, err)
	require.NotNil(t, pattern)
}

// TestCompileErrors checks that the Compile error names the offending node by
// its path from the root.
func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name    string
		root    Node
		wantErr string
	}{
		{
			name:    "empty literal segment",
			root:    Literal(""),
			wantErr: `literal(""): a literal segment cannot be empty`,
		},
		{
			name:    "escape in literal",
			root:    Literal("a%2Fb"),
			wantErr: `literal("a%2Fb"): literal segment "a%2Fb" contains %`,
		},
		{
			name:    "double slash in literal",
			root:    Literal("a//b"),
			wantErr: `literal("a//b"): a literal segment cannot be empty`,
		},
		{
			name:    "leading slash in literal",
			root:    Literal("/api"),
			wantErr: `literal("/api"): a literal segment cannot be empty`,
		},
		{
			name:    "trailing slash in literal",
			root:    Literal("api/"),
			wantErr: `literal("api/"): a literal segment cannot be empty`,
		},
		{
			name:    "empty exclude",
			root:    Literal("files", GlobWithout([]string{""})),
			wantErr: `literal("files") > glob_without(set("")): an excluded segment cannot be empty`,
		},
		{
			name:    "slash in exclude",
			root:    GlobWithout([]string{"a/b"}),
			wantErr: `glob_without(set("a/b")): excluded segment "a/b" cannot contain /`,
		},
		{
			name:    "empty optional",
			root:    Literal("files", Optional()),
			wantErr: `literal("files") > optional(): optional requires at least one child subtree`,
		},
		{
			name:    "empty root",
			root:    Root(),
			wantErr: `root(): root requires at least one alternative`,
		},
		{
			name:    "nested failure keeps the path",
			root:    Root(Literal("api", Greedy()), Literal("health", Optional())),
			wantErr: `root() > literal("health") > optional(): optional requires at least one child subtree`,
		},
		{
			name:    "failure under a capture",
			root:    Capture("v", Optional()),
			wantErr: `capture("v") > optional(): optional requires at least one child subtree`,
		},
		{
			name:    "failure under a glob",
			root:    Glob(Optional()),
			wantErr: `glob() > optional(): optional requires at least one child subtree`,
		},
		{
			name:    "capture bound twice on one branch",
			root:    Capture("v", Literal("x", Capture("v", Greedy()))),
			wantErr: `capture("v") > literal("x") > capture("v"): capture "v" is already bound on this branch`,
		},
		{
			name:    "nil child",
			root:    Literal("api", nil),
			wantErr: `literal("api") has a nil child`,
		},
		{
			name:    "nil tree",
			root:    nil,
			wantErr: "cannot compile a nil tree",
		},
		{
			name:    "empty capture name",
			root:    Capture(""),
			wantErr: `capture(""): capture name "" must be a letter or underscore`,
		},
		{
			name:    "capture name with slash",
			root:    Literal("api", Capture("a/b")),
			wantErr: `literal("api") > capture("a/b"): capture name "a/b" must be a letter`,
		},
		{
			name:    "capture name starting with a digit",
			root:    Capture("1x"),
			wantErr: `capture("1x"): capture name "1x" must be a letter or underscore`,
		},
		{
			name:    "root below the top",
			root:    Literal("api", Root(Literal("v1"))),
			wantErr: `literal("api") > root() must be the top node`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := Compile(tt.root)
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, pattern)
		})
	}
}

// TestCompileRejectsSharedSubtree checks that one subtree under two parents is
// an error naming the second position, while a shared leaf, which cannot make
// evaluation exponential, is accepted.
func TestCompileRejectsSharedSubtree(t *testing.T) {
	shared := Literal("v1", Greedy())
	_, err := Compile(Root(Literal("api", shared), Literal("admin", shared)))
	require.ErrorContains(t, err, `root() > literal("admin") > literal("v1") appears twice in the tree`)

	leaf := Greedy()
	_, err = Compile(Root(Literal("api", leaf), Literal("admin", leaf)))
	require.NoError(t, err)
}

// TestConstructorsCopyChildren checks that a constructor does not retain the
// caller's slice, so changing that slice after Compile cannot swap a branch of
// an approved pattern or splice a cycle into it.
func TestConstructorsCopyChildren(t *testing.T) {
	childNodes := []Node{Literal("private")}
	root := Root(childNodes...)
	pattern, err := Compile(root)
	require.NoError(t, err)

	childNodes[0] = root

	matched, _ := Match(pattern, []Token{{Raw: "private", Decoded: "private"}})
	require.True(t, matched)
	matched, _ = Match(pattern, []Token{{Raw: "public", Decoded: "public"}})
	require.False(t, matched)
}

func TestDecode(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{"plain", "plain"},
		{"My%20Project", "My Project"},
		{"caf%C3%A9", "café"},
		{"group%2Fproj", "group/proj"},
		{"group%2fproj", "group/proj"},
		{"a%20b%2Fc", "a b/c"},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			require.Equal(t, tt.want, decode(tt.token))
		})
	}
}

func TestHasEncodedSlash(t *testing.T) {
	tok := func(raw string) Token { return Token{Raw: raw, Decoded: decode(raw)} }
	require.True(t, tok("group%2Fproj").hasEncodedSlash())
	require.True(t, tok("group%2fproj").hasEncodedSlash())
	require.False(t, tok("My%20Project").hasEncodedSlash())
	require.False(t, tok("plain").hasEncodedSlash())
	require.False(t, tok("caf%C3%A9").hasEncodedSlash())
}
