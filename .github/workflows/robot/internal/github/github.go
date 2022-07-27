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

package github

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	go_github "github.com/google/go-github/v37/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client *go_github.Client
}

// New returns a new GitHub Client.
func New(ctx context.Context, token string) (*Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	return &Client{
		client: go_github.NewClient(oauth2.NewClient(ctx, ts)),
	}, nil
}

// RequestReviewers is used to assign reviewers to a Pull Requests.
func (c *Client) RequestReviewers(ctx context.Context, organization string, repository string, number int, reviewers []string) error {
	_, _, err := c.client.PullRequests.RequestReviewers(ctx,
		organization,
		repository,
		number,
		go_github.ReviewersRequest{
			Reviewers: reviewers,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DismissReviewers is used to remove the review request from a Pull Request.
func (c *Client) DismissReviewers(ctx context.Context, organization string, repository string, number int, reviewers []string) error {
	_, err := c.client.PullRequests.RemoveReviewers(ctx,
		organization,
		repository,
		number,
		go_github.ReviewersRequest{
			Reviewers: reviewers,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Review is a GitHub PR review.
type Review struct {
	// Author is the GitHub login of the user that created the PR.
	Author string
	// State is the state of the PR, for example APPROVED, COMMENTED,
	// CHANGES_REQUESTED, or DISMISSED.
	State string
	// SubmittedAt is the time the PR was created.
	SubmittedAt time.Time
}

func (c *Client) ListReviews(ctx context.Context, organization string, repository string, number int) ([]Review, error) {
	var reviews []Review

	opts := &go_github.ListOptions{
		Page:    0,
		PerPage: perPage,
	}
	for {
		page, resp, err := c.client.PullRequests.ListReviews(ctx,
			organization,
			repository,
			number,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range page {
			reviews = append(reviews, Review{
				Author:      r.GetUser().GetLogin(),
				State:       r.GetState(),
				SubmittedAt: r.GetSubmittedAt(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Sort oldest review first.
	sort.SliceStable(reviews, func(i, j int) bool {
		return reviews[i].SubmittedAt.Before(reviews[j].SubmittedAt)
	})

	return reviews, nil
}

// PullRequest is a Pull Requested submitted to the repository.
type PullRequest struct {
	// Author is the GitHub login of the user that created the PR.
	Author string
	// Repository is the name of the repository.
	Repository string
	// Number is the Pull Request number.
	Number int
	// State is the state of the submitted review.
	State string
	// UnsafeBase is the base of the branch.
	//
	// UnsafeBase can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeBase Branch
	// UnsafeHead is the name head of the branch.
	//
	// UnsafeHead can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeHead Branch
	// UnsafeTitle is the title of the Pull Request.
	//
	// UnsafeTitle can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeTitle string
	// UnsafeBody is the body of the Pull Request.
	//
	// UnsafeBody can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeBody string
	// UnsafeLabels are the labels attached to the Pull Request.
	//
	// UnsafeLabels can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeLabels []string
	// Fork determines if the pull request is from a fork.
	Fork bool
}

// Branch is a git Branch.
type Branch struct {
	// Ref is a human readable name branch name.
	Ref string
	// SHA is the SHA1 hash of the commit.
	SHA string
}

// ListReviewers returns a list of reviewers that have yet to submit a review.
func (c *Client) ListReviewers(ctx context.Context, organization string, repository string, number int) ([]string, error) {
	var reviewers []string

	opts := &go_github.ListOptions{
		Page:    0,
		PerPage: perPage,
	}
	for {
		page, resp, err := c.client.PullRequests.ListReviewers(ctx,
			organization,
			repository,
			number,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range page.Users {
			reviewers = append(reviewers, r.GetLogin())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return reviewers, nil
}

// PullRequestFile is a file that was modified in a pull request.
type PullRequestFile struct {
	// Name is the name of the file.
	Name string
	// Additions is the number of new lines added to the file
	Additions int
	// Deletions is the number of lines removed from the file
	Deletions int
}

// GetPullRequest returns a specific Pull Request.
func (c *Client) GetPullRequest(ctx context.Context, organization string, repository string, number int) (PullRequest, error) {
	pull, _, err := c.client.PullRequests.Get(ctx,
		organization,
		repository,
		number)
	if err != nil {
		return PullRequest{}, trace.Wrap(err)
	}

	var labels []string
	for _, label := range pull.Labels {
		labels = append(labels, label.GetName())
	}

	return PullRequest{
		Author:     pull.GetUser().GetLogin(),
		Repository: repository,
		Number:     pull.GetNumber(),
		State:      pull.GetState(),
		UnsafeBase: Branch{
			Ref: pull.GetBase().GetRef(),
			SHA: pull.GetBase().GetSHA(),
		},
		UnsafeHead: Branch{
			Ref: pull.GetHead().GetRef(),
			SHA: pull.GetHead().GetSHA(),
		},
		UnsafeTitle:  pull.GetTitle(),
		UnsafeBody:   pull.GetBody(),
		UnsafeLabels: labels,
		Fork:         pull.GetHead().GetRepo().GetFork(),
	}, nil
}

// ListPullRequests returns a list of Pull Requests.
func (c *Client) ListPullRequests(ctx context.Context, organization string, repository string, state string) ([]PullRequest, error) {
	var pulls []PullRequest

	opts := &go_github.PullRequestListOptions{
		State: state,
		ListOptions: go_github.ListOptions{
			Page:    0,
			PerPage: perPage,
		},
	}
	for {
		page, resp, err := c.client.PullRequests.List(ctx,
			organization,
			repository,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, pull := range page {
			var labels []string
			for _, label := range pull.Labels {
				labels = append(labels, label.GetName())
			}

			pulls = append(pulls, PullRequest{
				Author:     pull.GetUser().GetLogin(),
				Repository: repository,
				Number:     pull.GetNumber(),
				State:      pull.GetState(),
				UnsafeBase: Branch{
					Ref: pull.GetBase().GetRef(),
					SHA: pull.GetBase().GetSHA(),
				},
				UnsafeHead: Branch{
					Ref: pull.GetHead().GetRef(),
					SHA: pull.GetHead().GetSHA(),
				},
				UnsafeTitle:  pull.GetTitle(),
				UnsafeBody:   pull.GetBody(),
				UnsafeLabels: labels,
				Fork:         pull.GetHead().GetRepo().GetFork(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return pulls, nil
}

// ListFiles is used to list all the files within a Pull Request.
func (c *Client) ListFiles(ctx context.Context, organization string, repository string, number int) ([]PullRequestFile, error) {
	var files []PullRequestFile

	opts := &go_github.ListOptions{
		Page:    0,
		PerPage: perPage,
	}
	for {
		page, resp, err := c.client.PullRequests.ListFiles(ctx,
			organization,
			repository,
			number,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, file := range page {
			files = append(files, PullRequestFile{
				Name:      file.GetFilename(),
				Additions: file.GetAdditions(),
				Deletions: file.GetDeletions(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return files, nil
}

// AddLabels will add labels to an Issue or Pull Request.
func (c *Client) AddLabels(ctx context.Context, organization string, repository string, number int, labels []string) error {
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx,
		organization,
		repository,
		number,
		labels)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Workflow contains information about a workflow.
type Workflow struct {
	// ID of the workflow.
	ID int64
	// Name of the workflow.
	Name string
	// Path of the workflow.
	Path string
}

// ListWorkflows lists all workflows within a repository.
func (c *Client) ListWorkflows(ctx context.Context, organization string, repository string) ([]Workflow, error) {
	var workflows []Workflow

	opts := &go_github.ListOptions{
		Page:    0,
		PerPage: perPage,
	}
	for {
		page, resp, err := c.client.Actions.ListWorkflows(ctx,
			organization,
			repository,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if page.Workflows == nil {
			log.Printf("Got empty page of workflows for %v.", repository)
			continue
		}

		for _, workflow := range page.Workflows {
			workflows = append(workflows, Workflow{
				Name: workflow.GetName(),
				Path: workflow.GetPath(),
				ID:   workflow.GetID(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return workflows, nil
}

// Run is a specific workflow run.
type Run struct {
	// ID of the workflow run.
	ID int64
	// CreatedAt time the workflow run was created.
	CreatedAt time.Time
}

// ListWorkflowRuns is used to list all workflow runs for an ID.
func (c *Client) ListWorkflowRuns(ctx context.Context, organization string, repository string, branch string, workflowID int64) ([]Run, error) {
	var runs []Run

	opts := &go_github.ListWorkflowRunsOptions{
		Branch: branch,
		ListOptions: go_github.ListOptions{
			Page:    0,
			PerPage: perPage,
		},
	}
	for {
		page, resp, err := c.client.Actions.ListWorkflowRunsByID(ctx,
			organization,
			repository,
			workflowID,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if page.WorkflowRuns == nil {
			log.Printf("Got empty page of workflow runs for branch: %v, workflowID: %v.", branch, workflowID)
			continue
		}

		for _, run := range page.WorkflowRuns {
			runs = append(runs, Run{
				ID:        run.GetID(),
				CreatedAt: run.GetCreatedAt().Time,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return runs, nil
}

// DeleteWorkflowRun is directly implemented because it is missing from go-github.
//
// https://docs.github.com/en/rest/reference/actions#delete-a-workflow-run
func (c *Client) DeleteWorkflowRun(ctx context.Context, organization string, repository string, runID int64) error {
	url := url.URL{
		Scheme: "https",
		Host:   "api.github.com",
		Path:   path.Join("repos", organization, repository, "actions", "runs", strconv.FormatInt(runID, 10)),
	}
	req, err := c.client.NewRequest(http.MethodDelete, url.String(), nil)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.client.Do(ctx, req, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateComment will leave a comment on an Issue or Pull Request.
func (c *Client) CreateComment(ctx context.Context, organization string, repository string, number int, comment string) error {
	_, _, err := c.client.Issues.CreateComment(ctx,
		organization,
		repository,
		number,
		&go_github.IssueComment{
			Body: &comment,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreatePullRequest will create a Pull Request.
func (c *Client) CreatePullRequest(ctx context.Context, organization string, repository string, title string, head string, base string, body string, draft bool) (int, error) {
	pull, _, err := c.client.PullRequests.Create(ctx,
		organization,
		repository,
		&go_github.NewPullRequest{
			Title: &title,
			Head:  &head,
			Base:  &base,
			Body:  &body,
			Draft: &draft,
		})
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return pull.GetNumber(), nil
}

// ListWorkflowJobs lists all jobs for a workflow run.
func (c *Client) ListWorkflowJobs(ctx context.Context, organization string, repository string, runID int64) ([]Job, error) {
	var jobs []Job

	opts := &go_github.ListWorkflowJobsOptions{
		ListOptions: go_github.ListOptions{
			Page:    0,
			PerPage: perPage,
		},
	}
	for {
		page, resp, err := c.client.Actions.ListWorkflowJobs(ctx,
			organization,
			repository,
			runID,
			opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, job := range page.Jobs {
			jobs = append(jobs, Job{
				Name: job.GetName(),
				ID:   job.GetID(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return jobs, nil
}

// Job is a job within a workflow run.
type Job struct {
	// Name of the workflow job.
	Name string

	// ID of the job.
	ID int64
}

const (
	// perPage is the number of items per page to request.
	perPage = 100
)
