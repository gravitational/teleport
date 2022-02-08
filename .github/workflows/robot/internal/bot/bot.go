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
	"strings"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/env"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/review"

	"github.com/gravitational/trace"
)

// Client implements the GitHub API.
type Client interface {
	// RequestReviewers is used to assign reviewers to a PR.
	RequestReviewers(ctx context.Context, organization string, repository string, number int, reviewers []string) error

	// ListReviews is used to list all submitted reviews for a PR.
	ListReviews(ctx context.Context, organization string, repository string, number int) (map[string]*github.Review, error)

	// ListPullRequests returns a list of Pull Requests.
	ListPullRequests(ctx context.Context, organization string, repository string, state string) ([]github.PullRequest, error)

	// ListFiles is used to list all the files within a PR.
	ListFiles(ctx context.Context, organization string, repository string, number int) ([]string, error)

	// AddLabels will add labels to an Issue or Pull Request.
	AddLabels(ctx context.Context, organization string, repository string, number int, labels []string) error

	// ListWorkflows lists all workflows within a repository.
	ListWorkflows(ctx context.Context, organization string, repository string) ([]github.Workflow, error)

	// ListWorkflowRuns is used to list all workflow runs for an ID.
	ListWorkflowRuns(ctx context.Context, organization string, repository string, branch string, workflowID int64) ([]github.Run, error)

	// DeleteWorkflowRun is used to delete a workflow run.
	DeleteWorkflowRun(ctx context.Context, organization string, repository string, runID int64) error
}

// Config contains configuration for the bot.
type Config struct {
	// GitHub is a GitHub client.
	GitHub Client

	// Environment holds information about the workflow run event.
	Environment *env.Environment

	// Review is used to get code and docs reviewers.
	Review *review.Assignments
}

// CheckAndSetDefaults checks and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.GitHub == nil {
		return trace.BadParameter("missing parameter GitHub")
	}
	if c.Environment == nil {
		return trace.BadParameter("missing parameter Environment")
	}
	if c.Review == nil {
		return trace.BadParameter("missing parameter Review")
	}

	return nil
}

// Bot performs repository management.
type Bot struct {
	c *Config
}

// New returns a new repository management bot.
func New(c *Config) (*Bot, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Bot{
		c: c,
	}, nil
}

func (b *Bot) parseChanges(ctx context.Context) (bool, bool, error) {
	var docs bool
	var code bool

	files, err := b.c.GitHub.ListFiles(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return false, true, trace.Wrap(err)
	}

	for _, file := range files {
		if strings.HasPrefix(file, "docs/") {
			docs = true
		} else {
			code = true
		}

	}
	return docs, code, nil
}
