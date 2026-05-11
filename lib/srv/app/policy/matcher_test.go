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

func TestCompilePath_Errors(t *testing.T) {
	bad := []string{
		"",
		"foo",            // missing leading slash
		"/foo/**/bar",    // ** not last
		"/foo/{}/bar",    // empty capture name
		"/foo/{a/b}/bar", // bad character in capture
		"/foo/",          // trailing slash
		"/foo//bar",      // empty segment
		"/foo*bar",       // stray *
		"/foo{x}",        // stray {
	}
	for _, p := range bad {
		t.Run(p, func(t *testing.T) {
			_, err := CompilePath(p)
			require.Error(t, err, "wanted err for %q", p)
		})
	}
}

func TestPathMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
		caps    map[string]string
	}{
		{pattern: "/foo", path: "/foo", match: true},
		{pattern: "/foo", path: "/foo/bar", match: false},
		{pattern: "/foo/*", path: "/foo/bar", match: true},
		{pattern: "/foo/*", path: "/foo/bar/baz", match: false},
		{pattern: "/foo/**", path: "/foo", match: true},
		{pattern: "/foo/**", path: "/foo/bar", match: true},
		{pattern: "/foo/**", path: "/foo/bar/baz", match: true},
		{pattern: "/foo/{x}", path: "/foo/bar", match: true, caps: map[string]string{"x": "bar"}},
		{pattern: "/foo/{x}", path: "/foo/", match: false},
		{
			pattern: "/api/v4/projects/{project}/merge_requests/{mr}/notes",
			path:    "/api/v4/projects/123/merge_requests/4/notes",
			match:   true,
			caps:    map[string]string{"project": "123", "mr": "4"},
		},
		{pattern: "/", path: "/", match: true},
		{pattern: "/", path: "/foo", match: false},
		// `**` is documented as matching zero or more segments, so a
		// root-glob `/**` must include the root path itself; an empty
		// allow set against `/**` should not have a deny-by-default
		// hole at "/".
		{pattern: "/**", path: "/", match: true},
		{pattern: "/**", path: "/foo", match: true},
		{pattern: "/**", path: "/foo/bar", match: true},
		{pattern: "/foo/**", path: "/", match: false},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+"_vs_"+tc.path, func(t *testing.T) {
			m, err := CompilePath(tc.pattern)
			require.NoError(t, err)
			caps, ok := m.Match(tc.path)
			require.Equal(t, tc.match, ok)
			if tc.match && len(tc.caps) > 0 {
				require.Equal(t, tc.caps, caps)
			}
		})
	}
}

func TestPathMatch_TrailingSlashNormalizesAway(t *testing.T) {
	m := MustCompilePath("/foo")
	norm, err := Normalize("/foo/")
	require.NoError(t, err)
	_, ok := m.Match(norm)
	require.True(t, ok)
}
