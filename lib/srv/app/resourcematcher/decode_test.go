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

// TestTokenize pins the strict, separator-only tokenizer. The path is never
// decoded before it is split or forwarded: the only encoding admitted is the
// encoded separator (%2F/%2f), which stays one opaque token, and every other
// percent-escape is rejected. Safety checks run on a decode-for-validation view
// (%2F to "/"), so a "." or ".." or "//" hidden behind an encoded slash is
// caught the same as a raw one.
func TestTokenize(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    []string
		wantErr bool
	}{
		{
			name: "plain path splits on real slashes",
			path: "/api/v4/projects",
			want: []string{"api", "v4", "projects"},
		},
		{
			name: "encoded slash stays one opaque token",
			path: "/files/a%2Fb",
			want: []string{"files", "a%2Fb"},
		},
		{
			name: "lowercase encoded slash stays one token",
			path: "/files/a%2fb",
			want: []string{"files", "a%2fb"},
		},
		{
			name: "multiple encoded slashes stay one token",
			path: "/api/v4/projects/mygroup%2Fmyproject/issues",
			want: []string{"api", "v4", "projects", "mygroup%2Fmyproject", "issues"},
		},
		{
			name: "trailing slash yields a trailing empty token",
			path: "/files/",
			want: []string{"files", ""},
		},
		{
			// A trailing encoded slash decodes to a harmless trailing slash on
			// the validation view, and forwards raw as one token.
			name: "trailing encoded slash is admitted raw",
			path: "/files/a%2F",
			want: []string{"files", "a%2F"},
		},
		{
			name:    "double-encoded slash is rejected: %25 is not the separator",
			path:    "/files/a%252Fb",
			wantErr: true,
		},
		{
			name:    "a non-separator escape is rejected",
			path:    "/files/a%40b",
			wantErr: true,
		},
		{
			name:    "an encoded dot is rejected",
			path:    "/files/a%2Eb",
			wantErr: true,
		},
		{
			name:    "a truncated escape is rejected",
			path:    "/files/a%2",
			wantErr: true,
		},
		{
			name:    "a lone percent is rejected",
			path:    "/files/a%",
			wantErr: true,
		},
		{
			name:    "a dot-dot smuggled between encoded slashes is rejected",
			path:    "/a%2F..%2Fadmin",
			wantErr: true,
		},
		{
			name:    "an empty inner part smuggled in encoded slashes is rejected",
			path:    "/a%2F%2Fb",
			wantErr: true,
		},
		{
			name:    "a raw dot-dot segment is rejected",
			path:    "/api/v4/../secret",
			wantErr: true,
		},
		{
			name:    "a raw single-dot segment is rejected",
			path:    "/api/./v4",
			wantErr: true,
		},
		{
			name:    "consecutive slashes are rejected",
			path:    "/api//v4",
			wantErr: true,
		},
		{
			name:    "a path without a leading slash is rejected",
			path:    "api/v4",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tokenize(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestRejectIllegalPathBytes pins the RFC 3986 path-character gate: pchar,
// "/", and "%" are admitted; anything else in the raw path is rejected.
func TestRejectIllegalPathBytes(t *testing.T) {
	legal := []string{
		"/api/v4/projects",
		"/api/@@@",                        // "@" is a pchar
		"/api/(group)/sub.tree",           // sub-delims and unreserved
		"/api/a:b/c,d/e=f/g;h",            // ":" and sub-delims
		"/api/a-b_c~d!$&'()*+,;=",         // the full unreserved and sub-delim set
		"/api/v4/projects/group%2Frepo/x", // the encoded separator, admitted
	}
	for _, p := range legal {
		t.Run("legal "+p, func(t *testing.T) {
			_, err := Tokenize(p)
			require.NoError(t, err)
		})
	}

	illegal := []string{
		"/api/a b",  // space
		"/api/a\"b", // double quote
		"/api/a<b",  // angle bracket
		"/api/a>b",  // angle bracket
		"/api/a{b}", // braces
		"/api/a|b",  // pipe
		"/api/a^b",  // caret
		"/api/a`b",  // backtick
		"/api/a\\b", // backslash
		"/api/a[b]", // square brackets
		"/api/a#b",  // fragment delimiter
		"/api/a?b",  // query delimiter
		"/api/café", // raw non-ASCII, must be percent-encoded
	}
	for _, p := range illegal {
		t.Run("illegal "+p, func(t *testing.T) {
			_, err := Tokenize(p)
			require.Error(t, err)
		})
	}
}
