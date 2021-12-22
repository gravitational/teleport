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

	for _, pull := range pulls {
		if skipUpdate(pull) {
			continue
		}

		err = b.c.GitHub.UpdateBranch(ctx,
			b.c.Environment.Organization,
			b.c.Environment.Repository,
			pull.Number)
		if err != nil {
			log.Printf("Failed to update PR %v: %v.", pull.Number, err)
			continue
		}
	}

	return nil
}

// skipUpdate returns if this branch should be skipped over for updating. PRs
// are skipped over if they are from a fork, are a branch other than master,
// auto-merge is disabled, or are not mergeable.
func skipUpdate(pull github.PullRequest) bool {
	if pull.Fork {
		return true
	}
	if pull.UnsafeBase != "master" {
		return true
	}
	if !pull.AutoMerge {
		return true
	}
	if pull.Mergeable {
		return true
	}

	return false
}
