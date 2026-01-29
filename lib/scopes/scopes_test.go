/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package scopes

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateSegment tests the ValidateSegment function for a few specific special cases. Most coverage is actually
// done by the StrongValidate test, which relies on ValidateSegment internally for most of its functionality.
func TestValidateSegment(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		segment  string
		strongOk bool
		weakOk   bool
	}{
		{
			name:     "valid segment",
			segment:  "aa",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "empty segment",
			segment:  "",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "segment with leading symbol",
			segment:  ".aa",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "segment with trailing symbol",
			segment:  "aa_",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "segment with reserved symbol",
			segment:  "aa@bb",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "segment with whitespace",
			segment:  "aa bb",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "segment with middle separator",
			segment:  "aa/bb",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "segment with leading separator",
			segment:  "/aa",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "segment with trailing separator",
			segment:  "aa/",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "root-like segment",
			segment:  "/",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "leading uppercase segment",
			segment:  "Aa",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "middle uppsercase segment",
			segment:  "aAa",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "trailing uppercase segment",
			segment:  "aA",
			strongOk: false,
			weakOk:   true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidateSegment(tt.segment)
			if tt.strongOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			err = WeakValidateSegment(tt.segment)
			if tt.weakOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestStrongValidate tests the StrongValidate function for various combinations of scopes.
func TestStrongValidate(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name  string
		scope string
		ok    bool
	}{
		{
			name:  "valid root",
			scope: "/",
			ok:    true,
		},
		{
			name:  "valid single-segment",
			scope: "/aa",
			ok:    true,
		},
		{
			name:  "valid multi-segment",
			scope: "/aa/bb/cc",
			ok:    true,
		},
		{
			name:  "empty rejected",
			scope: "",
			ok:    false,
		},
		{
			name:  "missing prefix rejected",
			scope: "aa/bb/cc",
			ok:    false,
		},
		{
			name:  "dangling separator rejected",
			scope: "/aa/bb/cc/",
			ok:    false,
		},
		{
			name:  "single-segment invalid chars",
			scope: "/a ",
			ok:    false,
		},
		{
			name:  "multi-segment invalid chars fist",
			scope: "/a(a/bb",
			ok:    false,
		},
		{
			name:  "multi-segment invalid chars last",
			scope: "/aa/b)b",
			ok:    false,
		},
		{
			name:  "multi-segment invalid chars middle",
			scope: "/aa/b!b/cc",
			ok:    false,
		},
		{
			name:  "single-segment too short",
			scope: "/a",
			ok:    false,
		},
		{
			name:  "multi-segment too short",
			scope: "/aa/b",
			ok:    false,
		},
		{
			name:  "long but ok scope",
			scope: "/aaaaaaaaaaaaaaa/bbbbbbbbbbbbbbb/ccccccccccccccc/ddddddddddddddd",
			ok:    true,
		},
		{
			name:  "scope too long",
			scope: "/aaaaaaaaaaaaaaa/bbbbbbbbbbbbbbb/ccccccccccccccc/dddddddddddddddd",
			ok:    false,
		},
		{
			name:  "long but ok segment",
			scope: "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ok:    true,
		},
		{
			name:  "segment too long",
			scope: "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ok:    false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidate(tt.scope)
			if tt.ok {
				require.NoError(t, err)
				require.NoError(t, WeakValidate(tt.scope)) // weak validate should accept all strongly valid scopes too
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestWeakValidate tests the WeakValidate function for various combinations of scopes. The WeakValidate function
// is already partially tested in the StrongValidate tests, so this test focuses on the specific cases that are
// not covered there.
func TestWeakValidate(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name  string
		scope string
		ok    bool
	}{
		{
			name:  "explicitly disallowed character is disallowed",
			scope: "/a@a/bb/cc",
			ok:    false,
		},
		{
			name:  "control character disallowed",
			scope: "/a\na/bb/cc",
			ok:    false,
		},
		{
			name:  "unsupported character passes",
			scope: "/aaa/b:b/cc",
			ok:    true,
		},
		{
			name:  "short segment passes",
			scope: "/a/bb/cc",
			ok:    true,
		},
		{
			name:  "long segment passes",
			scope: "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/bbbbbbbbbbbbbbb/ccccccccccccccc/ddddddddddddddd",
			ok:    true,
		},
		{
			name:  "empty segment passes",
			scope: "/aa//bb/cc",
			ok:    true,
		},
		{
			name:  "dangling separator passes",
			scope: "/aa/bb/cc/",
			ok:    true,
		},
		{
			name:  "empty rejected",
			scope: "",
			ok:    false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := WeakValidate(tt.scope)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestDescendingSegments tests the DescendingSegments iterator for various combinations of scopes, and verifies that
// re-joining and re-segmenting produces the same segments.
func TestDescendingSegments(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		scope    string
		segments []string
	}{
		{
			name:     "root",
			scope:    "/",
			segments: nil,
		},
		{
			name:     "empty",
			scope:    "",
			segments: nil,
		},
		{
			name:     "single-segment",
			scope:    "/aa",
			segments: []string{"aa"},
		},
		{
			name:     "multi-segment",
			scope:    "/aa/bb/cc",
			segments: []string{"aa", "bb", "cc"},
		},
		{
			name:     "dangling separator single",
			scope:    "/aa/",
			segments: []string{"aa"},
		},
		{
			name:     "dangling separator multi",
			scope:    "/aa/bb/cc/",
			segments: []string{"aa", "bb", "cc"},
		},
		{
			name:     "leading empty segment",
			scope:    "//aa/bb",
			segments: []string{"", "aa", "bb"},
		},
		{
			name:     "middle empty segment",
			scope:    "/aa//bb",
			segments: []string{"aa", "", "bb"},
		},
		{
			name:     "trailing empty segment",
			scope:    "/aa/bb//",
			segments: []string{"aa", "bb", ""},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// verify that the iterator produces the expected segments
			segs := slices.Collect(DescendingSegments(tt.scope))
			require.Equal(t, tt.segments, segs)

			// verfiy that joining and re-segmenting produces the same segments
			segs2 := slices.Collect(DescendingSegments(Join(segs...)))
			require.Equal(t, tt.segments, segs2)
		})
	}
}

// TestJoin tests the Join function for various combinations of segments, and verifies that re-segmenting
// the joined value produces the original segments.
func TestJoin(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		segments []string
		expect   string
	}{
		{
			name:     "root",
			segments: nil,
			expect:   "/",
		},
		{
			name:     "single segment",
			segments: []string{"aa"},
			expect:   "/aa",
		},
		{
			name:     "multi segment",
			segments: []string{"aa", "bb", "cc"},
			expect:   "/aa/bb/cc",
		},
		{
			name:     "leading empty segment preserved",
			segments: []string{"", "aa", "bb"},
			expect:   "//aa/bb",
		},
		{
			name:     "middle empty segment preserved",
			segments: []string{"aa", "", "bb"},
			expect:   "/aa//bb",
		},
		{
			name:     "trailing empty segment preserved",
			segments: []string{"aa", "bb", ""},
			expect:   "/aa/bb//",
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// verify that Join produces the expected scope value
			joined := Join(tt.segments...)
			require.Equal(t, tt.expect, joined)

			// verify that re-segmentation of the joined value produces the original segments
			reSegmented := slices.Collect(DescendingSegments(joined))
			require.Equal(t, tt.segments, reSegmented)
		})
	}
}

// TestDescendingScopes tests the DescendingScopes function for various combinations of scopes, and verifies that
func TestDescendingScopes(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		scope  string
		expect []string
	}{
		{
			name:   "root",
			scope:  "/",
			expect: []string{"/"},
		},
		{
			name:   "empty",
			scope:  "",
			expect: nil,
		},
		{
			name:   "single-segment",
			scope:  "/aa",
			expect: []string{"/", "/aa"},
		},
		{
			name:   "multi-segment",
			scope:  "/aa/bb/cc",
			expect: []string{"/", "/aa", "/aa/bb", "/aa/bb/cc"},
		},
		{
			name:   "dangling separator single",
			scope:  "/aa/",
			expect: []string{"/", "/aa"},
		},
		{
			name:   "dangling separator multi",
			scope:  "/aa/bb/cc/",
			expect: []string{"/", "/aa", "/aa/bb", "/aa/bb/cc"},
		},
		{
			name:   "missing prefix single",
			scope:  "aa",
			expect: []string{"/", "/aa"},
		},
		{
			name:   "missing prefix multi",
			scope:  "aa/bb/cc",
			expect: []string{"/", "/aa", "/aa/bb", "/aa/bb/cc"},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// verify that the iterator produces the expected scopes
			scopes := slices.Collect(DescendingScopes(tt.scope))
			require.Equal(t, tt.expect, scopes)
		})
	}
}

// TestAscendingScopes tests the AscendingScopes function for various combinations of scopes.
func TestAscendingScopes(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		scope  string
		expect []string
	}{
		{
			name:   "root",
			scope:  "/",
			expect: []string{"/"},
		},
		{
			name:   "empty",
			scope:  "",
			expect: nil,
		},
		{
			name:   "single-segment",
			scope:  "/aa",
			expect: []string{"/aa", "/"},
		},
		{
			name:   "multi-segment",
			scope:  "/aa/bb/cc",
			expect: []string{"/aa/bb/cc", "/aa/bb", "/aa", "/"},
		},
		{
			name:   "dangling separator single",
			scope:  "/aa/",
			expect: []string{"/aa", "/"},
		},
		{
			name:   "dangling separator multi",
			scope:  "/aa/bb/cc/",
			expect: []string{"/aa/bb/cc", "/aa/bb", "/aa", "/"},
		},
		{
			name:   "missing prefix single",
			scope:  "aa",
			expect: []string{"/aa", "/"},
		},
		{
			name:   "missing prefix multi",
			scope:  "aa/bb/cc",
			expect: []string{"/aa/bb/cc", "/aa/bb", "/aa", "/"},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// verify that the iterator produces the expected scopes
			scopes := slices.Collect(AscendingScopes(tt.scope))
			require.Equal(t, tt.expect, scopes)
		})
	}
}

// TestCompare tests the Compare function for various combinations of scopes.
func TestCompare(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		lhs, rhs string
		rel      Relationship
	}{
		{
			name: "simple equivalence",
			lhs:  "/aa/bb/cc",
			rhs:  "/aa/bb/cc",
			rel:  Equivalent,
		},
		{
			name: "simple ancestor",
			lhs:  "/aa/bb/cc",
			rhs:  "/aa/bb",
			rel:  Ancestor,
		},
		{
			name: "simple descendant",
			lhs:  "/aa/bb",
			rhs:  "/aa/bb/cc",
			rel:  Descendant,
		},
		{
			name: "simple orthogonal",
			lhs:  "/aa/bb/cc",
			rhs:  "/dd/ee/ff",
			rel:  Orthogonal,
		},
		{
			name: "multi-level ancestor",
			lhs:  "/aa/bb/cc",
			rhs:  "/aa",
			rel:  Ancestor,
		},
		{
			name: "multi-level descendant",
			lhs:  "/aa",
			rhs:  "/aa/bb/cc",
			rel:  Descendant,
		},
		{
			name: "root equivalence",
			lhs:  "/",
			rhs:  "/",
			rel:  Equivalent,
		},
		{
			name: "root in descendant case",
			lhs:  "/",
			rhs:  "/aa",
			rel:  Descendant,
		},
		{
			name: "root in ancestor case",
			lhs:  "/aa",
			rhs:  "/",
			rel:  Ancestor,
		},
		{
			name: "empty root equivalence",
			lhs:  "",
			rhs:  "",
			rel:  Orthogonal,
		},
		{
			name: "empty lhs root in equivalence case",
			lhs:  "",
			rhs:  "/",
			rel:  Orthogonal,
		},
		{
			name: "empty rhs root in equivalence case",
			lhs:  "/",
			rhs:  "",
			rel:  Orthogonal,
		},
		{
			name: "empty root in descendant case",
			lhs:  "",
			rhs:  "/aa",
			rel:  Orthogonal,
		},
		{
			name: "empty root in ancestor case",
			lhs:  "/aa",
			rhs:  "",
			rel:  Orthogonal,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.rel, Compare(tt.lhs, tt.rhs), "Compare(%q, %q)", tt.lhs, tt.rhs)

			// verify that NormalizeForEquality produces equivalent equality results *except* in the case of two empty scopes,
			// which are considered orthogonal by Compare but will end up being equal when normalized.
			if tt.rel == Equivalent || (tt.lhs == "" && tt.rhs == "") {
				require.Equal(t, NormalizeForEquality(tt.lhs), NormalizeForEquality(tt.rhs), "expected NormalizeForEquality(%q) == NormalizeForEquality(%q)", tt.lhs, tt.rhs)
			} else {
				require.NotEqual(t, NormalizeForEquality(tt.lhs), NormalizeForEquality(tt.rhs), "expected NormalizeForEquality(%q) != NormalizeForEquality(%q)", tt.lhs, tt.rhs)
			}
		})
	}
}

func TestNormalizeForEquality(t *testing.T) {
	tts := []struct {
		name   string
		scope  string
		expect string
	}{
		{
			name:   "root",
			scope:  "/",
			expect: "/",
		},
		{
			name:   "dangling separator",
			scope:  "/aa/bb/cc/",
			expect: "/aa/bb/cc",
		},
		{
			name:   "missing prefix",
			scope:  "aa/bb/cc",
			expect: "/aa/bb/cc",
		},
		{
			name:   "both missing prefix and dangling separator",
			scope:  "aa/bb/cc/",
			expect: "/aa/bb/cc",
		},
		{
			name:   "normal scope",
			scope:  "/aa/bb/cc",
			expect: "/aa/bb/cc",
		},
		{
			name:   "empty scope",
			scope:  "",
			expect: "",
		},
		{
			name:   "empty segment",
			scope:  "/aa//bb/cc/",
			expect: "/aa//bb/cc",
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, NormalizeForEquality(tt.scope), "NormalizeForEquality(%q)", tt.scope)
		})
	}
}

// Test Sort verifies that the Sort function produces the expected ordering for various scopes.
func TestSort(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name        string
		scopes      []string
		expected    []string
		lexographic bool
	}{
		{
			name:        "basic hierarchy",
			scopes:      []string{"/aa/bb", "/aa", "/", "/aa/bb/cc"},
			expected:    []string{"/", "/aa", "/aa/bb", "/aa/bb/cc"},
			lexographic: true,
		},
		{
			name:        "basic non-lexographic",
			scopes:      []string{"/aa-bb", "/aa", "/", "/aa/bb", "/aa/bb-cc"},
			expected:    []string{"/", "/aa", "/aa/bb", "/aa/bb-cc", "/aa-bb"},
			lexographic: false,
		},
		{
			name:        "empty",
			scopes:      []string{},
			expected:    []string{},
			lexographic: true,
		},
		{
			name:        "single element",
			scopes:      []string{"/aa/bb/cc"},
			expected:    []string{"/aa/bb/cc"},
			lexographic: true,
		},
		{
			name:        "missing prefixes",
			scopes:      []string{"/aa/bb/cc", "aa/bb", "/aa", "/", "xx/yy", "/xx/yy"},
			expected:    []string{"/", "/aa", "aa/bb", "/aa/bb/cc", "xx/yy", "/xx/yy"},
			lexographic: false,
		},
		{
			name:        "dangling suffixes",
			scopes:      []string{"/aa/bb/cc/", "/aa/", "/"},
			expected:    []string{"/", "/aa/", "/aa/bb/cc/"},
			lexographic: true,
		},
		{
			name:        "prefix and suffix do not affect ordering of equivalents",
			scopes:      []string{"xx/yy", "/aa/bb/", "/xx/yy", "/aa/bb", "/aa/bb/", "xx/yy"},
			expected:    []string{"/aa/bb/", "/aa/bb", "/aa/bb/", "xx/yy", "/xx/yy", "xx/yy"},
			lexographic: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			sorted := make([]string, len(tt.scopes))
			copy(sorted, tt.scopes)

			slices.SortFunc(sorted, Sort)

			require.Equal(t, tt.expected, sorted, "expected scope sort")

			lex := make([]string, len(tt.scopes))
			copy(lex, tt.scopes)
			slices.Sort(lex)

			if tt.lexographic {
				require.Equal(t, lex, sorted, "scope sort is expected to match lexographic sort")
			} else {
				require.NotEqual(t, lex, sorted, "scope sort is expected to differ from lexographic sort")
			}
		})
	}
}

// TestScopeOfEffectAndResourceScope tests the relationship between scope of effect and resource scopes helpers
// given various combinations of effect and resource scopes.
func TestScopeOfEffectAndResourceScope(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		effect   string
		resource string
		applies  bool
	}{
		{
			name:     "simple root",
			effect:   "/",
			resource: "/",
			applies:  true,
		},
		{
			name:     "simple root effect",
			effect:   "/",
			resource: "/aa",
			applies:  true,
		},
		{
			name:     "simple root resource",
			effect:   "/aa",
			resource: "/",
			applies:  false,
		},
		{
			name:     "equivalent scopes",
			effect:   "/staging",
			resource: "/staging",
			applies:  true,
		},
		{
			name:     "effect at ancestor of resource",
			effect:   "/staging",
			resource: "/staging/west",
			applies:  true,
		},
		{
			name:     "effect at descendant of resource",
			effect:   "/staging/west",
			resource: "/staging",
			applies:  false,
		},
		{
			name:     "orthogonal scopes",
			effect:   "/staging",
			resource: "/prod",
			applies:  false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.applies, ScopeOfEffect(tt.effect).AppliesToResourceScope(tt.resource),
				"ScopeOfEffect(%q).AppliesToResourceScope(%q)", tt.effect, tt.resource)

			require.Equal(t, tt.applies, ResourceScope(tt.resource).IsSubjectToScopeOfEffect(tt.effect),
				"ResourceScope(%q).IsSubjectToScopeOfEffect(%q)", tt.resource, tt.effect)
		})
	}
}

// TestValidateGlob tests the ValidateGlob function for various combinations of globs.
func TestValidateGlob(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		glob     string
		strongOk bool
		weakOk   bool
	}{
		{
			name:     "standard literal",
			glob:     "/aa/bb/cc",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "root literal",
			glob:     "/",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "valid exclusive child glob",
			glob:     "/aa/bb/**",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "inclusive glob rejected",
			glob:     "/aa/bb/*",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "inline exclusive glob rejected",
			glob:     "/aa/**/cc",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "root exclusive child glob",
			glob:     "/**",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "root exclusive child glob with trailing slash",
			glob:     "/**/",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "root glob without leading slash",
			glob:     "**",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "glob with mildly invalid segment",
			glob:     "/a/**",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "glob with very invalid segment",
			glob:     "/a@/**",
			strongOk: false,
			weakOk:   false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidateGlob(tt.glob)
			if tt.strongOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			err = WeakValidateGlob(tt.glob)
			if tt.weakOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestGlobMatch tests Glob.Matches for various combinations of globs and scopes.
func TestGlobMatch(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name  string
		glob  string
		scope string
		match bool
	}{
		{
			name:  "simple literal exact match",
			glob:  "/aa/bb/cc",
			scope: "/aa/bb/cc",
			match: true,
		},
		{
			name:  "simple literal mismatch",
			glob:  "/aa/bb/cc",
			scope: "/aa/bb/dd",
			match: false,
		},
		{
			name:  "simple literal mismatch root",
			glob:  "/aa/bb",
			scope: "/",
		},
		{
			name:  "root literal exact match",
			glob:  "/",
			scope: "/",
			match: true,
		},
		{
			name:  "simple literal match child",
			glob:  "/aa/bb",
			scope: "/aa/bb/cc",
			match: true,
		},
		{
			name:  "simple literal mismatch child",
			glob:  "/aa/bb",
			scope: "/aa/cc/bb",
			match: false,
		},
		{
			name:  "root literal match child",
			glob:  "/",
			scope: "/aa",
			match: true,
		},
		{
			name:  "exclusive child glob match",
			glob:  "/aa/bb/**",
			scope: "/aa/bb/cc",
			match: true,
		},
		{
			name:  "exclusive child glob match multipart",
			glob:  "/aa/bb/**",
			scope: "/aa/bb/cc/dd",
			match: true,
		},
		{
			name:  "exclusive child glob mismatch equivalent",
			glob:  "/aa/bb/**",
			scope: "/aa/bb",
			match: false,
		},
		{
			name:  "exclusive child glob mismatch ancestor",
			glob:  "/aa/bb/**",
			scope: "/aa",
			match: false,
		},
		{
			name:  "exclusive child glob mismatch orthogonal",
			glob:  "/aa/bb/**",
			scope: "/aa/cc/dd",
			match: false,
		},
		{
			name:  "exclusive child glob match root",
			glob:  "/**",
			scope: "/aa",
			match: true,
		},
		{
			name:  "exclusive child glob match root multipart",
			glob:  "/**",
			scope: "/aa/bb/cc",
			match: true,
		},
		{
			name:  "exclusive child glob mismatch root",
			glob:  "/**",
			scope: "/",
			match: false,
		},
		{
			name:  "exclusive child glob mismatch empty root",
			glob:  "/**",
			scope: "",
			match: false,
		},
		{
			name:  "inline glob match",
			glob:  "/aa/**/cc",
			scope: "/aa/bb/cc",
			match: true,
		},
		{
			name:  "inline glob match multipart",
			glob:  "/aa/**/cc",
			scope: "/aa/bb/cc/dd",
			match: true,
		},
		{
			name:  "inline glob simple mismatch",
			glob:  "/aa/**/cc",
			scope: "/aa/bb/dd",
			match: false,
		},
		{
			name:  "inline glob mismatch due to multiple segments",
			glob:  "/aa/**/cc",
			scope: "/aa/bb/dd/cc",
			match: false,
		},
		{
			name:  "inline glob mismatch due to leading segment",
			glob:  "/aa/**/cc",
			scope: "/bb/dd/cc",
			match: false,
		},
		{
			name:  "inline root glob match",
			glob:  "/**/bb",
			scope: "/aa/bb",
			match: true,
		},
		{
			name:  "inline root glob match multipart",
			glob:  "/**/bb",
			scope: "/aa/bb/cc/dd",
			match: true,
		},
		{
			name:  "inline root glob mismatch",
			glob:  "/**/bb",
			scope: "/aa/cc",
			match: false,
		},
		{
			name:  "inline root glob mismatch due to early segment",
			glob:  "/**/bb",
			scope: "/bb/cc",
			match: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.match, Glob(tt.glob).MatchesScopeLiteral(tt.scope),
				"Glob(%q).MatchesScopeLiteral(%q)", tt.glob, tt.scope)
		})
	}
}

func TestGlobOnlyMatchesSubjectsOf(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name    string
		glob    string
		scope   string
		subject bool
	}{
		{
			name:    "equivalent root",
			glob:    "/",
			scope:   "/",
			subject: true,
		},
		{
			name:    "non-subject root",
			glob:    "/",
			scope:   "/aa",
			subject: false,
		},
		{
			name:    "exclusive child glob root",
			glob:    "/**",
			scope:   "/",
			subject: true,
		},
		{
			name:    "non-subject exclusive child glob root",
			glob:    "/**",
			scope:   "/aa",
			subject: false,
		},
		{
			name:    "child of root",
			glob:    "/foo",
			scope:   "/",
			subject: true,
		},
		{
			name:    "exclusive child glob in child of root",
			glob:    "/foo/**",
			scope:   "/",
			subject: true,
		},
		{
			name:    "equivalent children",
			glob:    "/foo",
			scope:   "/foo",
			subject: true,
		},
		{
			name:    "orthogonal children",
			glob:    "/foo",
			scope:   "/bar",
			subject: false,
		},
		{
			name:    "orthogonal exclusive child glob",
			glob:    "/foo/**",
			scope:   "/bar",
			subject: false,
		},
		{
			name:    "exclusive child glob in child",
			glob:    "/foo/**",
			scope:   "/foo",
			subject: true,
		},
		{
			name:    "child of child",
			glob:    "/foo/bar",
			scope:   "/foo",
			subject: true,
		},
		{
			name:    "orthogonal child of child",
			glob:    "/foo/bar",
			scope:   "/foo/baz",
			subject: false,
		},
		{
			name:    "exclusive child glob in child of child",
			glob:    "/foo/bar/**",
			scope:   "/foo",
			subject: true,
		},
		{
			name:    "inline glob",
			glob:    "/foo/**/bar",
			scope:   "/foo",
			subject: true,
		},
		{
			name:    "inline glob potentially orthogonal",
			glob:    "/foo/**/bar",
			scope:   "/foo/bar",
			subject: false,
		},
		{
			name:    "inline glob in child",
			glob:    "/foo/**/bar",
			scope:   "/foo/baz/bar",
			subject: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.subject, Glob(tt.glob).OnlyMatchesSubjectsOf(tt.scope),
				"Glob(%q).OnlyMatchesSubjectsOf(%q)", tt.glob, tt.scope)
		})
	}
}

// TestEnforcementPointsForResourceScope verifies that EnforcementPointsForResourceScope yields all (ScopeOfOrigin, ScopeOfEffect)
// pairs in the correct order, independent of any particular assignment tree.
func TestEnforcementPointsForResourceScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		resourceScope string
		expect        []EnforcementPoint
	}{
		{
			name:          "root scope",
			resourceScope: "/",
			expect: []EnforcementPoint{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/"},
			},
		},
		{
			name:          "single segment scope",
			resourceScope: "/foo",
			expect: []EnforcementPoint{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/foo"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/"},
				{ScopeOfOrigin: "/foo", ScopeOfEffect: "/foo"},
			},
		},
		{
			name:          "two segment scope",
			resourceScope: "/staging/west",
			expect: []EnforcementPoint{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging/west"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/staging"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging/west"},
				{ScopeOfOrigin: "/staging", ScopeOfEffect: "/staging"},
				{ScopeOfOrigin: "/staging/west", ScopeOfEffect: "/staging/west"},
			},
		},
		{
			name:          "three segment scope",
			resourceScope: "/prod/us/east",
			expect: []EnforcementPoint{
				{ScopeOfOrigin: "/", ScopeOfEffect: "/prod/us/east"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/prod/us"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/prod"},
				{ScopeOfOrigin: "/", ScopeOfEffect: "/"},
				{ScopeOfOrigin: "/prod", ScopeOfEffect: "/prod/us/east"},
				{ScopeOfOrigin: "/prod", ScopeOfEffect: "/prod/us"},
				{ScopeOfOrigin: "/prod", ScopeOfEffect: "/prod"},
				{ScopeOfOrigin: "/prod/us", ScopeOfEffect: "/prod/us/east"},
				{ScopeOfOrigin: "/prod/us", ScopeOfEffect: "/prod/us"},
				{ScopeOfOrigin: "/prod/us/east", ScopeOfEffect: "/prod/us/east"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []EnforcementPoint
			for level := range EnforcementPointsForResourceScope(tt.resourceScope) {
				got = append(got, level)
			}

			require.Equal(t, tt.expect, got, "scope hierarchy levels should match expected order")
		})
	}
}

// TestScopeOfOriginAndEffect tests the relationship between Scope of Origin and Scope of Effect.
// A Scope of Effect must be equivalent to or descendant from its Scope of Origin (cannot reach up).
func TestScopeOfOriginAndEffect(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name       string
		origin     string
		effect     string
		assignable bool
	}{
		{
			name:       "equivalent root",
			origin:     "/",
			effect:     "/",
			assignable: true,
		},
		{
			name:       "root origin can assign to any child",
			origin:     "/",
			effect:     "/staging",
			assignable: true,
		},
		{
			name:       "root origin can assign to deep child",
			origin:     "/",
			effect:     "/staging/west/testbed",
			assignable: true,
		},
		{
			name:       "equivalent non-root",
			origin:     "/staging",
			effect:     "/staging",
			assignable: true,
		},
		{
			name:       "origin can assign to child",
			origin:     "/staging",
			effect:     "/staging/west",
			assignable: true,
		},
		{
			name:       "origin can assign to deep descendant",
			origin:     "/staging",
			effect:     "/staging/west/testbed",
			assignable: true,
		},
		{
			name:       "origin cannot assign to parent",
			origin:     "/staging/west",
			effect:     "/staging",
			assignable: false,
		},
		{
			name:       "origin cannot assign to root",
			origin:     "/staging",
			effect:     "/",
			assignable: false,
		},
		{
			name:       "origin cannot assign to orthogonal scope",
			origin:     "/staging",
			effect:     "/prod",
			assignable: false,
		},
		{
			name:       "origin cannot assign to orthogonal child",
			origin:     "/staging/west",
			effect:     "/staging/east",
			assignable: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// Test both directions of the relationship
			require.Equal(t, tt.assignable, ScopeOfOrigin(tt.origin).IsAssignableToScopeOfEffect(tt.effect),
				"ScopeOfOrigin(%q).IsAssignableToScopeOfEffect(%q)", tt.origin, tt.effect)

			require.Equal(t, tt.assignable, ScopeOfEffect(tt.effect).IsAssignableFromScopeOfOrigin(tt.origin),
				"ScopeOfEffect(%q).IsAssignableFromScopeOfOrigin(%q)", tt.effect, tt.origin)
		})
	}
}

// TestPolicyResourceScope tests the PolicyResourceScope helper, which represents the scope of a policy resource
// itself (e.g., the top-level scope field of a role or assignment). Policy resources can only depend on state
// from their own scope or ancestor scopes (configuration state flows down the hierarchy).
func TestPolicyResourceScope(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name          string
		resourceScope string
		stateScope    string
		canDepend     bool
	}{
		{
			name:          "can depend on self",
			resourceScope: "/staging",
			stateScope:    "/staging",
			canDepend:     true,
		},
		{
			name:          "can depend on parent",
			resourceScope: "/staging/west",
			stateScope:    "/staging",
			canDepend:     true,
		},
		{
			name:          "can depend on root",
			resourceScope: "/staging/west",
			stateScope:    "/",
			canDepend:     true,
		},
		{
			name:          "cannot depend on child",
			resourceScope: "/staging",
			stateScope:    "/staging/west",
			canDepend:     false,
		},
		{
			name:          "cannot depend on orthogonal",
			resourceScope: "/staging",
			stateScope:    "/prod",
			canDepend:     false,
		},
		{
			name:          "cannot depend on orthogonal descendant",
			resourceScope: "/staging/west",
			stateScope:    "/staging/east",
			canDepend:     false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.canDepend,
				PolicyResourceScope(tt.resourceScope).CanDependOnStateFromPolicyResourceAtScope(tt.stateScope),
				"PolicyResourceScope(%q).CanDependOnStateFromPolicyResourceAtScope(%q)",
				tt.resourceScope, tt.stateScope)
		})
	}
}

// TestScopeOfEffectGlob tests the ScopeOfEffectGlob helper, which wraps Glob for use specifically
// with Scope of Effect values. This is used when roles specify assignable_scopes globs.
func TestScopeOfEffectGlob(t *testing.T) {
	t.Parallel()

	t.Run("MatchesScopeOfEffectLiteral", func(t *testing.T) {
		tts := []struct {
			name    string
			glob    string
			literal string
			matches bool
		}{
			{
				name:    "exact match",
				glob:    "/staging/west",
				literal: "/staging/west",
				matches: true,
			},
			{
				name:    "child match",
				glob:    "/staging",
				literal: "/staging/west",
				matches: true,
			},
			{
				name:    "exclusive child glob match",
				glob:    "/staging/**",
				literal: "/staging/west",
				matches: true,
			},
			{
				name:    "exclusive child glob no match on parent",
				glob:    "/staging/**",
				literal: "/staging",
				matches: false,
			},
			{
				name:    "no match on orthogonal",
				glob:    "/staging",
				literal: "/prod",
				matches: false,
			},
		}

		for _, tt := range tts {
			t.Run(tt.name, func(t *testing.T) {
				require.Equal(t, tt.matches,
					ScopeOfEffectGlob(tt.glob).MatchesScopeOfEffectLiteral(tt.literal),
					"ScopeOfEffectGlob(%q).MatchesScopeOfEffectLiteral(%q)", tt.glob, tt.literal)
			})
		}
	})

	t.Run("IsAlwaysAssignableFromScopeOfOrigin", func(t *testing.T) {
		tts := []struct {
			name       string
			glob       string
			origin     string
			assignable bool
		}{
			{
				name:       "glob at same scope as origin",
				glob:       "/staging",
				origin:     "/staging",
				assignable: true,
			},
			{
				name:       "glob at child of origin",
				glob:       "/staging/west",
				origin:     "/staging",
				assignable: true,
			},
			{
				name:       "exclusive child glob ok",
				glob:       "/staging/**",
				origin:     "/staging",
				assignable: true,
			},
			{
				name:       "glob at ancestor cannot be assigned from child origin",
				glob:       "/staging",
				origin:     "/staging/west",
				assignable: false,
			},
			{
				name:       "exclusive child glob at ancestor cannot be assigned from child origin",
				glob:       "/staging/**",
				origin:     "/staging/west",
				assignable: false,
			},
			{
				name:       "glob at root cannot be assigned from child origin",
				glob:       "/",
				origin:     "/staging",
				assignable: false,
			},
			{
				name:       "exclusive child glob at root cannot be assigned from child origin",
				glob:       "/**",
				origin:     "/staging",
				assignable: false,
			},
			{
				name:       "orthogonal glob",
				glob:       "/prod",
				origin:     "/staging",
				assignable: false,
			},
			{
				name:       "exclusive child glob orthogonal",
				glob:       "/prod/**",
				origin:     "/staging",
				assignable: false,
			},
		}

		for _, tt := range tts {
			t.Run(tt.name, func(t *testing.T) {
				require.Equal(t, tt.assignable,
					ScopeOfEffectGlob(tt.glob).IsAlwaysAssignableFromScopeOfOrigin(tt.origin),
					"ScopeOfEffectGlob(%q).IsAlwaysAssignableFromScopeOfOrigin(%q)", tt.glob, tt.origin)
			})
		}
	})
}

// TestScopeHierarchyExample demonstrates a non-trivial scoping model with a realistic example.
func TestScopeHierarchyExample(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - a role is defined at /staging with assignable_scopes = ["/staging/west", "/staging/east"]
	// - an assignment assigns this role at /staging/west with effect at /staging/west
	// - a user with this assignment tries to access a resource at /staging/west/db1

	// role scoping
	roleOrigin := "/staging"
	assignableScopes := []string{
		"/staging/west",
		"/staging/east",
	}

	// assignment scoping
	assignmentOrigin := "/staging/west"
	assignmentEffect := "/staging/west"

	// resource scoping
	resourceScope := "/staging/west/db1"

	// role assignable scopes enforcment
	globMatched := false
	for _, assignableScope := range assignableScopes {
		if ScopeOfEffectGlob(assignableScope).MatchesScopeOfEffectLiteral(assignmentEffect) {
			globMatched = true
			break
		}
	}
	require.True(t, globMatched, "assignment effect %q should match one of the assignable scopes", assignmentEffect)

	// assignment effect/origin enforcement
	require.True(t, ScopeOfOrigin(assignmentOrigin).IsAssignableToScopeOfEffect(assignmentEffect),
		"assignment at %q should be able to assign effect at %q", assignmentOrigin, assignmentEffect)

	// assignment state dependency check
	require.True(t, PolicyResourceScope(assignmentOrigin).CanDependOnStateFromPolicyResourceAtScope(roleOrigin),
		"assignment at %q should be able to reference role at %q", assignmentOrigin, roleOrigin)

	// assignment applicability to resource check
	require.True(t, ScopeOfEffect(assignmentEffect).AppliesToResourceScope(resourceScope),
		"role with effect %q should apply to resource at %q", assignmentEffect, resourceScope)

	// resource subject to effect check (inverse of above/redundant)
	require.True(t, ResourceScope(resourceScope).IsSubjectToScopeOfEffect(assignmentEffect),
		"resource at %q should be subject to effect at %q", resourceScope, assignmentEffect)

	// checks that should fail (sanity check)

	// verify that the assignment cannot reach up to effect a parent scope
	require.False(t, ScopeOfOrigin(assignmentOrigin).IsAssignableToScopeOfEffect("/staging"),
		"assignment at %q should not be able to assign effect at parent %q", assignmentOrigin, "/staging")

	// verify that the assignment effect does not apply to an orthogonal resource
	require.False(t, ScopeOfEffect(assignmentEffect).AppliesToResourceScope("/staging/east/db2"),
		"role with effect %q should not apply to orthogonal resource", assignmentEffect)

	// verify that the assignment cannot depend on state from an orthogonal scope
	require.False(t, PolicyResourceScope(assignmentOrigin).CanDependOnStateFromPolicyResourceAtScope("/staging/east"),
		"assignment at %q should not be able to reference role at orthogonal scope", assignmentOrigin)
}
