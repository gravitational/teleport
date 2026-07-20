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
	"unicode"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestProperty_Tokenize_RejectsRawNonASCII asserts that any raw byte
// at or above 0x80 inserted into an accepted path causes rejection.
// Raw unicode must arrive percent-encoded.
func TestProperty_Tokenize_RejectsRawNonASCII(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := acceptedPathGen().Draw(t, "path")
		pos := rapid.IntRange(1, len(path)).Draw(t, "pos")
		b := byte(rapid.IntRange(0x80, 0xff).Draw(t, "b"))
		mutated := path[:pos] + string(b) + path[pos:]
		_, err := Tokenize(mutated)
		require.Errorf(t, err,
			"raw byte %#x at pos %d not rejected: %q",
			b, pos, mutated)
	})
}

// TestProperty_Tokenize_LengthCap asserts that any path over the 8 KiB
// cap is rejected regardless of content. The length check runs before
// every content check, so the property holds for any byte pattern.
func TestProperty_Tokenize_LengthCap(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		size := rapid.IntRange(maxPathBytes, maxPathBytes*2).Draw(t, "size")
		path := "/" + strings.Repeat("a", size)
		_, err := Tokenize(path)
		require.Errorf(t, err, "path of length %d should be rejected", len(path))
	})
}

// TestProperty_Tokenize_AcceptedContentIsGraphic pins the graphic-only
// content invariant. For every accepted path, every rune of every
// decoded segment is a letter, mark, number, punctuation, or symbol.
// A control, format, separator, surrogate, private-use, or unassigned
// rune slipping through would fail this property.
func TestProperty_Tokenize_AcceptedContentIsGraphic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := rapid.String().Draw(t, "path")
		tokens, err := Tokenize(path)
		if err != nil {
			return
		}
		for _, tok := range tokens {
			content := decodeNonASCII(tok)
			for _, r := range content {
				graphic := unicode.IsLetter(r) || unicode.IsMark(r) ||
					unicode.IsNumber(r) || unicode.IsPunct(r) ||
					unicode.IsSymbol(r)
				require.Truef(t, graphic,
					"accepted token %q has non-graphic rune %U", tok, r)
			}
		}
	})
}

// TestProperty_Tokenize_FoldSafety_Substring asserts that any known
// fold or bypass substring, inserted anywhere in an accepted path, is
// rejected. The set covers banned ASCII escapes (%2E fold-to-dot, %00
// null), overlong UTF-8 for "/" (%C0%AF), an NFKC-unstable fullwidth
// solidus (%EF%BC%8F), and a non-graphic zero-width space
// (%E2%80%8B). If any of these ever slipped past the validator, the
// matcher would see a segment the caller never wrote.
func TestProperty_Tokenize_FoldSafety_Substring(t *testing.T) {
	attacks := []string{"%2E", "%00", "%C0%AF", "%EF%BC%8F", "%E2%80%8B"}
	for _, attack := range attacks {
		t.Run(attack, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				path := acceptedPathGen().Draw(t, "path")
				pos := rapid.IntRange(1, len(path)).Draw(t, "pos")
				mutated := path[:pos] + attack + path[pos:]
				_, err := Tokenize(mutated)
				require.Errorf(t, err,
					"attack %q inserted at %d not rejected: %q",
					attack, pos, mutated)
			})
		})
	}
}

// TestProperty_Tokenize_FoldSafety_Segment asserts that inserting a
// bad segment between two "/" of an accepted path is rejected. The
// set covers the "." and ".." segments and the empty segment that
// creates consecutive slashes.
func TestProperty_Tokenize_FoldSafety_Segment(t *testing.T) {
	cases := []struct {
		name    string
		segment string
	}{
		{"dot", "."},
		{"dotdot", ".."},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				path := acceptedPathGen().Draw(t, "path")
				var slashIdxs []int
				for i := 0; i < len(path); i++ {
					if path[i] == '/' {
						slashIdxs = append(slashIdxs, i)
					}
				}
				n := rapid.IntRange(0, len(slashIdxs)-1).Draw(t, "slashIdx")
				i := slashIdxs[n]
				mutated := path[:i+1] + tc.segment + "/" + path[i+1:]
				_, err := Tokenize(mutated)
				require.Errorf(t, err,
					"segment %q at slash %d not rejected: %q",
					tc.segment, n, mutated)
			})
		})
	}
}

// acceptedPathGen produces paths the validator accepts. Each path is
// "/" followed by one to five segments of lowercase ASCII letters and
// digits. Attack tests inject known-bad content into these paths and
// check that Tokenize rejects the result.
func acceptedPathGen() *rapid.Generator[string] {
	alnum := rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz0123456789"))
	seg := rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(1, 8).Draw(t, "seg-len")
		var b strings.Builder
		for range n {
			b.WriteRune(alnum.Draw(t, "ch"))
		}
		return b.String()
	})
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(1, 5).Draw(t, "seg-count")
		segs := make([]string, n)
		for i := range segs {
			segs[i] = seg.Draw(t, "seg")
		}
		return "/" + strings.Join(segs, "/")
	})
}
