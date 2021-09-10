package cron

import (
	"context"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/bot"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
)

// DimissStaleWorkflowRunsForExternalContributors dismisses stale workflow runs for external contributors
func DimissStaleWorkflowRunsForExternalContributors(ctx context.Context, token, repoOwner, repoName string, clt *github.Client) error {
	pulls, _, err := clt.PullRequests.List(ctx, repoOwner, repoName, &github.PullRequestListOptions{State: ci.Open})
	if err != nil {
		return err
	}
	for _, pull := range pulls {
		err := bot.DismissStaleWorkflowRuns(ctx, token, *pull.Base.User.Login, *pull.Base.Repo.Name, *pull.Head.Ref, clt)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
