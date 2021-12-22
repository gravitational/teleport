/*
Copyright 2021 Gravitational, Inc.

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

package bot

import (
	"testing"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/stretchr/testify/require"
)

// TestFilter checks if the filtering correctly filters out non-eligible PRs.
func TestFilter(t *testing.T) {
	pulls := []github.PullRequest{
		// From fork.
		github.PullRequest{
			Author:        "foo",
			Repository:    "bar",
			Number:        1,
			UnsafeHeadRef: "baz/qux",
			HeadSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			UnsafeBaseRef: "master",
			BaseSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			Fork:          true,
			AutoMerge:     true,
		},
		// Not master.
		github.PullRequest{
			Author:        "foo",
			Repository:    "bar",
			Number:        2,
			UnsafeHeadRef: "baz/qux",
			HeadSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			UnsafeBaseRef: "branch/v0",
			BaseSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			Fork:          false,
			AutoMerge:     true,
		},
		// No auto merge.
		github.PullRequest{
			Author:        "foo",
			Repository:    "bar",
			Number:        3,
			UnsafeHeadRef: "baz/qux",
			HeadSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			UnsafeBaseRef: "master",
			BaseSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			Fork:          false,
			AutoMerge:     false,
		},
		// Up to date.
		github.PullRequest{
			Author:        "foo",
			Repository:    "bar",
			Number:        4,
			UnsafeHeadRef: "baz/qux",
			HeadSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			UnsafeBaseRef: "master",
			BaseSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			Fork:          false,
			AutoMerge:     true,
		},
		// No up to date.
		github.PullRequest{
			Author:        "foo",
			Repository:    "bar",
			Number:        5,
			UnsafeHeadRef: "baz/qux",
			HeadSHA:       "0000000000000000000000000000000000000000000000000000000000000001",
			UnsafeBaseRef: "master",
			BaseSHA:       "0000000000000000000000000000000000000000000000000000000000000002",
			Fork:          false,
			AutoMerge:     true,
		},
	}
	master := github.Branch{
		UnsafeName: "master",
		SHA:        "0000000000000000000000000000000000000000000000000000000000000001",
	}

	filtered, err := filter(pulls, master)
	require.NoError(t, err)
	require.ElementsMatch(t, filtered, []int{5})
}
