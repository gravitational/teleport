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
	"net/url"
	"strings"
	"testing"
	"unicode"
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
			name: "encoded space %20 stays raw in the token",
			path: "/job/My%20Job/lastBuild",
			want: []string{"job", "My%20Job", "lastBuild"},
		},
		{
			name: "encoded space %20 in the first segment",
			path: "/My%20Jobs/config",
			want: []string{"My%20Jobs", "config"},
		},
		{
			name: "encoded space %20 in the last segment",
			path: "/job/last%20build",
			want: []string{"job", "last%20build"},
		},
		{
			name: "several encoded spaces %20 in one segment",
			path: "/job/a%20b%20c",
			want: []string{"job", "a%20b%20c"},
		},
		{
			name: "consecutive encoded spaces %20%20 are allowed",
			path: "/job/a%20%20b",
			want: []string{"job", "a%20%20b"},
		},
		{
			name: "encoded spaces %20 in several segments are allowed",
			path: "/sites/Team%20Site/Shared%20Documents/x",
			want: []string{"sites", "Team%20Site", "Shared%20Documents", "x"},
		},
		{
			name: "interior encoded space %20 next to the encoded separator %2F",
			path: "/files/a%20b%2Fc",
			want: []string{"files", "a%20b%2Fc"},
		},
		{
			name: "encoded space %20 next to a non-ASCII escape",
			path: "/files/caf%C3%A9%20menu",
			want: []string{"files", "caf%C3%A9%20menu"},
		},
		{
			name: "dots and an encoded space %20 among other characters are allowed",
			path: "/a/a.%20.b/c",
			want: []string{"a", "a.%20.b", "c"},
		},
		{
			name: "combining mark %CC%87 (U+0307) on a base character is allowed",
			path: "/p/q%CC%87x",
			want: []string{"p", "q%CC%87x"},
		},
		{
			name: "interior dots in a segment are allowed",
			path: "/a/1.2.3/b",
			want: []string{"a", "1.2.3", "b"},
		},
		{
			name: "a leading dot in a segment is allowed",
			path: "/.well-known/openid-configuration",
			want: []string{".well-known", "openid-configuration"},
		},
		{
			// The segment ends with the mark riding the space, not
			// with a space byte, so no upstream trim fires on it.
			name: "a combining mark %CC%81 on a trailing interior space %20 is allowed",
			path: "/p/x%20%CC%81",
			want: []string{"p", "x%20%CC%81"},
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
			name:    "an encoded hash %23 (#) is rejected",
			path:    "/files/a%23b",
			wantErr: true,
		},
		{
			name:    "an encoded percent %25 (%) is rejected",
			path:    "/files/a%25b",
			wantErr: true,
		},
		{
			name:    "an encoded question mark %3F (?) is rejected",
			path:    "/files/a%3Fb",
			wantErr: true,
		},
		{
			name:    "an encoded NUL %00 is rejected",
			path:    "/files/a%00b",
			wantErr: true,
		},
		{
			name:    "an encoded double quote %22 (\") is rejected",
			path:    "/files/a%22b",
			wantErr: true,
		},
		{
			name:    "an encoded less-than %3C (<) is rejected",
			path:    "/files/a%3Cb",
			wantErr: true,
		},
		{
			name:    "an encoded greater-than %3E (>) is rejected",
			path:    "/files/a%3Eb",
			wantErr: true,
		},
		{
			name:    "an encoded opening bracket %5B ([) is rejected",
			path:    "/files/a%5Bb",
			wantErr: true,
		},
		{
			name:    "an encoded closing bracket %5D (]) is rejected",
			path:    "/files/a%5Db",
			wantErr: true,
		},
		{
			name:    "an encoded backslash %5C (\\) is rejected",
			path:    "/files/a%5Cb",
			wantErr: true,
		},
		{
			name:    "an encoded caret %5E (^) is rejected",
			path:    "/files/a%5Eb",
			wantErr: true,
		},
		{
			name:    "an encoded backtick %60 (`) is rejected",
			path:    "/files/a%60b",
			wantErr: true,
		},
		{
			name:    "an encoded opening brace %7B ({) is rejected",
			path:    "/files/a%7Bb",
			wantErr: true,
		},
		{
			name:    "an encoded pipe %7C (|) is rejected",
			path:    "/files/a%7Cb",
			wantErr: true,
		},
		{
			name:    "an encoded closing brace %7D (}) is rejected",
			path:    "/files/a%7Db",
			wantErr: true,
		},
		{
			name:    "a segment of only the encoded space %20 is rejected",
			path:    "/a/%20/b",
			wantErr: true,
		},
		{
			name:    "a leading encoded space %20 in a segment is rejected",
			path:    "/a/%20x",
			wantErr: true,
		},
		{
			name:    "a trailing encoded space %20 in a segment is rejected",
			path:    "/a/x%20",
			wantErr: true,
		},
		{
			// The segment decodes to ".. ", which trims to "..".
			name:    "a dot-dot with a trailing encoded space %20 is rejected",
			path:    "/a/..%20/b",
			wantErr: true,
		},
		{
			// The segment decodes to ". ", which trims to ".".
			name:    "a dot with a trailing encoded space %20 is rejected",
			path:    "/a/.%20/b",
			wantErr: true,
		},
		{
			name:    "a dot-dot with a leading encoded space %20 is rejected",
			path:    "/a/%20../b",
			wantErr: true,
		},
		{
			name:    "a leading encoded space %20 after an encoded slash %2F is rejected",
			path:    "/p/a%2F%20b/c",
			wantErr: true,
		},
		{
			name:    "a trailing encoded space %20 before an encoded slash %2F is rejected",
			path:    "/p/b%20%2Fc/d",
			wantErr: true,
		},
		{
			// The segment starts with a space that carries the mark.
			name:    "a combining mark %CC%81 (U+0301) on a leading encoded space %20 is rejected",
			path:    "/p/%20%CC%81x",
			wantErr: true,
		},
		{
			// The segment decodes to ". .", stripped of spaces "..".
			name:    "an encoded space %20 between dots is rejected",
			path:    "/a/.%20./b",
			wantErr: true,
		},
		{
			// The dot-segment rule fires; the segment also violates
			// the edge-space rule.
			name:    "a segment of alternating %20 and dots is rejected",
			path:    "/a/%20.%20.%20/b",
			wantErr: true,
		},
		{
			name:    "a dot-dot with %20 and a trailing dot is rejected",
			path:    "/a/..%20./b",
			wantErr: true,
		},
		{
			name:    "dots separated by encoded spaces %20 are rejected",
			path:    "/a/.%20.%20./b",
			wantErr: true,
		},
		{
			name:    "dots and %20 between encoded slashes %2F are rejected",
			path:    "/p/a%2F.%20.%2Fb",
			wantErr: true,
		},
		{
			name:    "a segment of only dots is rejected",
			path:    "/a/.../b",
			wantErr: true,
		},
		{
			name:    "a trailing dot in a segment is rejected",
			path:    "/files/secret./x",
			wantErr: true,
		},
		{
			name:    "trailing dots in a segment are rejected",
			path:    "/files/secret../x",
			wantErr: true,
		},
		{
			name:    "a trailing encoded space %20 and dot in a segment are rejected",
			path:    "/files/secret%20./x",
			wantErr: true,
		},
		{
			name:    "a trailing dot before an encoded slash %2F is rejected",
			path:    "/p/a.%2Fb",
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
			name:    "a segment starting with the combining mark %CC%87 is rejected",
			path:    "/p/%CC%87x",
			wantErr: true,
		},
		{
			name:    "a combining mark after an encoded slash %2F is rejected",
			path:    "/p/a%2F%CC%87x",
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
		"/api/a b",   // raw space, allowed only encoded as %20
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
		"raw non-ASCII byte":                                 {path: "/files/caf\xc3\xa9"},
		"overlong UTF-8 of slash":                            {path: "/p/a%C0%AFb"},
		"lone continuation byte":                             {path: "/p/%A9"},
		"truncated two-byte sequence":                        {path: "/p/%C3"},
		"fullwidth solidus folds to /":                       {path: "/p/a%EF%BC%8Fb"},
		"fullwidth A folds to A":                             {path: "/p/%EF%BC%A1dmin"},
		"fullwidth lowercase a":                              {path: "/p/%EF%BD%81dmin"},
		"zero-width space is format":                         {path: "/p/a%E2%80%8Bb"},
		"bidi override is format":                            {path: "/p/a%E2%80%AEb"},
		"decomposed e plus accent":                           {path: "/p/cafe%CC%81"},
		"ligature fi folds to fi":                            {path: "/p/o%EF%AC%81ce"},
		"non-breaking space folds":                           {path: "/p/a%C2%A0b", errContains: "not NFKC-normalized"},
		"en quad %E2%80%80 (U+2000) folds to space":          {path: "/p/a%E2%80%80b", errContains: "not NFKC-normalized"},
		"ideographic space %E3%80%80 (U+3000) folds":         {path: "/p/a%E3%80%80b", errContains: "not NFKC-normalized"},
		"ogham space mark %E1%9A%80 (U+1680) is a separator": {path: "/p/a%E1%9A%80b", errContains: "disallowed character"},
		"line separator %E2%80%A8 (U+2028) is a separator":   {path: "/p/a%E2%80%A8b", errContains: "disallowed character"},
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
		"/job/My%20Job/lastBuild", "/p/%20%CC%81x", "/p/a%C2%A0b",
		"/a/..%20/b", "/a/%20/b", "/a/.%20./b", "/a/.../b",
		"/files/secret./x", "/files/secret%20./x",
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
			for _, part := range strings.Split(content, "/") {
				if part == "" {
					continue
				}
				r, _ := utf8.DecodeRuneInString(part)
				require.False(t, unicode.IsMark(r), "accepted token %q has a part starting with the combining mark %q", tok, string(r))
				require.False(t, strings.HasPrefix(part, " "), "accepted token %q has a part starting with a space", tok)
				require.False(t, strings.HasSuffix(part, " "), "accepted token %q has a part ending with a space", tok)
				dotSpaces := strings.Trim(part, ". ") == "" && strings.Contains(part, ".")
				require.False(t, dotSpaces, "accepted token %q has a part of only dots and spaces", tok)
				tail := part[len(strings.TrimRight(part, ". ")):]
				require.NotContains(t, tail, ".", "accepted token %q has a part ending with dots and spaces", tok)
			}
		}
		// An accepted token contains no raw non-ASCII bytes.
		for _, tok := range tokens {
			for i := range len(tok) {
				require.Less(t, tok[i], byte(0x80), "accepted token %q has a raw non-ASCII byte", tok)
			}
		}
		// Rejoining path segments roundtrips cleanly.
		require.Equal(t, path, "/"+strings.Join(tokens, "/"))
		// An accepted path is its own escaped form.
		u, err := url.ParseRequestURI(path)
		require.NoError(t, err, "accepted path does not parse: %q", path)
		require.Equal(t, path, u.EscapedPath(), "accepted path is not its own escaped form")
	})
}
