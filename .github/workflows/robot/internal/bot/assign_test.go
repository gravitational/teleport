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

package bot

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/env"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/review"

	"github.com/stretchr/testify/require"
)

// TestBackportReviewers checks if backport reviewers are correctly assigned.
func TestBackportReviewers(t *testing.T) {
	r, err := review.New(&review.Config{
		CodeReviewers:     map[string]review.Reviewer{},
		CodeReviewersOmit: map[string]bool{},
		DocsReviewers:     map[string]review.Reviewer{},
		DocsReviewersOmit: map[string]bool{},
		Admins:            []string{},
	})
	require.NoError(t, err)

	tests := []struct {
		desc      string
		pull      github.PullRequest
		reviewers []string
		reviews   []github.Review
		err       bool
		expected  []string
	}{
		{
			desc: "backport-original-pr-number-approved",
			pull: github.PullRequest{
				Author:      "baz",
				Repository:  "bar",
				UnsafeHead:  "baz/fix",
				UnsafeTitle: "Backport #0 to branch/v8",
				UnsafeBody:  "",
				Fork:        false,
			},
			reviewers: []string{"3"},
			reviews: []github.Review{
				{Author: "4", State: "APPROVED"},
			},
			err:      false,
			expected: []string{"3", "4"},
		},
		{
			desc: "backport-original-url-approved",
			pull: github.PullRequest{
				Author:      "baz",
				Repository:  "bar",
				UnsafeHead:  "baz/fix",
				UnsafeTitle: "Fixed an issue",
				UnsafeBody:  "https://github.com/gravitational/teleport/pull/0",
				Fork:        false,
			},
			reviewers: []string{"3"},
			reviews: []github.Review{
				{Author: "4", State: "APPROVED"},
			},
			err:      false,
			expected: []string{"3", "4"},
		},
		{
			desc: "backport-multiple-reviews",
			pull: github.PullRequest{
				Author:      "baz",
				Repository:  "bar",
				UnsafeHead:  "baz/fix",
				UnsafeTitle: "Fixed feature",
				UnsafeBody:  "",
				Fork:        false,
			},
			reviewers: []string{"3"},
			reviews: []github.Review{
				{Author: "4", State: "COMMENTED"},
				{Author: "4", State: "CHANGES_REQUESTED"},
				{Author: "4", State: "APPROVED"},
				{Author: "9", State: "APPROVED"},
			},
			err:      true,
			expected: []string{},
		},
		{
			desc: "backport-original-not-found",
			pull: github.PullRequest{
				Author:      "baz",
				Repository:  "bar",
				UnsafeHead:  "baz/fix",
				UnsafeTitle: "Fixed feature",
				UnsafeBody:  "",
				Fork:        false,
			},
			reviewers: []string{"3"},
			reviews: []github.Review{
				{Author: "4", State: "APPROVED"},
			},
			err:      true,
			expected: []string{},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			b := &Bot{
				c: &Config{
					Environment: &env.Environment{
						Organization: "foo",
						Author:       "9",
						Repository:   "bar",
						Number:       0,
						UnsafeBase:   "branch/v8",
						UnsafeHead:   "fix",
					},
					Review: r,
					GitHub: &fakeGithub{
						pull:      test.pull,
						reviewers: test.reviewers,
						reviews:   test.reviews,
					},
				},
			}
			reviewers, err := b.backportReviewers(context.Background())
			require.Equal(t, err != nil, test.err)
			require.ElementsMatch(t, reviewers, test.expected)
		})
	}
}
