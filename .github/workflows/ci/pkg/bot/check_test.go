package bot

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	bot := &Bot{Environment: &environment.Environment{}}
	pull := &environment.PullRequestMetadata{Author: "test"}
	tests := []struct {
		botInstance    *Bot
		pr             *environment.PullRequestMetadata
		required       []string
		currentReviews []review
		desc           string
		checkErr       require.ErrorAssertionFunc
	}{
		{
			botInstance: bot,
			pr:          pull,
			required:    []string{"foo", "bar", "baz"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
				{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
			},
			desc:     "PR does not have all required approvals",
			checkErr: require.Error,
		},
		{
			botInstance: bot,

			pr:       pull,
			required: []string{"foo", "bar", "baz"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				{name: "bar", status: "APPROVED", commitID: "12ga34", id: 2},
				{name: "baz", status: "APPROVED", commitID: "12ga34", id: 3},
			},
			desc:     "PR has required approvals, commit shas match",
			checkErr: require.NoError,
		},
		{
			botInstance: bot,
			pr:          pull,
			required:    []string{"foo", "bar"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
			},
			desc:     "PR does not have all required approvals",
			checkErr: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.botInstance.check(context.TODO(), test.pr, test.required, test.currentReviews)
			test.checkErr(t, err)
		})
	}
}

func TestContainsApprovalReview(t *testing.T) {
	reviews := []review{
		{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 1},
	}
	// Has a review but no approval
	ok := containsApprovalReview("bar", reviews)
	require.Equal(t, false, ok)

	// Does not have revire from reviewer
	ok = containsApprovalReview("car", reviews)
	require.Equal(t, false, ok)

	// Has review and is approved
	ok = containsApprovalReview("foo", reviews)
	require.Equal(t, true, ok)
}

func TestHasNewCommit(t *testing.T) {
	reviews := []review{
		{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
	}
	ok := hasNewCommit("fe324e", reviews)
	require.Equal(t, true, ok)

	reviews = []review{
		{name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		{name: "baz", status: "APPROVED", commitID: "fe324c", id: 3},
	}
	ok = hasNewCommit("fe324c", reviews)
	require.Equal(t, false, ok)
	reviews = []review{
		{name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
		{name: "bar", status: "APPROVED", commitID: "fe324c", id: 2},
		{name: "baz", status: "APPROVED", commitID: "fe324c", id: 3},
	}
	ok = hasNewCommit("fe324d", reviews)
	require.Equal(t, true, ok)
}
