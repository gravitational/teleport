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

	"github.com/gravitational/trace"
)

// Check checks if required reviewers have approved the PR.
//
// Team specific reviews require an approval from both sets of reviews.
// External reviews require approval from admins.
func (b *Bot) Check(ctx context.Context) error {
	reviews, err := b.c.GitHub.ListReviews(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return trace.Wrap(err)
	}

	if b.c.Review.IsInternal(b.c.Environment.Author) {
		// Remove stale "Check" status badges inline for internal reviews.
		err := b.dismiss(ctx,
			b.c.Environment.Organization,
			b.c.Environment.Repository,
			b.c.Environment.UnsafeBranch)
		if err != nil {
			return trace.Wrap(err)
		}

		docs, code, err := b.parseChanges(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := b.c.Review.CheckInternal(b.c.Environment.Author, reviews, docs, code); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	if err := b.c.Review.CheckExternal(b.c.Environment.Author, reviews); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
