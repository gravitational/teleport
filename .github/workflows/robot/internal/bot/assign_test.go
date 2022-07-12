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
				Author:     "baz",
				Repository: "bar",
				UnsafeHead: github.Branch{
					Ref: "baz/fix",
				},
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
				Author:     "baz",
				Repository: "bar",
				UnsafeHead: github.Branch{
					Ref: "baz/fix",
				},
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
				Author:     "baz",
				Repository: "bar",
				UnsafeHead: github.Branch{
					Ref: "baz/fix",
				},

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
				Author:     "baz",
				Repository: "bar",
				UnsafeHead: github.Branch{
					Ref: "baz/fix",
				},
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

func TestDismissUnnecessaryReviewers(t *testing.T) {
	for _, test := range []struct {
		desc      string
		reviewers []string
		reviews   []github.Review
		assert    require.ErrorAssertionFunc
		dismiss   []string
	}{
		{
			desc:      "exactly-2-approvals",
			reviewers: []string{"user1", "user2"},
			reviews: []github.Review{
				{Author: "user1", State: "APPROVED"},
				{Author: "user2", State: "APPROVED"},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "extra-approvals",
			reviewers: []string{"user1", "user2", "user3"},
			reviews: []github.Review{
				{Author: "user1", State: "APPROVED"},
				{Author: "user2", State: "APPROVED"},
				{Author: "user3", State: "APPROVED"},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "2-dismissals",
			reviewers: []string{"user1", "user2", "user3", "user4"},
			reviews: []github.Review{
				{Author: "user1", State: "APPROVED"},
				{Author: "user2", State: "APPROVED"},
			},
			assert:  require.NoError,
			dismiss: []string{"user3", "user4"},
		},
		{
			desc:      "2-approvals-1-comment",
			reviewers: []string{"user1", "user2", "user3"},
			reviews: []github.Review{
				{Author: "user1", State: "APPROVED"},
				{Author: "user2", State: "APPROVED"},
				{Author: "user3", State: "COMMENTED"},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "2-approvals-1-requestchange-1-to-dismiss",
			reviewers: []string{"user1", "user2", "user3", "user4"},
			reviews: []github.Review{
				{Author: "user1", State: "APPROVED"},
				{Author: "user2", State: "APPROVED"},
				{Author: "user3", State: "CHANGES_REQUESTED"},
			},
			assert:  require.NoError,
			dismiss: []string{"user4"},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			b := &Bot{
				c: &Config{
					Environment: &env.Environment{},
					GitHub: &fakeGithub{
						reviewers: test.reviewers,
						reviews:   test.reviews,
					},
				},
			}

			toDismiss, err := b.reviewersToDismiss(context.Background())
			test.assert(t, err)
			require.Equal(t, test.dismiss, toDismiss)
		})
	}
}
