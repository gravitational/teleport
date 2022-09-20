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
	"log"
	"sort"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"

	"github.com/gravitational/trace"
)

// Dimiss dismisses all stale workflow runs within a repository. This is done
// to dismiss stale workflow runs for external contributors whose workflows
// run without permissions to dismiss stale workflows inline.
//
// This is needed because GitHub appends each "Check" workflow run to the status
// of a PR instead of replacing the "Check" status of the previous run.
func (b *Bot) Dismiss(ctx context.Context) error {
	pulls, err := b.c.GitHub.ListPullRequests(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		"open")
	if err != nil {
		return trace.Wrap(err)
	}

	for _, pull := range pulls {
		// Only dismiss stale runs from forks (external) as the workflow that triggers
		// this method is intended for. Dismissing runs for internal contributors
		// (non-fork) here could result in a race condition as runs are deleted upon
		// trigger separately during the `Check` workflow.
		if pull.Fork {
			// HEAD could be controlled by an attacker, however, all this would allow is
			// the attacker to dismiss a workflow run.
			if err := b.dismiss(ctx, b.c.Environment.Organization, b.c.Environment.Repository, pull.UnsafeHead); err != nil {
				log.Printf("Failed to dismiss workflow: %v %v %v: %v.", b.c.Environment.Organization, b.c.Environment.Repository, pull.UnsafeHead, err)
				continue
			}
		}
	}

	return nil
}

// dismiss dismisses all but the most recent "Check" workflow run.
//
// This is needed because GitHub appends each "Check" workflow run to the status
// of a PR instead of replacing the status of an existing run.
func (b *Bot) dismiss(ctx context.Context, organization string, repository string, branch string) error {
	check, err := b.findWorkflow(ctx,
		organization,
		repository,
		".github/workflows/check.yaml")
	if err != nil {
		return trace.Wrap(err)
	}

	runs, err := b.c.GitHub.ListWorkflowRuns(ctx,
		organization,
		repository,
		branch,
		check.ID)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.deleteRuns(ctx,
		organization,
		repository,
		runs)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *Bot) findWorkflow(ctx context.Context, organization string, repository string, path string) (github.Workflow, error) {
	workflows, err := b.c.GitHub.ListWorkflows(ctx, organization, repository)
	if err != nil {
		return github.Workflow{}, trace.Wrap(err)
	}

	var matching []github.Workflow
	for _, workflow := range workflows {
		if workflow.Path == path {
			matching = append(matching, workflow)
		}
	}

	if len(matching) == 0 {
		return github.Workflow{}, trace.NotFound("workflow %v not found", path)
	}
	if len(matching) > 1 {
		return github.Workflow{}, trace.BadParameter("found %v matching workflows", len(matching))
	}
	return matching[0], nil
}

// deleteRuns deletes all workflow runs except the most recent one because that is
// the run in the current context.
func (b *Bot) deleteRuns(ctx context.Context, organization string, repository string, runs []github.Run) error {
	// Sort runs oldest to newest then pop off last item (newest).
	sort.Slice(runs, func(i, j int) bool {
		time1, time2 := runs[i].CreatedAt, runs[j].CreatedAt
		return time1.Before(time2)
	})
	if len(runs) > 0 {
		runs = runs[:len(runs)-1]
	}

	// Deleting all runs except the most recent one.
	for _, run := range runs {
		err := b.c.GitHub.DeleteWorkflowRun(ctx,
			organization,
			repository,
			run.ID)
		if err != nil {
			log.Printf("Dismiss: Failed to dismiss workflow run %v: %v.", run.ID, err)
			continue
		}

		log.Printf("Dismiss: Successfully deleted workflow run: %v.", run.ID)
	}
	return nil
}
