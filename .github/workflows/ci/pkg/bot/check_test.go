package bot

import (
	"testing"

	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	bot := &Bot{Environment: &environment.Environment{}, invalidate: invalidateTest, verify: verifyTest}
	botNewCommit := &Bot{Environment: &environment.Environment{}, invalidate: invalidateTest, verify: verifyFileChange}
	pull := &environment.PullRequestMetadata{Author: "test"}
	tests := []struct {
		botInstance    *Bot
		isInternal     bool
		pr             *environment.PullRequestMetadata
		required       []string
		currentReviews []review
		desc           string
		checkErr       require.ErrorAssertionFunc
	}{
		{
			botInstance: bot,
			isInternal:  true,
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
			botInstance: botNewCommit,
			isInternal:  true,
			pr:          pull,
			required:    []string{"foo", "bar", "baz"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				{name: "bar", status: "APPROVED", commitID: "fe324c", id: 2},
				{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
			},
			desc:     "Internal contributor, PR does have required approvals, but commit shas do not match",
			checkErr: require.NoError,
		},
		{
			botInstance: botNewCommit,
			isInternal:  false,
			pr:          pull,
			required:    []string{"foo", "bar", "baz"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				{name: "bar", status: "APPROVED", commitID: "fe324c", id: 2},
				{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
			},
			desc:     "External contributor, PR has required approvals, but commit shas do not match (new commit pushed)",
			checkErr: require.Error,
		},
		{
			botInstance: bot,
			isInternal:  false,
			pr:          pull,
			required:    []string{"foo", "bar", "baz"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
				{name: "bar", status: "APPROVED", commitID: "12ga34", id: 2},
				{name: "baz", status: "APPROVED", commitID: "12ga34", id: 3},
			},
			desc:     "External contributor, PR has required approvals, commit shas match",
			checkErr: require.NoError,
		},
		{
			botInstance: bot,
			isInternal:  true,
			pr:          pull,
			required:    []string{"foo", "bar"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
				{name: "bar", status: "APPROVED", commitID: "fe324c", id: 2},
			},
			desc:     "Internal contributor, PR does have all required approvals, commit SHAs match",
			checkErr: require.NoError,
		},
		{
			botInstance: bot,
			isInternal:  true,
			pr:          pull,
			required:    []string{"foo", "bar"},
			currentReviews: []review{
				{name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
			},
			desc:     "Internal contributor, PR does not have all required approvals",
			checkErr: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.botInstance.check(test.isInternal, test.pr, test.required, test.currentReviews)
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
}

func verifyFileChange(repoOwner, repoName, base, head string) error {
	return trace.BadParameter("file change")
}

func verifyTest(repoOwner, repoName, base, head string) error {
	return nil
}

func invalidateTest(repoOwner, repoName, msg string, number int, reviews []review, clt *github.Client) error {
	return nil
}
