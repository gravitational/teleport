package bot

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
)

// Assign assigns reviewers to the pull request in the
// current context.
func (a *Bot) Assign(ctx context.Context) error {
	pullReq := a.Environment.Metadata
	// Getting reviewers for author of pull request
	r := a.Environment.GetReviewersForAuthor(pullReq.Author)
	client := a.Environment.Client
	// Assigning reviewers to pull request
	_, _, err := client.PullRequests.RequestReviewers(ctx,
		pullReq.RepoOwner,
		pullReq.RepoName, pullReq.Number,
		github.ReviewersRequest{Reviewers: r})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
