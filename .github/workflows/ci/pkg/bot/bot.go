package bot

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"
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

// DimissStaleWorkflowRunsForExternalContributors dismisses stale workflow runs for external contributors.
// Dismissing stale workflows for external contributors is done on a cron job and checks the whole repo for
// stale runs on PRs.
func (c *Bot) DimissStaleWorkflowRunsForExternalContributors(ctx context.Context) error {
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
		err = c.DismissStaleWorkflowRuns(ctx, *pull.Base.User.Login, *pull.Base.Repo.Name, *pull.Head.Ref)
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
	// Get the target workflow to know get runs triggered by the `Check` workflow.
	// The `Check` workflow is being targeted because it is the only workflow
	// that runs multiple times per PR.
	workflow, err := c.getCheckWorkflow(ctx, owner, repoName)
	if err != nil {
		return trace.Wrap(err)
	}
	runs, err := c.findStaleWorkflowRuns(ctx, owner, repoName, branch, *workflow.ID)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.deleteRuns(ctx, owner, repoName, runs)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// deleteRuns deletes all workflow runs except the most recent one because that is
// the run in the current context.
func (c *Bot) deleteRuns(ctx context.Context, owner, repoName string, runs []*github.WorkflowRun) error {
	// Sorting runs by time from oldest to newest.
	sort.Slice(runs, func(i, j int) bool {
		time1, time2 := runs[i].CreatedAt, runs[j].CreatedAt
		return time1.Time.Before(time2.Time)
	})
	// Deleting all runs except the most recent one.
	for i := 0; i < len(runs)-1; i++ {
		run := runs[i]
		err := c.deleteRun(ctx, owner, repoName, *run.ID)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func getRepositoryMetadata() (repositoryOwner string, repositoryName string, err error) {
	repository := os.Getenv(ci.GithubRepository)
	if repository == "" {
		return "", "", trace.BadParameter("environment variable GITHUB_REPOSITORY is not set")
	}
	metadata := strings.Split(repository, "/")
	if len(metadata) != 2 {
		return "", "", trace.BadParameter("environment variable GITHUB_REPOSITORY is not in the correct format,\n the valid format is '<repo owner>/<repo name>'")
	}
	return metadata[0], metadata[1], nil
}

func (c *Bot) findStaleWorkflowRuns(ctx context.Context, owner, repoName, branchName string, workflowID int64) ([]*github.WorkflowRun, error) {
	clt := c.GithubClient.Client
	list, _, err := clt.Actions.ListWorkflowRunsByID(ctx, owner, repoName, workflowID, &github.ListWorkflowRunsOptions{Branch: branchName})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return list.WorkflowRuns, nil
}

// getCheckWorkflow gets the workflow named 'Check'.
func (c *Bot) getCheckWorkflow(ctx context.Context, owner, repoName string) (*github.Workflow, error) {
	clt := c.GithubClient.Client
	workflows, _, err := clt.Actions.ListWorkflows(ctx, owner, repoName, &github.ListOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, w := range workflows.Workflows {
		if *w.Name == ci.CheckWorkflow {
			return w, nil
		}
	}
	return nil, trace.NotFound("workflow %s not found", ci.CheckWorkflow)
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

// validatePullRequestFields checks that pull request fields needed for
// dismissing workflow runs are not nil.
func validatePullRequestFields(pr *github.PullRequest) error {
	switch {
	case pr.Base == nil:
		return trace.BadParameter("missing base branch")
	case pr.Base.User == nil:
		return trace.BadParameter("missing base branch user")
	case pr.Base.User.Login == nil:
		return trace.BadParameter("missing repository owner")
	case pr.Base.Repo == nil:
		return trace.BadParameter("missing base repository")
	case pr.Base.Repo.Name == nil:
		return trace.BadParameter("missing repository name")
	case pr.Head == nil:
		return trace.BadParameter("missing head branch")
	case pr.Head.Ref == nil:
		return trace.BadParameter("missing branch name")
	}
	if err := validateField(*pr.Base.User.Login); err != nil {
		return trace.Errorf("user login err: %v", err)
	}
	if err := validateField(*pr.Base.Repo.Name); err != nil {
		return trace.Errorf("repository name err: %v", err)
	}
	if err := validateField(*pr.Head.Ref); err != nil {
		return trace.Errorf("branch name err: %v", err)
	}
	return nil
}

// reg is used for validating various fields on Github types.
// Only allow strings that contain alphanumeric characters,
// underscores, and dashes for fields.
var reg = regexp.MustCompile(`^[\da-zA-Z-_]+$`)

func validateField(field string) error {
	found := reg.MatchString(field)
	if !found {
		return trace.BadParameter("invalid field, %s contains illegal characters or is empty", field)
	}
	return nil
}
