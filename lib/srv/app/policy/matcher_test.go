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

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMatch covers the "Pattern / Path / Match?" cases, the worked
// path-syntax examples, and capture extraction.
func TestMatch(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		want     bool
		captures map[string]string
	}{
		// The path-syntax match cases, row for row.
		{pattern: "/foo", path: "/foo", want: true},
		{pattern: "/foo", path: "/foo/", want: false},
		{pattern: "/foo", path: "/foo/bar", want: false},
		{pattern: "/foo/", path: "/foo/", want: true},
		{pattern: "/foo/*", path: "/foo", want: false},
		{pattern: "/foo/*", path: "/foo/", want: false},
		{pattern: "/foo/*", path: "/foo/bar", want: true},
		{pattern: "/foo/*", path: "/foo/bar/baz", want: false},
		{pattern: "/foo/**", path: "/foo", want: true},
		{pattern: "/foo/**", path: "/foo/", want: true},
		{pattern: "/foo/**", path: "/foo/bar", want: true},
		{pattern: "/foo/**", path: "/foo/bar/baz", want: true},
		{pattern: "/foo/{x}", path: "/foo/bar", want: true, captures: map[string]string{"x": "bar"}},
		{pattern: "/foo/{x}", path: "/foo/", want: false},

		// Exact matches.
		{pattern: "/health", path: "/health", want: true},
		{pattern: "/health", path: "/healthz", want: false},
		{pattern: "/", path: "/", want: true},
		{pattern: "/", path: "/foo", want: false},

		// Single-segment wildcard in the middle.
		{pattern: "/api/v4/projects/*", path: "/api/v4/projects/123", want: true},
		{pattern: "/api/v4/projects/*", path: "/api/v4/projects", want: false},
		{pattern: "/api/v4/projects/*", path: "/api/v4/projects/123/issues", want: false},
		{pattern: "/api/v4/projects/*/repository/**", path: "/api/v4/projects/7/repository/files/README", want: true},
		{pattern: "/api/v4/projects/*/repository/**", path: "/api/v4/projects/7/issues", want: false},

		// Greedy tail.
		{pattern: "/downloads/sdk/**", path: "/downloads/sdk", want: true},
		{pattern: "/downloads/sdk/**", path: "/downloads/sdk/cuda/12/linux.tar", want: true},
		{pattern: "/downloads/sdk/**", path: "/downloads/cudnn", want: false},

		// Root-level "**" matches any path, including "/".
		{pattern: "/**", path: "/", want: true},
		{pattern: "/**", path: "/a/b", want: true},

		// "**" after a capture, including the zero-tail boundary.
		{pattern: "/{x}/**", path: "/a", want: true, captures: map[string]string{"x": "a"}},
		{pattern: "/{x}/**", path: "/a/b/c", want: true, captures: map[string]string{"x": "a"}},
		{pattern: "/{x}/**", path: "/", want: false},

		// Captures, including multiple and a tail.
		{pattern: "/api/v4/projects/{project}", path: "/api/v4/projects/frontend", want: true, captures: map[string]string{"project": "frontend"}},
		{
			pattern:  "/api/v4/projects/{project}/merge_requests/{mr}/**",
			path:     "/api/v4/projects/9/merge_requests/42/notes",
			want:     true,
			captures: map[string]string{"project": "9", "mr": "42"},
		},

		// Byte-literal, case-sensitive.
		{pattern: "/Foo", path: "/foo", want: false},
		{pattern: "/foo", path: "/FOO", want: false},

		// The wildcard never spans a separator.
		{pattern: "/a/*/b", path: "/a/x/b", want: true},
		{pattern: "/a/*/b", path: "/a/x/y/b", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+" vs "+tc.path, func(t *testing.T) {
			m, err := Compile(tc.pattern)
			require.NoError(t, err)
			captures, ok := m.Match(tc.path)
			require.Equal(t, tc.want, ok)
			if tc.want {
				require.Equal(t, tc.captures, captures)
			} else {
				require.Nil(t, captures)
			}
		})
	}
}

// TestValidCaptureName covers the capture-identifier rules directly,
// independent of the Compile patterns that exercise them in context.
func TestValidCaptureName(t *testing.T) {
	valid := []string{"x", "abc", "a1", "user_id", "_", "_x", "A", "fooBar123"}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			require.True(t, validCaptureName(name))
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"leading digit", "1bad"},
		{"all digits", "123"},
		{"hyphen", "a-b"},
		{"dot", "a.b"},
		{"space", "a b"},
		{"non-ascii", "café"},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			require.False(t, validCaptureName(tc.input))
		})
	}
}

// TestCompile covers the patterns Compile must accept and reject.
func TestCompile(t *testing.T) {
	valid := []string{
		"/",
		"/foo",
		"/foo/",
		"/foo/bar",
		"/foo/*",
		"/foo/**",
		"/foo/{x}",
		"/a/*/b/**",
		"/api/v4/projects/{project}/merge_requests/{mr}",
		"/files...", // three dots are an ordinary literal segment
	}
	for _, p := range valid {
		t.Run("valid/"+p, func(t *testing.T) {
			_, err := Compile(p)
			require.NoError(t, err)
		})
	}

	invalid := []struct {
		pattern string
		reason  string
	}{
		{"foo", "must start with '/'"},
		{"", "must start with '/'"},
		{"/foo/**/bar", "'**' is only valid as the final segment"},
		{"/**/foo", "'**' is only valid as the final segment"},
		{"/foo/{x}/bar/{x}", "duplicate capture name"},
		{"/foo/{1bad}", "invalid capture name"},
		{"/foo/{}", "invalid capture name"},
		{"/foo/ba*r", "must occupy a whole segment"},
		{"/foo/{x}y", "must occupy a whole segment"},
		{"/foo//bar", "empty segment"},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.pattern, func(t *testing.T) {
			_, err := Compile(tc.pattern)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.reason)
		})
	}
}
