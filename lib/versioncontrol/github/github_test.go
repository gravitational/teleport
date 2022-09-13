/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package github

import (
	_ "embed"
	"fmt"
	"os"
	"testing"

	apiutils "github.com/gravitational/teleport/api/utils"

	"github.com/stretchr/testify/require"
)

// TestGithubAPI tests the github releases iterator against the real github
// api. this test is disabled by default since we use the unauthenticated endpoint
// which doesn't like frequent request spamming.
func TestGithubAPI(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("TEST_GITHUB_API")); !run {
		t.Skip("Test disabled in CI. Enable it by setting env variable TEST_GITHUB_API=yes")
	}
	t.Parallel()

	var rr []Release
	var iter Iterator
	iter.halt = "v4"
	for iter.Next() {
		rr = append(rr, iter.Page()...)
	}
	require.NoError(t, iter.Error())

	seen := make(map[string]struct{})
	for _, r := range rr {
		seen[r.Version] = struct{}{}
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
		for iter.Next() {
			ct++
			require.NotZero(t, len(iter.Page()), tt.desc)
		}

		if tt.err == nil {
			require.NoError(t, iter.Error(), tt.desc)
		} else {
			require.Error(t, iter.Error(), tt.desc)
		}

		require.Equal(t, tt.pageCount, ct, tt.desc)

		require.Equal(t, tt.hitsEmptyPage, lastPageWasEmpty)

	}
}

func TestLatestStable(t *testing.T) {
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

		latest, err := latestStable(iter, "v1.2.3")
		require.NoError(t, err)

		require.Equal(t, tt.expect, latest)
	}
}
