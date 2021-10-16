package bot

import (
	"context"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/trace"
)

// DimissStaleWorkflowRuns dismisses stale workflow runs for external contributors.
// Dismissing stale workflows for external contributors is done on a cron job and checks the whole repo for
// stale runs on PRs.
func (c *Bot) DimissStaleWorkflowRuns(ctx context.Context) error {
	clt := c.GithubClient.Client
	// Get the repository name and owner, on the Github Actions runner the
	// GITHUB_REPOSITORY environment variable is in the format of
	// repo-owner/repo-name.
	repoOwner, repoName, err := getRepositoryMetadata()
	if err != nil {
		return trace.Wrap(err)
	}
	pullReqs, _, err := clt.PullRequests.List(ctx, repoOwner, repoName, &github.PullRequestListOptions{State: ci.Open})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, pull := range pullReqs {
		err := validatePullRequestFields(pull)
		if err != nil {
			return trace.Wrap(err)
		}
		err = c.dismissStaleWorkflowRuns(ctx, *pull.Base.User.Login, *pull.Base.Repo.Name, *pull.Head.Ref)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
