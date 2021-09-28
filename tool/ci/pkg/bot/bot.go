package bot

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"sort"

	"github.com/gravitational/teleport/tool/ci"
	"github.com/gravitational/teleport/tool/ci/pkg/environment"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
)

// Config is used to configure Bot
type Config struct {
	Environment  *environment.PullRequestEnvironment
	GithubClient *github.Client
}

// Bot assigns reviewers and checks assigned reviewers for a pull request
type Bot struct {
	Environment  *environment.PullRequestEnvironment
	GithubClient GithubClient
}

// GithubClient is a wrapper around the Github client
// to be used on methods that require the client, but don't
// don't need the full functionality of Bot with
// Environment.
type GithubClient struct {
	Client *github.Client
}

// New returns a new instance of  Bot
func New(c Config) (*Bot, error) {
	var bot Bot
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if c.Environment != nil {
		bot.Environment = c.Environment
	}
	bot.GithubClient = GithubClient{
		Client: c.GithubClient,
	}
	return &bot, nil
}

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.GithubClient == nil {
		return trace.BadParameter("missing parameter GithubClient")
	}
	return nil
}

// DimissStaleWorkflowRunsForExternalContributors dismisses stale workflow runs for external contributors
func (c *Bot) DimissStaleWorkflowRunsForExternalContributors(ctx context.Context, repoOwner, repoName string) error {
	clt := c.GithubClient.Client
	pulls, _, err := clt.PullRequests.List(ctx, repoOwner, repoName, &github.PullRequestListOptions{State: ci.Open})
	if err != nil {
		return err
	}
	for _, pull := range pulls {
		err := c.DismissStaleWorkflowRuns(ctx, *pull.Base.User.Login, *pull.Base.Repo.Name, *pull.Head.Ref)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DismissStaleWorkflowRuns dismisses stale Check workflow runs.
// Stale workflow runs are workflow runs that were previously ran and are no longer valid
// due to a new event triggering thus a change in state. The workflow running in the current context is the source of truth for
// the state of checks.
func (c *Bot) DismissStaleWorkflowRuns(ctx context.Context, owner, repoName, branch string) error {
	var targetWorkflow *github.Workflow
	var workflowRuns []*github.WorkflowRun
	clt := c.GithubClient.Client

	workflows, _, err := clt.Actions.ListWorkflows(ctx, owner, repoName, &github.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, w := range workflows.Workflows {
		if *w.Name == ci.CheckWorkflow {
			targetWorkflow = w
			break
		}
	}
	list, _, err := clt.Actions.ListWorkflowRunsByID(ctx, owner, repoName, *targetWorkflow.ID, &github.ListWorkflowRunsOptions{Branch: branch})
	if err != nil {
		return trace.Wrap(err)
	}
	workflowRuns = list.WorkflowRuns
	sort.Slice(workflowRuns, func(i, j int) bool {
		time1, time2 := workflowRuns[i].CreatedAt, workflowRuns[j].CreatedAt
		return time1.Time.Before(time2.Time)
	})
	// Deleting all runs except the most recent one.
	for i := 0; i < len(workflowRuns)-1; i++ {
		run := list.WorkflowRuns[i]
		err := c.deleteRun(ctx, owner, repoName, *run.ID)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

const (
	// GithubAPIHostname is the Github API hostname.
	GithubAPIHostname = "api.github.com"
	// Scheme is the protocol scheme used when making
	// a request to delete a workflow run to the Github API.
	Scheme = "https"
)

// deleteRun deletes a workflow run.
// Note: the go-github client library does not support this endpoint.
func (c *Bot) deleteRun(ctx context.Context, owner, repo string, runID int64) error {
	clt := c.GithubClient.Client
	// Construct url
	s := fmt.Sprintf("repos/%s/%s/actions/runs/%v", owner, repo, runID)
	cleanPath := path.Join("/", s)
	url := url.URL{
		Scheme: Scheme,
		Host:   GithubAPIHostname,
		Path:   cleanPath,
	}
	req, err := clt.NewRequest("DELETE", url.String(), nil)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = clt.Do(ctx, req, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
