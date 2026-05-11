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

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		code string // expected NormalizeError code, empty on success
	}{
		{name: "plain", in: "/foo/bar", want: "/foo/bar"},
		{name: "trailing slash", in: "/foo/bar/", want: "/foo/bar"},
		{name: "root", in: "/", want: "/"},
		{name: "dot segments", in: "/foo/./bar/../baz", want: "/foo/baz"},
		{name: "query stripped", in: "/foo?x=1", want: "/foo"},
		{name: "fragment stripped", in: "/foo#anchor", want: "/foo"},
		{name: "single percent decode", in: "/foo/%62ar", want: "/foo/bar"},
		{
			name: "encoded slash in segment",
			in:   "/api/foo%2Fbar",
			code: ReasonEncodedSlashInSegment,
		},
		{
			name: "double-encoded slash decodes to slash",
			in:   "/api/projects/org%252Frepo/files",
			code: ReasonEncodedSlashInSegment,
		},
		{
			name: "triple-encoded slash decodes on the 3rd round",
			in:   "/api/projects/org%25252Frepo/files",
			code: ReasonEncodedSlashInSegment,
		},
		{
			name: "quad-encoded slash still has %2F after 3 rounds",
			in:   "/api/projects/org%2525252Frepo/files",
			code: ReasonDoubleEncodedSlash,
		},
		{
			name: "mixed case encoded slash",
			in:   "/api/foo%2fbar",
			code: ReasonEncodedSlashInSegment,
		},
		{name: "nfc unicode", in: "/café", want: "/café"},

		// Bypass variants the engine must reject. Each of these reaches
		// the upstream as a different effective path, so feeding the
		// stripped form to the engine would defeat path-based deny rules.
		{name: "leading double slash", in: "//admin", code: ReasonPathDecodeFailed},
		{name: "leading triple slash", in: "///admin", code: ReasonPathDecodeFailed},
		{name: "leading slash plus host", in: "//evil.com/admin", code: ReasonPathDecodeFailed},
		{name: "internal double slash collapses", in: "/foo//bar", want: "/foo/bar"},
		{name: "trailing dot segment", in: "/admin.", code: ReasonPathDecodeFailed},
		{name: "trailing dot via percent encoded dot", in: "/admin%2e", code: ReasonPathDecodeFailed},
		{name: "middle segment ending in dot", in: "/admin./inner", code: ReasonPathDecodeFailed},
		{name: "percent-encoded backslash", in: "/admin%5cusers", code: ReasonPathDecodeFailed},
		{name: "percent-encoded backslash upper hex", in: "/admin%5Cusers", code: ReasonPathDecodeFailed},
		{name: "double percent-encoded backslash", in: "/admin%255cusers", code: ReasonPathDecodeFailed},
		{name: "semicolon matrix param", in: "/admin;foo/users", code: ReasonPathDecodeFailed},
		{name: "percent-encoded semicolon", in: "/admin%3bfoo/users", code: ReasonPathDecodeFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.in)
			if tc.code != "" {
				require.Error(t, err)
				code, ok := IsNormalizeError(err)
				require.True(t, ok, "expected NormalizeError, got %T: %v", err, err)
				require.Equal(t, tc.code, code)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
