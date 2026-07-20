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

// TestTokenize pins the tokenizer. The path is never decoded before it
// is split or forwarded: the encoded separator (%2F/%2f) stays one
// opaque token in the hex case the client sent, percent-encoded
// non-ASCII content is admitted but kept raw, and every other ASCII
// escape is rejected. Safety checks run on a decode-for-validation view
// (%2F to "/"), so structure hidden behind an encoded slash is caught
// the same as a raw one.
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
			name: "path at the 8 KiB cap is admitted",
			path: "/" + strings.Repeat("a", maxPathBytes-1),
			want: []string{strings.Repeat("a", maxPathBytes-1)},
		},
		{
			name:    "path over the 8 KiB cap is rejected",
			path:    "/" + strings.Repeat("a", maxPathBytes),
			wantErr: true,
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
		{
			// "café.md": the é arrives as percent-encoded UTF-8 and is
			// kept raw in the token, since the wire bytes are forwarded
			// as sent. The fold and homoglyph reject class lives in
			// TestNonASCIIFoldCorpus; this one admit case pins the
			// token shape.
			name: "percent-encoded UTF-8 content is admitted raw",
			path: "/files/caf%C3%A9.md",
			want: []string{"files", "caf%C3%A9.md"},
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

// TestTokenizeRejectsIllegalBytes pins the RFC 3986 path-character
// gate: pchar less ";", plus "/" and "%", are admitted; anything else
// in the raw path is rejected.
func TestTokenizeRejectsIllegalBytes(t *testing.T) {
	legal := []string{
		"/api/v4/projects",
		"/api/@@@",                        // "@" is a pchar
		"/api/(group)/sub.tree",           // sub-delims and unreserved
		"/api/a:b/c,d/e=f/gh",             // ":" and sub-delims
		"/api/a-b_c~d!$&'()*+,=",          // the unreserved and sub-delim set, less ";"
		"/api/v4/projects/group%2Frepo/x", // the encoded separator, admitted
	}
	for _, p := range legal {
		t.Run("legal "+p, func(t *testing.T) {
			_, err := Tokenize(p)
			require.NoError(t, err)
		})
	}

	illegal := []string{
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
		"/foo;bar",   // semicolon, the matrix-parameter / jsessionid vector
		"/api/a;b/c", // semicolon mid-path
	}
	for _, p := range illegal {
		t.Run("illegal "+p, func(t *testing.T) {
			_, err := Tokenize(p)
			require.Error(t, err)
		})
	}
}

// TestNonASCIIFoldCorpus is the fold/homoglyph corpus for the non-ASCII path
// pipeline. The admitted entries are plain percent-encoded UTF-8 that is
// NFKC-stable and graphic; the rejected entries are the fold and homoglyph
// bypasses the pipeline must close, each a form that could land on a different
// segment once an upstream normalizes it. They are tabled together so
// the four checks (decode-once, valid UTF-8, NFKC-stable, graphic
// category) are pinned as one unit.
func TestNonASCIIFoldCorpus(t *testing.T) {
	admit := map[string]string{
		"precomposed accent (café.md)": "/files/caf%C3%A9.md",
		"CJK han character":            "/files/%E6%97%A5.txt",
		"cyrillic letter":              "/u/%D0%B4",
		"emoji is a symbol":            "/r/%F0%9F%98%80",
		"accent next to encoded slash": "/p/caf%C3%A9%2Fx",
	}
	for name, path := range admit {
		t.Run("admit "+name, func(t *testing.T) {
			_, err := Tokenize(path)
			require.NoError(t, err)
		})
	}

	reject := map[string]string{
		"raw non-ASCII byte":           "/files/caf\xc3\xa9",
		"overlong UTF-8 of slash":      "/p/a%C0%AFb",
		"lone continuation byte":       "/p/%A9",
		"truncated two-byte sequence":  "/p/%C3",
		"fullwidth solidus folds to /": "/p/a%EF%BC%8Fb",
		"fullwidth A folds to A":       "/p/%EF%BC%A1dmin",
		"fullwidth lowercase a":        "/p/%EF%BD%81dmin",
		"zero-width space is format":   "/p/a%E2%80%8Bb",
		"bidi override is format":      "/p/a%E2%80%AEb",
		"decomposed e plus accent":     "/p/cafe%CC%81",
		"ligature fi folds to fi":      "/p/o%EF%AC%81ce",
		"non-breaking space folds":     "/p/a%C2%A0b",
	}
	for name, path := range reject {
		t.Run("reject "+name, func(t *testing.T) {
			_, err := Tokenize(path)
			require.Error(t, err)
		})
	}
}

// FuzzTokenizeNonASCII asserts the safety invariant the non-ASCII
// pipeline exists to hold: every path Tokenize accepts decodes, per
// segment, to valid UTF-8 that is NFKC-stable. If a fold or homoglyph
// bypass ever slipped through, a segment that is not NFKC-stable would
// be accepted and the fuzzer would catch it. The corpus seeds the known
// bypasses; the fuzzer explores around them. The oracle decodes the
// encoded separator too, so it checks the matcher's decoded view of a
// token, not the stricter raw view the validator itself uses.
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
			content := decodeNonASCII(decodeSeparators(tok))
			valid := utf8.ValidString(content)
			require.True(t, valid, "accepted token %q decodes to invalid UTF-8", tok)
			normal := norm.NFKC.IsNormalString(content)
			require.True(t, normal, "accepted token %q is not NFKC-stable; a fold bypass slipped through", tok)
		}
		// An accepted token never carries a raw byte at or above 0x80:
		// non-ASCII must arrive percent-encoded.
		for _, tok := range tokens {
			for i := range len(tok) {
				require.Less(t, tok[i], byte(0x80), "accepted token %q has a raw non-ASCII byte", tok)
			}
		}
		// The split must round-trip: rejoining on "/" rebuilds the path
		// minus the leading slash, so tokenization never drops or
		// invents a byte.
		require.Equal(t, strings.TrimPrefix(path, "/"), strings.Join(tokens, "/"))
	})
}
