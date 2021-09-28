package bot

import (
	"testing"

	"github.com/gravitational/teleport/tool/ci/pkg/environment"

	"github.com/stretchr/testify/require"
)

func TestApproved(t *testing.T) {
	bot := &Bot{Environment: &environment.PullRequestEnvironment{}}
	pull := &environment.Metadata{Author: "test"}
	tests := []struct {
		botInstance    *Bot
		pr             *environment.Metadata
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
			err := approved(test.currentReviews, test.required)
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
	_, ok := hasApproved("bar", reviews)
	require.Equal(t, false, ok)

	// Does not have revire from reviewer
	_, ok = hasApproved("car", reviews)
	require.Equal(t, false, ok)

	// Has review and is approved
	_, ok = hasApproved("foo", reviews)
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
