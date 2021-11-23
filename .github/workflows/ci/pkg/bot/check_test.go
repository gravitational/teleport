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
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

	"github.com/stretchr/testify/require"
)

func TestApproved(t *testing.T) {
	bot := &Bot{Config: Config{Environment: &environment.PullRequestEnvironment{}}}
	pull := &environment.Metadata{Author: "test"}
	tests := []struct {
		botInstance    *Bot
		pr             *environment.Metadata
		required       []string
		currentReviews map[string]review
		desc           string
		checkErr       require.ErrorAssertionFunc
	}{
		{
			botInstance: bot,
			pr:          pull,
			required:    []string{"foo", "bar", "baz"},
			currentReviews: map[string]review{
				"foo": {name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				"bar": {name: "bar", status: "Commented", commitID: "fe324c", id: 2},
				"baz": {name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
			},
			desc:     "PR does not have all required approvals",
			checkErr: require.Error,
		},
		{
			botInstance: bot,

			pr:       pull,
			required: []string{"foo", "bar", "baz"},
			currentReviews: map[string]review{
				"foo": {name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				"bar": {name: "bar", status: "APPROVED", commitID: "12ga34", id: 2},
				"baz": {name: "baz", status: "APPROVED", commitID: "12ga34", id: 3},
			},
			desc:     "PR has required approvals, commit shas match",
			checkErr: require.NoError,
		},
		{
			botInstance: bot,
			pr:          pull,
			required:    []string{"foo", "bar"},
			currentReviews: map[string]review{
				"foo": {name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
			},
			desc:     "PR does not have all required approvals",
			checkErr: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := hasRequiredApprovals(test.currentReviews, test.required)
			test.checkErr(t, err)
		})
	}
}

func TestContainsApprovalReview(t *testing.T) {
	reviews := map[string]review{
		"foo": {name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		"bar": {name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		"baz": {name: "baz", status: "APPROVED", commitID: "ba0d35", id: 1},
	}
	// Has a review but no approval
	ok := hasApproved("bar", reviews)
	require.Equal(t, false, ok)

	// Does not have revire from reviewer
	ok = hasApproved("car", reviews)
	require.Equal(t, false, ok)

	// Has review and is approved
	ok = hasApproved("foo", reviews)
	require.Equal(t, true, ok)
}

func TestSplitReviews(t *testing.T) {
	reviews := map[string]review{
		"foo": {name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		"bar": {name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		"baz": {name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
	}
	valid, obs := splitReviews("fe324c", reviews)
	expectedValid := map[string]review{
		"bar": {name: "bar", status: "Commented", commitID: "fe324c", id: 2},
	}
	expectedObsolete := map[string]review{
		"foo": {name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		"baz": {name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
	}
	require.Equal(t, expectedValid, valid)
	require.Equal(t, expectedObsolete, obs)
}

func TestHasRequiredApprovals(t *testing.T) {
	reviews := map[string]review{
		"foo": {name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		"bar": {name: "bar", status: "APPROVED", commitID: "ba0d35", id: 3},
	}
	required := []string{"foo", "bar"}
	err := hasRequiredApprovals(reviews, required)
	require.NoError(t, err)

	reviews = map[string]review{
		"foo": {name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
		"bar": {name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		"baz": {name: "baz", status: "APPROVED", commitID: "fe324c", id: 3},
	}
	required = []string{"foo", "reviewer"}
	err = hasRequiredApprovals(reviews, required)
	require.Error(t, err)

}

func TestGetStaleReviews(t *testing.T) {
	metadata := &environment.Metadata{Author: "quinqu",
		RepoName:  "test-name",
		RepoOwner: "test-owner",
		HeadSHA:   "ecabd9d",
	}
	env := &environment.PullRequestEnvironment{Metadata: metadata}

	bot := Bot{Config: Config{Environment: env}}
	tests := []struct {
		mockC    mockCommitComparer
		reviews  map[string]review
		expected []string
		desc     string
	}{
		{
			mockC: mockCommitComparer{},
			reviews: map[string]review{
				"foo": {commitID: "ReviewHasFileChangeFromHead", name: "foo"},
				"bar": {commitID: "ReviewHasFileChangeFromHead", name: "bar"},
			},
			expected: []string{"foo", "bar"},
			desc:     "All pull request reviews are stale.",
		},
		{
			mockC: mockCommitComparer{},
			reviews: map[string]review{
				"foo": {commitID: "ecabd94", name: "foo"},
				"bar": {commitID: "abcde67", name: "bar"},
			},
			expected: []string{},
			desc:     "Pull request has no stale reviews.",
		},
		{
			mockC: mockCommitComparer{},
			reviews: map[string]review{
				"foo":  {commitID: "ReviewHasFileChangeFromHead", name: "foo"},
				"bar":  {commitID: "ReviewHasFileChangeFromHead", name: "bar"},
				"fizz": {commitID: "ecabd9d", name: "fizz"},
			},
			expected: []string{"foo", "bar"},
			desc:     "Pull request has two stale reviews.",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			bot.compareCommits = &test.mockC
			staleReviews, _ := bot.getStaleReviews(context.TODO(), test.reviews)
			for _, name := range test.expected {
				_, ok := staleReviews[name]
				require.Equal(t, true, ok)
			}
			require.Equal(t, len(test.expected), len(staleReviews))
		})
	}

}

func TestGetMostRecentReviews(t *testing.T) {
	// Bot setup.
	metadata := &environment.Metadata{Author: "quinqu",
		RepoName:  "test-name",
		RepoOwner: "test-owner",
		HeadSHA:   "ecabd9d",
		Number:    1,
	}
	env := &environment.PullRequestEnvironment{Metadata: metadata}
	cfg := Config{Environment: env, listReviews: &mockReviewLister{}}
	bot := Bot{Config: cfg}
	// Test login usernames.
	testLogin1 := "test-reviewer1"
	testLogin2 := "test-reviewer2"
	// Review state types.
	typeCommented := "COMMENTED"
	typeApproved := "APPROVED"
	// Test IDs.
	testCommitID := "commitID"
	testID := int64(1)
	// Test times.
	testSubmittedAtNow := time.Now()
	testSubmittedAtMostRecent := testSubmittedAtNow.Add(10 * time.Minute)

	tests := []struct {
		reviewList mockReviewLister
		expected   map[string]review
		desc       string
	}{
		{
			reviewList: mockReviewLister{reviews: []*github.PullRequestReview{
				{
					User:        &github.User{Login: &testLogin1},
					State:       &typeCommented,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtMostRecent,
				},
				{
					User:        &github.User{Login: &testLogin2},
					State:       &typeCommented,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtMostRecent,
				},
			},
			},
			expected: map[string]review{},
			desc:     "All pull request review statuses are COMMENTED.",
		},
		{
			reviewList: mockReviewLister{reviews: []*github.PullRequestReview{
				{
					User:        &github.User{Login: &testLogin1},
					State:       &typeCommented,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtMostRecent,
				},
				{
					User:        &github.User{Login: &testLogin2},
					State:       &typeApproved,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtMostRecent,
				},
				{
					User:        &github.User{Login: &testLogin2},
					State:       &typeCommented,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtNow,
				},
			},
			},
			expected: map[string]review{
				testLogin2: {name: testLogin2, status: typeApproved, commitID: testCommitID, id: testID, submittedAt: &testSubmittedAtMostRecent},
			},
			desc: "Pull request has a commented review and approved review by the same reviewer, the only one in the result should be the approved review. ",
		},
		{
			reviewList: mockReviewLister{reviews: []*github.PullRequestReview{
				{
					User:        &github.User{Login: &testLogin1},
					State:       &typeApproved,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtNow,
				},
				{
					User:        &github.User{Login: &testLogin2},
					State:       &typeApproved,
					CommitID:    &testCommitID,
					ID:          &testID,
					SubmittedAt: &testSubmittedAtMostRecent,
				},
			},
			},
			expected: map[string]review{
				testLogin1: {name: testLogin1, status: typeApproved, commitID: testCommitID, id: testID, submittedAt: &testSubmittedAtNow},
				testLogin2: {name: testLogin2, status: typeApproved, commitID: testCommitID, id: testID, submittedAt: &testSubmittedAtMostRecent},
			},
			desc: "All pull request review statuses are APPROVED.",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			bot.listReviews = &test.reviewList
			revs, err := bot.getMostRecentReviews(context.TODO())
			require.Nil(t, err)
			require.EqualValues(t, test.expected, revs)
		})
	}

}

type mockCommitComparer struct {
}

func (m *mockCommitComparer) CompareCommits(ctx context.Context, repoOwner, repoName, base, head string) (*github.CommitsComparison, *github.Response, error) {
	// FOR TESTS ONLY: Using the string "ReviewHasFileChangeFromHead" as an indicator that this test method should
	// return a non-empty CommitFile list in the CommitComparison.
	if base == "ReviewHasFileChangeFromHead" {
		return &github.CommitsComparison{Files: []*github.CommitFile{{}, {}}}, nil, nil
	}
	return &github.CommitsComparison{Files: []*github.CommitFile{}}, nil, nil
}

type mockReviewLister struct {
	reviews []*github.PullRequestReview
}

func (m *mockReviewLister) ListReviews(ctx context.Context, repoOwner, repoName string, number int, listOptions *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error) {
	return m.reviews, nil, nil
}
