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

// TestTokenizeDecodeIterations pins how allow_percent and decode_iterations
// interact. The default rejects any percent byte. allow_percent admits the
// bytes; decode_iterations then sets how many decode passes run before the
// path is split, which is how a double-encoded slash (%252F) collapses to a
// separator. The author must match the iteration count to the upstream's own
// decoding, or the matcher's view diverges from what the upstream acts on.
func TestTokenizeDecodeIterations(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		cfg     DecodeConfig
		want    []string
		wantErr bool
	}{
		{
			name:    "strict default rejects percent",
			path:    "/a/b%2Fc",
			cfg:     DecodeConfig{},
			wantErr: true,
		},
		{
			name: "allow_percent, zero iterations keeps the encoded byte",
			path: "/a/b%2Fc",
			cfg:  DecodeConfig{AllowPercent: true, DecodeIterations: 0},
			want: []string{"a", "b%2Fc"},
		},
		{
			name: "one iteration decodes a single-encoded slash into a separator",
			path: "/a/b%2Fc",
			cfg:  DecodeConfig{AllowPercent: true, DecodeIterations: 1},
			want: []string{"a", "b", "c"},
		},
		{
			name: "three iterations collapse a double-encoded slash",
			path: "/a/b%252Fc/d",
			cfg:  DecodeConfig{AllowPercent: true, DecodeIterations: 3},
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "three iterations leave an unencoded path unchanged",
			path: "/a/plain/d",
			cfg:  DecodeConfig{AllowPercent: true, DecodeIterations: 3},
			want: []string{"a", "plain", "d"},
		},
		{
			// allow_percent is the residual check after decoding: one pass
			// fully unwinds a single-encoded slash, so nothing is left to
			// reject.
			name: "one iteration, no allow_percent, single-encoded is admitted",
			path: "/a/b%2Fc",
			cfg:  DecodeConfig{AllowPercent: false, DecodeIterations: 1},
			want: []string{"a", "b", "c"},
		},
		{
			// One pass leaves %2F behind from a double-encoded slash; with
			// allow_percent false that residual is rejected.
			name:    "one iteration, no allow_percent, double-encoded is rejected",
			path:    "/a/b%252Fc",
			cfg:     DecodeConfig{AllowPercent: false, DecodeIterations: 1},
			wantErr: true,
		},
		{
			// Two passes fully unwind the double-encoded slash, so the same
			// strict residual policy now admits it.
			name: "two iterations, no allow_percent, double-encoded is admitted",
			path: "/a/b%252Fc",
			cfg:  DecodeConfig{AllowPercent: false, DecodeIterations: 2},
			want: []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tokenize(tt.path, tt.cfg)
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
		"/api/v4/projects/group%2Frepo/x", // percent-encoded, admitted here
	}
	for _, p := range legal {
		t.Run("legal "+p, func(t *testing.T) {
			_, err := Tokenize(p, DecodeConfig{AllowPercent: true})
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
			_, err := Tokenize(p, DecodeConfig{AllowPercent: true})
			require.Error(t, err)
		})
	}
}

// TestDecodeIterationsChangesMatch pins the end-to-end effect through a rule:
// the same request and pattern match or not depending on the iteration count,
// because the count decides whether an encoded slash is one segment or two.
func TestDecodeIterationsChangesMatch(t *testing.T) {
	const req = "/api/v4/projects/group%252Frepo/issues"

	// With three decode passes, %252F collapses to a separator, so the project
	// segment splits and {project} binds only the first part.
	collapsed := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/{repo}/issues"]
url_decoding:
  allow_percent: true
  decode_iterations: 3
`)
	c, err := collapsed.Compile()
	require.NoError(t, err)
	got, err := c.Evaluate(Request{Method: "GET", Path: req}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "group", got.Allow.Vars["project"])
	require.Equal(t, "repo", got.Allow.Vars["repo"])

	// With no decode pass, the encoded id stays one segment, so {project}
	// binds the whole encoded value and the two-segment pattern no longer
	// matches.
	kept := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/issues"]
url_decoding:
  allow_percent: true
  decode_iterations: 0
`)
	ck, err := kept.Compile()
	require.NoError(t, err)
	gotKept, err := ck.Evaluate(Request{Method: "GET", Path: req}, Identity{})
	require.NoError(t, err)
	require.True(t, gotKept.Allowed)
	require.Equal(t, "group%252Frepo", gotKept.Allow.Vars["project"])
}
