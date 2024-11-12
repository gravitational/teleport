/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package github

import (
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	apiutils "github.com/gravitational/teleport/api/utils"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
)

// TestGithubAPI tests the github releases iterator against the real github
// api. this test is disabled by default since we use the unauthenticated endpoint
// which doesn't like frequent request spamming.
func TestGithubAPI(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("TEST_GITHUB_API")); !run {
		t.Skip("Test disabled in CI. Enable it by setting env variable TEST_GITHUB_API=yes")
	}
	t.Parallel()

	var rr []vc.Target
	var iter Iterator
	iter.halt = "v4"
	for iter.Next() {
		rr = append(rr, iter.Page()...)
	}
	require.NoError(t, iter.Error())

	seen := make(map[string]struct{})
	for _, r := range rr {
		seen[r.Version()] = struct{}{}
	}

	// some arbitrary versions we expect to exist (this may need to be updated
	// at some point in the future if we ever start trimming history).
	expected := []string{
		"v8.0.0-beta.2",
		"v7.3.19",
		"v7.1.1",
		"v10.0.0",
		"v5.0.0-rc.2",
		"v7.3.21",
		"v9.3.12",
	}
	for _, e := range expected {
		require.Contains(t, seen, e)
	}

	// some known tags that we aught to be filtering out
	notExpected := []string{
		"teleport-connect-preview-1.0.2",
		"teleport-connect-preview-1.0.1",
	}
	for _, n := range notExpected {
		require.NotContains(t, seen, n)
	}
}

// TestGithubAPIError tests the api error message extraction.
func TestGithubAPIError(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("TEST_GITHUB_API")); !run {
		t.Skip("Test disabled in CI. Enable it by setting env variable TEST_GITHUB_API=yes")
	}
	t.Parallel()

	var iter Iterator
	// any page size + number combo that would look further than 1000 releases
	// into the past should generate an error.
	iter.n = 20
	iter.size = 100
	for iter.Next() {
		require.Fail(t, "request should fail on first iteration")
	}
	require.Error(t, iter.Error())

	require.Contains(t, iter.Error().Error(), "Only the first 1000 results are available. (422)")
}

//go:embed test_page_1.json
var page1 string

//go:embed test_page_2.json
var page2 string

//go:embed test_page_3.json
var page3 string

const cachedPageSize = 10

// TestCachedReleases verifies expected behavior of the github release iterator
// using cached pages.
func TestCachedReleases(t *testing.T) {
	tts := []struct {
		pages         []string
		pageCount     int
		secCount      int
		desc          string
		hitsEmptyPage bool
		haltingPoint  string
		err           error
	}{
		{
			pages: []string{
				page1,
				page2,
				page3, // page 3 is a partial page, which triggers iteration to halt
			},
			pageCount: 3,
			secCount:  1,
			desc:      "full iteration, ending on partial page",
		},
		{
			pages: []string{
				page3,
				page2,
				page1,
			},
			pageCount: 1,
			desc:      "stop on first partial page",
		},
		{
			pages: []string{
				page1,
				page2,
			},
			pageCount:     2,
			secCount:      1,
			hitsEmptyPage: true,
			desc:          "end on full page",
		},
		{
			pages: []string{
				page1,
				page2,
				page3,
			},
			pageCount:    1,
			secCount:     1,
			haltingPoint: "v10",
			desc:         "halt on fist v10.0.X release",
		},
		{
			pages: []string{
				page1,
				page2,
				page3,
			},
			pageCount:    2,
			secCount:     1,
			haltingPoint: "v7.3",
			desc:         "halt on fist v7.3.X release",
		},
		{
			hitsEmptyPage: true,
			desc:          "empty case",
		},
		{
			err:  fmt.Errorf("failure"),
			desc: "error case",
		},
	}

	for _, tt := range tts {
		lastPageWasEmpty := false

		var iter Iterator
		iter.size = cachedPageSize
		iter.halt = tt.haltingPoint
		iter.getPage = func(n, size int) ([]release, error) {
			require.NotZero(t, n, tt.desc)
			if tt.err != nil {
				return nil, tt.err
			}
			if n > len(tt.pages) {
				lastPageWasEmpty = true
				return nil, nil
			}
			page := tt.pages[n-1]
			return parsePage([]byte(page))
		}

		var ct int
		var sec int
		for iter.Next() {
			ct++
			require.NotEmpty(t, iter.Page(), tt.desc)
			for _, target := range iter.Page() {
				if target.SecurityPatch() {
					sec++
				}
			}
		}

		if tt.err == nil {
			require.NoError(t, iter.Error(), tt.desc)
		} else {
			require.Error(t, iter.Error(), tt.desc)
		}

		require.Equal(t, tt.pageCount, ct, tt.desc)

		require.Equal(t, tt.secCount, sec, tt.desc)

		require.Equal(t, tt.hitsEmptyPage, lastPageWasEmpty)

	}
}

func TestVisit(t *testing.T) {
	tts := []struct {
		pages  []string
		expect string
		desc   string
	}{
		{
			pages: []string{
				page1,
				page2,
				page3,
			},
			expect: "v10.0.2",
			desc:   "all cached pages",
		},
		{
			pages: []string{
				page2,
				page3,
			},
			expect: "v9.3.7",
			desc:   "earlier pages subset",
		},
		{
			pages: []string{
				page1,
			},
			expect: "v10.0.2",
			desc:   "page 1 only",
		},
		{
			pages: []string{
				page2,
			},
			expect: "v9.3.7",
			desc:   "page 2 only",
		},
		{
			pages: []string{
				page3,
			},
			expect: "v9.2.4",
			desc:   "page 3 only",
		},
	}

	for _, tt := range tts {
		var iter Iterator
		iter.size = cachedPageSize
		iter.getPage = func(n, size int) ([]release, error) {
			if n > len(tt.pages) {
				return nil, nil
			}
			page := tt.pages[n-1]
			return parsePage([]byte(page))
		}

		visitor := vc.Visitor{
			Current: vc.NewTarget("v1.2.3"),
		}

		require.NoError(t, visit(iter, &visitor), tt.desc)

		require.Equal(t, tt.expect, visitor.Newest().Version(), tt.desc)
	}
}

func TestLabelParse(t *testing.T) {
	tts := []struct {
		notes  string
		expect map[string]string
		desc   string
	}{
		{
			notes: "Labels: Spam=Eggs, Foo=Bar",
			expect: map[string]string{
				"spam": "eggs",
				"foo":  "bar",
			},
			desc: "normalize caps and spaces",
		},
		{
			notes: "labels: security-patch=yes, security-patch-alts=v1.2.3|v1.2.4",
			expect: map[string]string{
				vc.LabelSecurityPatch:     "yes",
				vc.LabelSecurityPatchAlts: "v1.2.3|v1.2.4",
			},
			desc: "real-world label set",
		},
		{
			notes: "labels: hello=world, greeting='Hey there! how are you?', , count=7",
			expect: map[string]string{
				"hello": "world",
				"count": "7",
			},
			desc: "ignore invalid and empty pairs",
		},
		{
			notes: `
## Heading
- list1
- list2

not real: "labels: some-label=some-val,other-label=other-val"

---
Labels: security-patch=yes
other notes`,
			expect: map[string]string{
				"security-patch": "yes",
			},
			desc: "ignore non-label lines",
		},
		{
			notes: "labels: invalid_key=valid-val, valid-key=@invalid-val, a=b",
			expect: map[string]string{
				"a": "b",
			},
			desc: "skip labels with invalid characters",
		},
		{
			notes: "labels foo=bar",
			desc:  "no valid labels line",
		},
		{
			desc: "empty release notes",
		},
	}

	for _, tt := range tts {
		// require.Equal doesn't think nil map is the same as empty map
		if tt.expect == nil {
			tt.expect = map[string]string{}
		}
		require.Equal(t, tt.expect, parseReleaseNoteLabels(tt.notes), tt.desc)
	}
}
