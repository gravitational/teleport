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

	"github.com/gravitational/trace"
)

// Assign will assign reviewers for this PR.
//
// Assign works by parsing the PR, discovering the changes, and returning a
// set of reviewers determined by: content of the PR, if the author is internal
// or external, and team they are on.
func (b *Bot) Assign(ctx context.Context) error {
	// Get list of reviewers for this PR.
	reviewers, err := b.getReviewers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Printf("Assign: Requesting reviews from: %v.", reviewers)

	// Request GitHub assign reviewers to this PR.
	err = b.c.GitHub.RequestReviewers(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number,
		reviewers)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *Bot) getReviewers(ctx context.Context) ([]string, error) {
	docs, code, err := b.parseChanges(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return b.c.Review.Get(b.c.Environment.Author, docs, code), nil
}
