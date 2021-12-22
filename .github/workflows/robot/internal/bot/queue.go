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

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/gravitational/trace"
)

// Queue will update all eligible PRs with the base branch.
func (b *Bot) Queue(ctx context.Context) error {
	pulls, err := b.c.GitHub.ListPullRequests(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		"open")
	if err != nil {
		return trace.Wrap(err)
	}

	// Get the latest master, will be used to check if branch is up to date.
	master, err := b.c.GitHub.GetBranch(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		"master")
	if err != nil {
		return trace.Wrap(err)
	}

	// Filter out any PRs that are not eligible to be updated.
	numbers, err := filter(pulls, master)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, number := range numbers {
		err = b.c.GitHub.UpdateBranch(ctx,
			b.c.Environment.Organization,
			b.c.Environment.Repository,
			number)
		if err != nil {
			log.Printf("Failed to update PR %v: %v.", number, err)
			continue
		}
	}

	return nil
}

func filter(pulls []github.PullRequest, master github.Branch) ([]int, error) {
	var filtered []int

	for _, pull := range pulls {
		// Skip over PRs from forks, branches other than master, branches that are
		// up to date with master, and auto merge is not turned on.
		if pull.Fork {
			continue
		}
		if pull.UnsafeBaseRef != "master" {
			continue
		}
		if pull.BaseSHA == master.SHA {
			continue
		}
		if !pull.AutoMerge {
			continue
		}

		filtered = append(filtered, pull.Number)
	}

	return filtered, nil
}
