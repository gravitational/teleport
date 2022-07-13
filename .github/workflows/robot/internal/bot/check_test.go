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
				{Author: "user1", State: review.Approved},
				{Author: "user2", State: review.Approved},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "extra-approvals",
			reviewers: []string{"user1", "user2", "user3"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
				{Author: "user2", State: review.Approved},
				{Author: "user3", State: review.Approved},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "2-dismissals",
			reviewers: []string{"user1", "user2", "user3", "user4"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
				{Author: "user2", State: review.Approved},
			},
			assert:  require.NoError,
			dismiss: []string{"user3", "user4"},
		},
		{
			desc:      "2-approvals-1-comment",
			reviewers: []string{"user1", "user2", "user3"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
				{Author: "user2", State: review.Approved},
				{Author: "user3", State: review.Commented},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "2-approvals-1-requestchange-1-to-dismiss",
			reviewers: []string{"user1", "user2", "user3", "user4"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
				{Author: "user2", State: review.Approved},
				{Author: "user3", State: review.ChangesRequested},
			},
			assert:  require.NoError,
			dismiss: []string{"user4"},
		},
		{
			desc:      "1-admin-approval",
			reviewers: []string{"user1", "user2", "user3", "user4"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "1-admin-approval-1-external-approval",
			reviewers: []string{"user1", "user2", "user3", "user4", "external"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
				{Author: "external", State: review.Approved},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
		{
			desc:      "only-counts-latest-review",
			reviewers: []string{"user1", "user2", "user3", "user4"},
			reviews: []github.Review{
				{Author: "user1", State: review.Approved},
				{Author: "user2", State: review.Approved},
				{Author: "user2", State: review.ChangesRequested},
			},
			assert:  require.NoError,
			dismiss: nil,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			a, err := review.New(&review.Config{
				Admins: []string{"admin1"},
				CodeReviewers: map[string]review.Reviewer{
					"user1": {},
					"user2": {},
					"user3": {},
					"user4": {},
				},
				CodeReviewersOmit: make(map[string]bool),
				DocsReviewers:     make(map[string]review.Reviewer),
				DocsReviewersOmit: make(map[string]bool),
			})
			require.NoError(t, err)

			b := &Bot{
				c: &Config{
					Environment: &env.Environment{},
					GitHub: &fakeGithub{
						reviewers: test.reviewers,
						reviews:   test.reviews,
					},
					Review: a,
				},
			}

			toDismiss, err := b.reviewersToDismiss(context.Background())
			test.assert(t, err)
			require.Equal(t, test.dismiss, toDismiss)
		})
	}
}
