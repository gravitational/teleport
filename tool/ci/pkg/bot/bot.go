package bot

import (
	"context"
	"fmt"
	"io/ioutil"

	"sort"

	"github.com/gravitational/teleport/tool/ci"
	"github.com/gravitational/teleport/tool/ci/pkg/environment"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
	log "github.com/sirupsen/logrus"
)

// Config is used to configure Bot
type Config struct {
	Environment *environment.Environment
}

// Bot assigns reviewers and checks assigned reviewers for a pull request
type Bot struct {
	Environment  *environment.Environment
	GithubClient GithubClient
}
type GithubClient struct {
	Client *github.Client
}

// New returns a new instance of  Bot
func New(c Config) (*Bot, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Bot{
		Environment: c.Environment,
		GithubClient: GithubClient{
			Client: c.Environment.Client,
		},
	}, nil
}

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Environment == nil {
		return trace.BadParameter("missing parameter Environment")
	}
	return nil
}

// invalidateApprovals dismisses all reviews on a pull request
func (c *Bot) invalidateApprovals(ctx context.Context, msg string, reviews []review) error {
	pr := c.Environment.PullRequest
	for _, v := range reviews {
		_, _, err := c.Environment.Client.PullRequests.DismissReview(ctx,
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			v.id,
			&github.PullRequestReviewDismissalRequest{Message: &msg},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DimissStaleWorkflowRunsForExternalContributors dismisses stale workflow runs for external contributors
func (gc GithubClient) DimissStaleWorkflowRunsForExternalContributors(ctx context.Context, token, repoOwner, repoName string) error {
	pulls, _, err := gc.Client.PullRequests.List(ctx, repoOwner, repoName, &github.PullRequestListOptions{State: ci.Open})
	if err != nil {
		return err
	}
	for _, pull := range pulls {
		err := gc.DismissStaleWorkflowRuns(ctx, token, *pull.Base.User.Login, *pull.Base.Repo.Name, *pull.Head.Ref)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DismissStaleWorkflowRuns dismisses stale Check workflow runs.
// Stale workflow runs are workflow runs that were previously ran/not the most recent and are no longer valid
// due to a new event triggering/change in state. The workflow in the current context is the source of truth for
// the state of the pull request.
func (gc GithubClient) DismissStaleWorkflowRuns(ctx context.Context, token, owner, repoName, branch string) error {
	var targetWorkflow *github.Workflow
	workflows, _, err := gc.Client.Actions.ListWorkflows(ctx, owner, repoName, &github.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, w := range workflows.Workflows {
		if *w.Name == ci.CheckWorkflow {
			targetWorkflow = w
			break
		}
	}
	list, _, err := gc.Client.Actions.ListWorkflowRunsByID(ctx, owner, repoName, *targetWorkflow.ID, &github.ListWorkflowRunsOptions{Branch: branch})
	if err != nil {
		return trace.Wrap(err)
	}
	sort.Sort(ByTime(list.WorkflowRuns))

	// Deleting all runs except the most recently started one.
	for i := 0; i < len(list.WorkflowRuns)-1; i++ {
		run := list.WorkflowRuns[i]
		err := gc.deleteRun(ctx, token, owner, repoName, *run.ID)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// deleteRun deletes a workflow run.
// Note: the go-github client library does not support this endpoint.
func (gc GithubClient) deleteRun(ctx context.Context, token, owner, repo string, runID int64) error {
	// Construct url
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%v/", owner, repo, runID)
	req, err := gc.Client.NewRequest("DELETE", url, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := gc.Client.Do(ctx, req, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Println(string(bodyBytes))
	return nil
}

type ByTime []*github.WorkflowRun

func (s ByTime) Len() int {
	return len(s)
}

func (s ByTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByTime) Less(i, j int) bool {
	time1, time2 := s[i].CreatedAt, s[j].CreatedAt
	return time1.Time.Before(time2.Time)
}
