/*
Copyright 2021 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
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
			// We do not want to stop dismissing stale workflow runs for the remaining PRs if there 
			// is a validation error, skip this iteration. Keep stale runs on PRs that have invalid fields in the event the
			// invalid fields were malicious input. 
			log.Error(err) 
			continue
		}
		err = c.dismissStaleWorkflowRuns(ctx, *pull.Base.User.Login, *pull.Base.Repo.Name, *pull.Head.Ref)
		if err != nil {
			// Log the error, keep trying to dimiss remaining stale runs. 
			log.Error(err)
		}
	}
	return nil
}

// dismissStaleWorkflowRuns dismisses stale Check workflow runs.
// Stale workflow runs are workflow runs that were previously ran and are no longer valid
// due to a new event triggering thus a change in state. The workflow running in the current context is the source of truth for
// the state of checks.
func (c *Bot) dismissStaleWorkflowRuns(ctx context.Context, owner, repoName, branch string) error {
	// Get the target workflow to know get runs triggered by the `Check` workflow.
	// The `Check` workflow is being targeted because it is the only workflow
	// that runs multiple times per PR.
	workflow, err := c.getCheckWorkflow(ctx, owner, repoName)
	if err != nil {
		return trace.Wrap(err)
	}
	runs, err := c.getWorkflowRuns(ctx, owner, repoName, branch, *workflow.ID)
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

func (c *Bot) getWorkflowRuns(ctx context.Context, owner, repoName, branchName string, workflowID int64) ([]*github.WorkflowRun, error) {
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

// deleteRun deletes a workflow run.
// Note: the go-github client library does not support this endpoint.
func (c *Bot) deleteRun(ctx context.Context, owner, repo string, runID int64) error {
	clt := c.GithubClient.Client
	// Construct url
	url := url.URL{
		Scheme: scheme,
		Host:   githubAPIHostname,
		Path:   path.Join("repos", owner, repo, "actions", "runs", fmt.Sprint(runID)),
	}
	req, err := clt.NewRequest(http.MethodDelete, url.String(), nil)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = clt.Do(ctx, req, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	// githubAPIHostname is the Github API hostname.
	githubAPIHostname = "api.github.com"
	// scheme is the protocol scheme used when making
	// a request to delete a workflow run to the Github API.
	scheme = "https"
)
