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
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

// TestTokenize pins the tokenizer's accept and reject cases, including
// the opaque encoded separator and the decode-for-validation view.
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
			name: "bare root yields a single empty token",
			path: "/",
			want: []string{""},
		},
		{
			name: "path at the length cap is allowed",
			path: "/" + strings.Repeat("a", lengthCap-1),
			want: []string{strings.Repeat("a", lengthCap-1)},
		},
		{
			name: "encoded slash stays one opaque token",
			path: "/files/a%2Fb",
			want: []string{"files", "a%2Fb"},
		},
		{
			name: "lowercase encoded slash stays one raw token",
			path: "/files/a%2fb",
			want: []string{"files", "a%2fb"},
		},
		{
			name: "trailing slash yields a trailing empty token",
			path: "/files/",
			want: []string{"files", ""},
		},
		{
			name: "trailing encoded slash is allowed raw",
			path: "/files/a%2F",
			want: []string{"files", "a%2F"},
		},
		{
			// é arrives percent-encoded and stays raw in the token.
			name: "percent-encoded UTF-8 content is allowed raw",
			path: "/files/caf%C3%A9.md",
			want: []string{"files", "caf%C3%A9.md"},
		},
		{
			// %2F decodes to a real "/" before the NFKC check, so a
			// combining mark after it has no "F" to fold with and
			// accepts just like the real-slash form below.
			name: "combining mark after an encoded slash is allowed",
			path: "/p/a%2F%CC%87x",
			want: []string{"p", "a%2F%CC%87x"},
		},
		{
			name: "combining mark after a real slash is allowed",
			path: "/p/a/%CC%87x",
			want: []string{"p", "a", "%CC%87x"},
		},
		{
			name:    "path over the length cap is rejected",
			path:    "/" + strings.Repeat("a", lengthCap),
			wantErr: true,
		},
		{
			name:    "double-encoded slash is rejected because %25 decodes to %",
			path:    "/files/a%252Fb",
			wantErr: true,
		},
		{
			name:    "an ASCII escape %40 (@) is rejected",
			path:    "/files/a%40b",
			wantErr: true,
		},
		{
			name:    "an encoded dot %2E (.) is rejected",
			path:    "/files/a%2Eb",
			wantErr: true,
		},
		{
			name:    "a truncated escape is rejected",
			path:    "/files/a%2",
			wantErr: true,
		},
		{
			name:    "a malformed escape with non-hex digits is rejected",
			path:    "/files/a%G1b",
			wantErr: true,
		},
		{
			name:    "a lone percent is rejected",
			path:    "/files/a%",
			wantErr: true,
		},
		{
			name:    "a dot-dot between encoded slashes is rejected",
			path:    "/a%2F..%2Fadmin",
			wantErr: true,
		},
		{
			name:    "an empty inner part in encoded slashes is rejected",
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

// TestTokenizeByteRules pins the RFC 3986 path-character rules.
// Pchar except for ";", plus "/" and "%", are allowed. Anything else
// in the raw path is rejected.
func TestTokenizeByteRules(t *testing.T) {
	allow := []string{
		"/api/v4/projects",
		"/api/@@@",                        // "@" is a pchar
		"/api/(group)/sub.tree",           // sub-delims and unreserved
		"/api/a:b/c,d/e=f/gh",             // ":" and sub-delims
		"/api/a-b_c~d!$&'()*+,=",          // the unreserved and sub-delim set, except for ";"
		"/api/v4/projects/group%2Frepo/x", // the encoded separator, allowed
	}
	for _, p := range allow {
		t.Run("allow "+p, func(t *testing.T) {
			_, err := Tokenize(p)
			require.NoError(t, err)
		})
	}

	reject := []string{
		"/api/a b",   // space
		"/api/a\"b",  // double quote
		"/api/a<b",   // angle bracket
		"/api/a>b",   // angle bracket
		"/api/a{b}",  // braces
		"/api/a|b",   // pipe
		"/api/a^b",   // caret
		"/api/a`b",   // backtick
		"/api/a\\b",  // backslash
		"/api/a[b]",  // square brackets
		"/api/a#b",   // fragment delimiter
		"/api/a?b",   // query delimiter
		"/api/café",  // raw non-ASCII, must be percent-encoded
		"/api/a;b/c", // semicolon, the matrix-parameter / jsessionid vector
	}
	for _, p := range reject {
		t.Run("reject "+p, func(t *testing.T) {
			_, err := Tokenize(p)
			require.Error(t, err)
		})
	}
}

// TestNonASCIIFold pins the fold and homoglyph cases for the
// non-ASCII path pipeline. Rejected entries are forms that could
// resolve to a different segment once an upstream normalizes them.
func TestNonASCIIFold(t *testing.T) {
	allow := map[string]string{
		"precomposed accent (café.md)": "/files/caf%C3%A9.md",
		"CJK han character":            "/files/%E6%97%A5.txt",
		"cyrillic letter":              "/u/%D0%B4",
		"emoji is a symbol":            "/r/%F0%9F%98%80",
		"accent next to encoded slash": "/p/caf%C3%A9%2Fx",
	}
	for name, path := range allow {
		t.Run("allow "+name, func(t *testing.T) {
			_, err := Tokenize(path)
			require.NoError(t, err)
		})
	}

	reject := map[string]struct {
		path string
		// errContains pins which rule must reject inputs that more
		// than one layer would catch. When empty, any error passes.
		errContains string
	}{
		"raw non-ASCII byte":           {path: "/files/caf\xc3\xa9"},
		"overlong UTF-8 of slash":      {path: "/p/a%C0%AFb"},
		"lone continuation byte":       {path: "/p/%A9"},
		"truncated two-byte sequence":  {path: "/p/%C3"},
		"fullwidth solidus folds to /": {path: "/p/a%EF%BC%8Fb"},
		"fullwidth A folds to A":       {path: "/p/%EF%BC%A1dmin"},
		"fullwidth lowercase a":        {path: "/p/%EF%BD%81dmin"},
		"zero-width space is format":   {path: "/p/a%E2%80%8Bb"},
		"bidi override is format":      {path: "/p/a%E2%80%AEb"},
		"decomposed e plus accent":     {path: "/p/cafe%CC%81"},
		"ligature fi folds to fi":      {path: "/p/o%EF%AC%81ce"},
		"non-breaking space folds":     {path: "/p/a%C2%A0b", errContains: "not NFKC-normalized"},
	}
	for name, tt := range reject {
		t.Run("reject "+name, func(t *testing.T) {
			_, err := Tokenize(tt.path)
			require.Error(t, err)
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

// FuzzTokenizeNonASCII checks that every accepted path decodes per
// segment to valid, NFKC-stable UTF-8. Corpus seeds cover the known
// fold and homoglyph bypasses. The fuzzer explores around them.
func FuzzTokenizeNonASCII(f *testing.F) {
	for _, seed := range []string{
		"/files/caf%C3%A9.md", "/p/a%EF%BC%8Fb", "/p/%EF%BC%A1dmin",
		"/p/a%C0%AFb", "/p/a%E2%80%8Bb", "/p/cafe%CC%81", "/api/v4/x%2Fy",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, path string) {
		tokens, err := Tokenize(path)
		if err != nil {
			return
		}
		for _, tok := range tokens {
			content := decode(tok)
			valid := utf8.ValidString(content)
			require.True(t, valid, "accepted token %q decodes to invalid UTF-8", tok)
			normal := norm.NFKC.IsNormalString(content)
			require.True(t, normal, "accepted token %q is not NFKC-stable; a fold bypass slipped through", tok)
		}
		// An accepted token contains no raw non-ASCII bytes.
		for _, tok := range tokens {
			for i := range len(tok) {
				require.Less(t, tok[i], byte(0x80), "accepted token %q has a raw non-ASCII byte", tok)
			}
		}
		// Rejoining path segments roundtrips cleanly.
		require.Equal(t, path, "/"+strings.Join(tokens, "/"))
	})
}
