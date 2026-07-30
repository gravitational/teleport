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
	"fmt"
	"net/url"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
	"pgregory.net/rapid"
)

// stableMarkPairs are base characters followed by a combining mark
// that has no pre-composed form with them, so each pair survives NFKC
// and gives the generator a mark where a mark is allowed.
var stableMarkPairs = []string{
	"q̇",           // U+0071 U+0307, latin q, combining dot above
	"v́",           // U+0076 U+0301, latin v, combining acute
	"j̣",           // U+006A U+0323, latin j, combining dot below
	"α̈",           // U+03B1 U+0308, greek alpha, combining diaeresis
	"б́",           // U+0431 U+0301, cyrillic be, combining acute
	"का",           // U+0915 U+093E, devanagari ka, vowel sign aa, a spacing mark
	"ก่",           // U+0E01 U+0E48, thai ko kai, mai ek
	"あ゙",           // U+3042 U+3099, hiragana a, combining voiced sound mark
	"\u05d0\u05b7", // U+05D0 U+05B7, hebrew alef, point patah, escaped because RTL reorders the line
	"\u0627\u064e", // U+0627 U+064E, arabic alef, fatha, escaped because RTL reorders the line
}

// safePunct is legalPathPunct without the two bytes the generator
// places itself, "/" as the segment boundary and "%" as the start of
// an escape.
var safePunct = strings.NewReplacer("/", "", "%", "").Replace(legalPathPunct)

const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// TestTokenizePropertyAcceptsWireForm asserts that every path the wire
// generator produces is accepted. It pins over-rejection, which the
// reject-only properties below cannot see.
func TestTokenizePropertyAcceptsWireForm(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := wirePathGen().Draw(t, "path")
		_, err := Tokenize(path)
		require.NoErrorf(t, err, "generated wire path rejected: %q", path)
	})
}

// TestTokenizePropertyAcceptsOnlyEscapedForm asserts that an accepted
// path is what net/url emits for it. The reverse proxy forwards
// EscapedPath, so a path Go would re-encode reaches the app as bytes
// the matcher never saw. net/url decides here, not the generator, so
// widening a byte rule without widening what net/url preserves fails
// this property.
func TestTokenizePropertyAcceptsOnlyEscapedForm(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := wirePathGen().Draw(t, "path")
		_, err := Tokenize(path)
		require.NoError(t, err)
		u, err := url.ParseRequestURI(path)
		require.NoErrorf(t, err, "accepted path does not parse: %q", path)
		require.Equal(t, path, u.EscapedPath(), "accepted path is not its own escaped form")
	})
}

// TestTokenizePropertyPreservesStructure asserts that tokens are the
// input split on raw "/" and nothing else. Rejoining them reproduces
// the input byte for byte, so no token was decoded or rewritten.
func TestTokenizePropertyPreservesStructure(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := wirePathGen().Draw(t, "path")
		tokens, err := Tokenize(path)
		require.NoError(t, err)
		require.Equal(t, path, "/"+strings.Join(tokens, "/"))
		require.Len(t, tokens, strings.Count(path, "/"))
		for _, tok := range tokens {
			require.NotContains(t, tok, "/")
		}
	})
}

// TestTokenizePropertyIgnoresHexCase asserts that the hex case of an
// escape never changes the verdict, and that the tokens keep the case
// they arrived in.
func TestTokenizePropertyIgnoresHexCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := wirePathGen().Draw(t, "path")
		flipped := flipHexCase(path)
		tokens, err := Tokenize(path)
		require.NoError(t, err)
		flippedTokens, err := Tokenize(flipped)
		require.NoErrorf(t, err, "hex case flip changed the verdict: %q", flipped)
		require.Equal(t, flipped, "/"+strings.Join(flippedTokens, "/"))
		require.Len(t, flippedTokens, len(tokens))
	})
}

// TestTokenizePropertyRejectsRawNonASCII asserts that a raw byte at or
// above 0x80 anywhere in an accepted path is rejected. Unicode must
// arrive percent-encoded.
func TestTokenizePropertyRejectsRawNonASCII(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := wirePathGen().Draw(t, "path")
		b := byte(rapid.IntRange(0x80, 0xff).Draw(t, "byte"))
		mutated := insertAt(t, path, string(b))
		_, err := Tokenize(mutated)
		require.Errorf(t, err, "raw byte %#x not rejected: %q", b, mutated)
	})
}

// TestTokenizePropertyRejectsFoldForms asserts that a known fold or
// canonicalization form inserted anywhere in an accepted path is
// rejected.
func TestTokenizePropertyRejectsFoldForms(t *testing.T) {
	forms := map[string]string{
		"%2E (.)":                          "%2E",
		"%00 (NUL)":                        "%00",
		"%3F (?)":                          "%3F",
		"%5C (backslash)":                  "%5C",
		"%C0%AF (overlong UTF-8 for /)":    "%C0%AF",
		"%EF%BC%8F (fullwidth solidus)":    "%EF%BC%8F",
		"%E2%80%8B (zero-width space)":     "%E2%80%8B",
		"%C2%A0 (no-break space)":          "%C2%A0",
		"%E1%9A%80 (ogham space mark)":     "%E1%9A%80",
		"%CC%81 (combining acute on café)": "cafe%CC%81",
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				path := wirePathGen().Draw(t, "path")
				mutated := insertAt(t, path, form)
				_, err := Tokenize(mutated)
				require.Errorf(t, err, "%s not rejected: %q", name, mutated)
			})
		})
	}
}

// TestTokenizePropertyRejectsBadSegments asserts that a bad segment
// spliced in after any "/" of an accepted path is rejected.
func TestTokenizePropertyRejectsBadSegments(t *testing.T) {
	segments := map[string]string{
		"dot":                           ".",
		"dot-dot":                       "..",
		"empty":                         "",
		"leading mark %CC%87":           "%CC%87x",
		"leading mark after %2F":        "x%2F%CC%87y",
		"encoded dot-dot between":       "x%2F..%2Fy",
		"only a space %20":              "%20",
		"leading space %20":             "%20x",
		"trailing space %20":            "x%20",
		"leading space %20 after %2F":   "x%2F%20y",
		"trailing space %20 before %2F": "x%20%2Fy",
		"dot space dot":                 ".%20.",
		"dot-dot space dot":             "..%20.",
		"only dots":                     "...",
		"dots and spaces after %2F":     "x%2F.%20.",
		"trailing dot":                  "x.",
		"trailing dots":                 "x..",
		"trailing space dot":            "x%20.",
		"trailing dot before %2F":       "x.%2Fy",
	}
	for name, segment := range segments {
		t.Run(name, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				path := wirePathGen().Draw(t, "path")
				var slashes []int
				for i := range len(path) {
					if path[i] == '/' {
						slashes = append(slashes, i)
					}
				}
				n := rapid.IntRange(0, len(slashes)-1).Draw(t, "slash")
				i := slashes[n]
				mutated := path[:i+1] + segment + "/" + path[i+1:]
				_, err := Tokenize(mutated)
				require.Errorf(t, err, "segment %q not rejected: %q", segment, mutated)
			})
		})
	}
}

// TestTokenizePropertyRejectsEdgeSpaces asserts that a %20 spliced at
// the start or end of any part of an accepted path is rejected. An
// upstream that trims the part would see a different part.
func TestTokenizePropertyRejectsEdgeSpaces(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := wirePathGen().Draw(t, "path")
		starts, ends := partBounds(path)
		bounds := starts
		if rapid.Bool().Draw(t, "trailing") {
			bounds = ends
		}
		i := rapid.SampledFrom(bounds).Draw(t, "bound")
		mutated := path[:i] + "%20" + path[i:]
		_, err := Tokenize(mutated)
		require.Errorf(t, err, "edge space not rejected: %q", mutated)
	})
}

// wirePathGen produces a valid wire path, one or more segments with an
// optional trailing slash.
func wirePathGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(1, 4).Draw(t, "segment-count")
		segments := make([]string, n)
		for i := range segments {
			segments[i] = segmentGen().Draw(t, "segment")
		}
		path := "/" + strings.Join(segments, "/")
		if rapid.Bool().Draw(t, "trailing-slash") {
			path += "/"
		}
		return path
	})
}

// segmentGen joins one or more parts with the encoded separator, %2F
// or %2f.
func segmentGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(1, 3).Draw(t, "part-count")
		parts := make([]string, n)
		for i := range parts {
			parts[i] = partGen().Draw(t, "part")
		}
		sep := rapid.SampledFrom([]string{"%2F", "%2f"}).Draw(t, "encoded-separator")
		return strings.Join(parts, sep)
	})
}

// partGen produces the percent-encoded content between two separators.
// No part starts with a mark and no rune composes with its neighbor.
// A space, emitted as %20, composes with no neighbor under NFKC, so
// it needs no neighbor rule. The filter drops parts made of only
// dots and spaces, parts that start or end with a space, and parts
// whose trailing run of dots and spaces contains a dot.
func partGen() *rapid.Generator[string] {
	content := rapid.Custom(func(t *rapid.T) string {
		var b strings.Builder
		n := rapid.IntRange(1, 5).Draw(t, "rune-count")
		for range n {
			switch rapid.IntRange(0, 4).Draw(t, "rune-kind") {
			case 0:
				b.WriteByte(alnum[rapid.IntRange(0, len(alnum)-1).Draw(t, "alnum")])
			case 1:
				b.WriteByte(safePunct[rapid.IntRange(0, len(safePunct)-1).Draw(t, "punct")])
			case 2:
				b.WriteRune(safeRuneGen().Draw(t, "non-ascii"))
			case 3:
				b.WriteString(rapid.SampledFrom(stableMarkPairs).Draw(t, "mark-pair"))
			case 4:
				b.WriteByte(' ')
			}
			if rapid.Bool().Draw(t, "trailing-mark") {
				b.WriteRune(freeMarkGen().Draw(t, "mark"))
			}
		}
		return b.String()
	}).Filter(func(s string) bool {
		return strings.Trim(s, ". ") != "" &&
			!strings.HasPrefix(s, " ") && !strings.HasSuffix(s, " ") &&
			!strings.Contains(s[len(strings.TrimRight(s, ". ")):], ".")
	})
	return rapid.Custom(func(t *rapid.T) string {
		return encodeNonASCII(t, content.Draw(t, "content"))
	})
}

// safeRuneGen produces a non-ASCII rune that is legal anywhere in a
// part, including first, so it excludes the marks.
func safeRuneGen() *rapid.Generator[rune] {
	return freeRuneGen().Filter(func(r rune) bool { return !unicode.IsMark(r) })
}

// freeMarkGen produces a mark that stands on its own, so it is legal
// anywhere except as the first rune of a part.
func freeMarkGen() *rapid.Generator[rune] {
	return freeRuneGen().Filter(unicode.IsMark)
}

// freeRuneGen produces a non-ASCII rune that NFKC puts a boundary
// before, so it never combines with whatever the generator wrote
// previously. That covers the combining marks with a non-zero class
// and the trailing Hangul jamo without naming either. The filter reads
// the stdlib category tables and the normalization properties rather
// than isGraphicRune, so the generator does not inherit the
// validator's own view of what is allowed.
func freeRuneGen() *rapid.Generator[rune] {
	return rapid.Rune().Filter(func(r rune) bool {
		if r < 0x80 || !unicode.IsGraphic(r) || unicode.IsSpace(r) {
			return false
		}
		return norm.NFKC.PropertiesString(string(r)).BoundaryBefore() &&
			norm.NFKC.IsNormalString(string(r))
	})
}

// encodeNonASCII percent-encodes the bytes of s at or above 0x80 and
// the space, which has no raw form in a path, and leaves the other
// ASCII bytes raw, in the hex case the generator picks.
func encodeNonASCII(t *rapid.T, s string) string {
	format := "%%%02x"
	if rapid.Bool().Draw(t, "upper-hex") {
		format = "%%%02X"
	}
	var b strings.Builder
	for i := range len(s) {
		if s[i] < 0x80 && s[i] != ' ' {
			b.WriteByte(s[i])
			continue
		}
		fmt.Fprintf(&b, format, s[i])
	}
	return b.String()
}

// partBounds returns the offsets where a part starts and ends. A part
// is bounded by a raw slash, an encoded separator, or the end of the
// path.
func partBounds(path string) (starts, ends []int) {
	for i := range len(path) {
		switch {
		case path[i] == '/':
			starts = append(starts, i+1)
			if i > 0 {
				ends = append(ends, i)
			}
		case path[i] == '%' && i+2 < len(path) && path[i+1] == '2' && (path[i+2] == 'F' || path[i+2] == 'f'):
			starts = append(starts, i+3)
			ends = append(ends, i)
		}
	}
	ends = append(ends, len(path))
	return starts, ends
}

// insertAt splices s into path at a generated offset after the leading
// slash.
func insertAt(t *rapid.T, path, s string) string {
	i := rapid.IntRange(1, len(path)).Draw(t, "offset")
	return path[:i] + s + path[i:]
}

// flipHexCase swaps the case of the hex digits of every escape in
// path.
func flipHexCase(path string) string {
	out := []byte(path)
	for i := 0; i+2 < len(out); i++ {
		if out[i] != '%' {
			continue
		}
		out[i+1] = flipByteCase(out[i+1])
		out[i+2] = flipByteCase(out[i+2])
	}
	return string(out)
}

// flipByteCase swaps the case of an ASCII letter.
func flipByteCase(b byte) byte {
	switch {
	case b >= 'a' && b <= 'z':
		return b - 'a' + 'A'
	case b >= 'A' && b <= 'Z':
		return b - 'A' + 'a'
	}
	return b
}
