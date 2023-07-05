// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import (
	"context"
	"log"
	"time"

	"github.com/google/go-github/v41/github"
	"github.com/gravitational/trace"
)

type RunIDSet map[int64]struct{}

// Equals performs a deep comparison of the set values
func (s RunIDSet) Equals(other RunIDSet) bool {
	if len(s) != len(other) {
		return false
	}
	for k := range s {
		if _, present := other[k]; !present {
			return false
		}
	}
	return true
}

// Insert adds an element to a set. Adding an element multiple times is not
// considered an error.
func (s RunIDSet) Insert(runID int64) {
	s[runID] = struct{}{}
}

// NotIn returns the elements that are in `s` but not in `other`
func (s RunIDSet) NotIn(other RunIDSet) RunIDSet {
	result := make(RunIDSet)
	for k := range s {
		if _, present := other[k]; !present {
			result[k] = struct{}{}
		}
	}
	return result
}

// WorkflowRuns defines the minimal API used to lst and query GitHub action
// runner workflows and jobs
type WorkflowRuns interface {
	GetWorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (*github.WorkflowRun, *github.Response, error)
	ListWorkflowRunsByFileName(ctx context.Context, owner, repo, workflowFileName string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error)
}

// Returns information about all matched runs started after `since`.
func ListWorkflowRuns(ctx context.Context, actions WorkflowRuns, owner, repo, path, branch string, since time.Time) ([]*github.WorkflowRun, error) {
	listOptions := github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
		Branch:  branch,
		Created: ">" + since.Format(time.RFC3339),
	}

	allRuns := make([]*github.WorkflowRun, 0)

	for {
		runs, resp, err := actions.ListWorkflowRunsByFileName(ctx, owner, repo, path, &listOptions)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to fetch runs")
		}

		allRuns = append(allRuns, runs.WorkflowRuns...)

		if resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	return allRuns, nil
}

// ListWorkflowRunIDs returns a set of RunIDs, representing the set of all for
// workflow runs created since the supplied start time.
func ListWorkflowRunIDs(ctx context.Context, actions WorkflowRuns, owner, repo, path, branch string, since time.Time) (RunIDSet, error) {
	workflowRuns, err := ListWorkflowRuns(ctx, actions, owner, repo, path, branch, since)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a list of workflow runs")
	}

	runIDs := make(RunIDSet, len(workflowRuns))
	for _, workflowRun := range workflowRuns {
		runIDs.Insert(workflowRun.GetID())
	}

	return runIDs, nil
}

// WorkflowJobLister defines the minimal GitHub client interafce required to
// list query and compose workflow jobs.
type WorkflowJobLister interface {
	ListWorkflowJobs(ctx context.Context, owner, repo string, runID int64, opts *github.ListWorkflowJobsOptions) (*github.Jobs, *github.Response, error)
}

// ListWorkflowJobs lists the jobs for a given workflow run in the specified
// repository.
func ListWorkflowJobs(ctx context.Context, lister WorkflowJobLister, owner, repo string, runID int64) ([]*github.WorkflowJob, error) {
	listOptions := github.ListWorkflowJobsOptions{}
	result := []*github.WorkflowJob{}
	for {
		jobs, resp, err := lister.ListWorkflowJobs(ctx, owner, repo, runID, &listOptions)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to fetch workflow jobs")
		}

		result = append(result, jobs.Jobs...)

		if resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	return result, nil
}

// WaitForRun blocks until the specified workflow run completes, and returns the overall
// workflow status.
func WaitForRun(ctx context.Context, actions WorkflowRuns, owner, repo, path string, runID int64) (string, error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			run, _, err := actions.GetWorkflowRunByID(ctx, owner, repo, runID)
			if err != nil {
				return "", trace.Wrap(err, "Failed polling run")
			}

			log.Printf("Workflow status: %s", run.GetStatus())

			if run.GetStatus() == "completed" {
				return run.GetConclusion(), nil
			}

		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// InstallationLister defines the minimal interface for listing GitHub App Installations
// via the GitHub API.
type InstallationLister interface {
	ListInstallations(ctx context.Context, opts *github.ListOptions) ([]*github.Installation, *github.Response, error)
}

// FindAppInstallID finds the ID of an app installation on a given GitHub account.
// The App ID is inferred by the credentials used by the `lister` to authenticate
// with the GitHub API
func FindAppInstallID(ctx context.Context, lister InstallationLister, owner string) (int64, error) {
	listOptions := github.ListOptions{PerPage: 100}
	for {
		installations, response, err := lister.ListInstallations(ctx, &listOptions)
		if err != nil {
			return 0, trace.Wrap(err, "Failed to list installations")
		}

		for _, inst := range installations {
			if inst.GetAccount().GetLogin() == owner {
				return inst.GetID(), nil
			}
		}

		if response.NextPage == 0 {
			return 0, trace.NotFound("No such installation found")
		}

		listOptions.Page = response.NextPage
	}
}
