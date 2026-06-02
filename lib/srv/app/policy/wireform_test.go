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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateWireform covers the path-syntax rejection rules, one
// accepted and one rejected case per rule.
func TestValidateWireform(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		accepted := []string{
			"/",
			"/foo",
			"/foo/", // trailing slash is significant, not "//"
			"/api/v4/projects/123/repository/files/README.md", // literal dots are fine
			"/files...",  // "..." is not ".."
			"/foo%20bar", // encoded space decodes to 0x20, not a control byte
			"/caf%C3%A9", // percent-encoded valid UTF-8
			"/café",      // literal valid UTF-8
		}
		for _, p := range accepted {
			t.Run(p, func(t *testing.T) {
				require.NoError(t, ValidateWireform(p))
			})
		}
	})

	t.Run("rejected", func(t *testing.T) {
		rejected := []struct {
			name string
			path string
		}{
			{"empty path", ""},
			{"rootless path", "foo"},
			{"rootless multi-segment", "foo/bar"},
			{"dotdot segment", "/foo/../etc"},
			{"dot segment", "/foo/./bar"},
			{"encoded dotdot segment", "/%2E%2E/etc"},
			{"encoded dot segment", "/foo/%2E/bar"},
			{"double slash", "/foo//bar"},
			{"leading double slash", "//foo"},
			{"trailing double slash", "/foo//"},
			{"raw control byte", "/foo\x01bar"},
			{"raw invalid utf-8", "/\x80"},
			{"encoded NUL", "/foo%00bar"},
			{"encoded DEL", "/foo%7Fbar"},
			{"raw backslash", "/foo\\bar"},
			{"encoded backslash traversal", "/foo%5C..%5Cetc"},
			{"raw semicolon matrix param", "/admin;v=1/secret"},
			{"encoded semicolon", "/admin%3Bv=1/secret"},
			{"encoded semicolon lowercase", "/admin%3bv=1/secret"},
			{"encoded slash", "/foo%2Fbar"},
			{"encoded slash lowercase", "/foo%2fbar"},
			{"encoded dot", "/foo%2Ebar"},
			{"encoded percent", "/foo%25bar"},
			{"double-encoded slash", "/foo%252Fbar"}, // begins with %25, caught
			{"invalid hex escape", "/foo%2Gbar"},
			{"truncated escape", "/foo%2"},
			{"bare percent", "/foo%"},
			{"overlong utf-8", "/api/%C0%AF/x"},
			{"encoded non-utf8 byte", "/caf%E9"}, // Latin-1 é: valid escape, not UTF-8
		}
		for _, tc := range rejected {
			t.Run(tc.name, func(t *testing.T) {
				require.Error(t, ValidateWireform(tc.path))
			})
		}
	})

	t.Run("oversize", func(t *testing.T) {
		require.NoError(t, ValidateWireform("/"+strings.Repeat("a", maxPathBytes-1)))
		require.Error(t, ValidateWireform("/"+strings.Repeat("a", maxPathBytes)))
	})
}

// FuzzValidateWireform checks the validator never panics and that a path
// it accepts is free of the structural hazards the matcher relies on:
// no "." or ".." segment and no interior empty segment.
func FuzzValidateWireform(f *testing.F) {
	for _, s := range []string{"", "/", "/foo", "/foo/bar/", "/a//b", "/%2F", "/%C0%AF", "/../x", "/foo%"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, path string) {
		if ValidateWireform(path) != nil {
			return
		}
		require.True(t, strings.HasPrefix(path, "/"), "accepted path is not rooted")
		for i, seg := range strings.Split(path, "/") {
			require.NotEqual(t, ".", seg)
			require.NotEqual(t, "..", seg)
			if i != 0 && i != strings.Count(path, "/") {
				require.NotEmpty(t, seg, "accepted path has an interior empty segment")
			}
		}
	})
}
