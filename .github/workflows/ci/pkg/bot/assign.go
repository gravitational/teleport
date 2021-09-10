package bot

import (
	"context"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/trace"
)

// Assign assigns reviewers to the pull request
func (a *Bot) Assign(ctx context.Context) error {
	pullReq := a.Environment.PullRequest
	// Getting and setting reviewers for author of pull request
	r := a.Environment.GetReviewersForAuthor(pullReq.Author)
	client := a.Environment.Client
	// Assigning reviewers to pull request
	pr, _, err := client.PullRequests.RequestReviewers(ctx,
		pullReq.RepoOwner,
		pullReq.RepoName, pullReq.Number,
		github.ReviewersRequest{Reviewers: r})
	if err != nil {
		return trace.Wrap(err)
	}
	return a.assign(r, pr.RequestedReviewers)
}

// assign verifies reviewers are assigned
func (a *Bot) assign(required []string, currentAssignedReviewers []*github.User) error {
	for _, requiredReviewer := range required {
		if ok := containsRequiredReviewer(currentAssignedReviewers, requiredReviewer); !ok {
			return trace.BadParameter("failed to assign all required reviewers")
		}
	}
	return nil
}

func containsRequiredReviewer(reviewers []*github.User, rev string) bool {
	for _, ghRev := range reviewers {
		if *ghRev.Login == rev {
			return true
		}
	}
	return false
}
