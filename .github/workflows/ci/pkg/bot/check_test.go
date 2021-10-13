package bot

import (
	"sort"
	"testing"
	"time"

	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

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
			err := hasRequiredApprovals(test.currentReviews, test.required)
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
	ok := hasApproved("bar", reviews)
	require.Equal(t, false, ok)

	// Does not have revire from reviewer
	ok = hasApproved("car", reviews)
	require.Equal(t, false, ok)

	// Has review and is approved
	ok = hasApproved("foo", reviews)
	require.Equal(t, true, ok)
}

func TestHasNewCommit(t *testing.T) {
	reviews := []review{
		{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
		{name: "foo", status: "APPROVED", commitID: "fe324c", id: 4},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 5},
	}
	valid, obs := splitReviews("fe324c", reviews)
	expectedValid := []review{
		{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		{name: "foo", status: "APPROVED", commitID: "fe324c", id: 4},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 5},
	}
	expectedObsolete := []review{
		{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		{name: "baz", status: "APPROVED", commitID: "ba0d35", id: 3},
	}
	require.Equal(t, expectedValid, valid)
	require.Equal(t, expectedObsolete, obs)
}

func TestHasRequiredApprovals(t *testing.T) {
	reviews := []review{
		{name: "foo", status: "APPROVED", commitID: "12ga34", id: 1},
		{name: "bar", status: "APPROVED", commitID: "ba0d35", id: 3},
	}
	required := []string{"foo", "bar"}
	err := hasRequiredApprovals(reviews, required)
	require.NoError(t, err)

	reviews = []review{
		{name: "foo", status: "APPROVED", commitID: "fe324c", id: 1},
		{name: "bar", status: "Commented", commitID: "fe324c", id: 2},
		{name: "baz", status: "APPROVED", commitID: "fe324c", id: 3},
	}
	required = []string{"foo", "reviewer"}
	err = hasRequiredApprovals(reviews, required)
	require.Error(t, err)

}

func TestMostRecent(t *testing.T) {
	time := time.Now()
	timePlus10 := time.Add(10)
	tests := []struct {
		currentReviews []review
		expectedOutput []review
	}{
		{
			currentReviews: []review{
				{name: "boo", submittedAt: &time},
				{name: "boo", submittedAt: &timePlus10},
				{name: "test", submittedAt: &time}},
			expectedOutput: []review{
				{name: "boo", submittedAt: &timePlus10},
				{name: "test", submittedAt: &time}},
		},
		{
			currentReviews: []review{
				{name: "boo", submittedAt: &time},
				{name: "boo", submittedAt: &timePlus10},
				{name: "test", submittedAt: &time},
				{name: "test", submittedAt: &timePlus10},
				{name: "bar", submittedAt: &time},
			},

			expectedOutput: []review{
				{name: "bar", submittedAt: &time},
				{name: "boo", submittedAt: &timePlus10},
				{name: "test", submittedAt: &timePlus10},
			},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			expected := test.expectedOutput
			sort.Slice(expected, func(i, j int) bool {
				return expected[i].name < expected[j].name
			})

			revs := mostRecent(test.currentReviews)
			sort.Slice(revs, func(i, j int) bool {
				return revs[i].name < revs[j].name
			})
			require.Equal(t, expected, revs)
		})
	}
}
