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
		})
	}
}

// TestPolicyAndResourceScope tests the relationship between policy and resource scopes helpers
// given various combinations of policy and resource scopes.
func TestPolicyAndResourceScope(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name      string
		privilege string
		resource  string
		applies   bool
	}{
		{
			name:      "simple root",
			privilege: "/",
			resource:  "/",
			applies:   true,
		},
		{
			name:      "simple root privilege",
			privilege: "/",
			resource:  "/aa",
			applies:   true,
		},
		{
			name:      "simple root resource",
			privilege: "/aa",
			resource:  "/",
			applies:   false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.applies, PolicyScope(tt.privilege).AppliesToResourceScope(tt.resource),
				"PolicyScope(%q).AppliesToResourceScope(%q)", tt.privilege, tt.resource)

			require.Equal(t, tt.applies, ResourceScope(tt.resource).IsSubjectToPolicyScope(tt.privilege),
				"ResourceScope(%q).IsSubjectToPolicyScope(%q)", tt.resource, tt.privilege)
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
			require.Equal(t, tt.match, Glob(tt.glob).Matches(tt.scope),
				"Glob(%q).Matches(%q)", tt.glob, tt.scope)
		})
	}
}

func TestGlobIsSubjectToPolicyResourceScope(t *testing.T) {
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
			require.Equal(t, tt.subject, Glob(tt.glob).IsSubjectToPolicyResourceScope(tt.scope),
				"Glob(%q).IsSubjectToPolicyResourceScope(%q)", tt.glob, tt.scope)
		})
	}
}
