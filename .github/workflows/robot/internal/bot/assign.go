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
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// Assign will assign reviewers for this PR.
//
// Assign works by parsing the PR, discovering the changes, and returning a
// set of reviewers determined by: content of the PR, if the author is internal
// or external, and team they are on.
func (b *Bot) Assign(ctx context.Context) error {
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
	// If a backport PR was found, assign original reviewers. Otherwise fall
	// through to normal assignment logic.
	if isBackport(b.c.Environment.UnsafeBase) {
		reviewers, err := b.backportReviewers(ctx)
		if err == nil {
			return reviewers, nil
		}
		log.Printf("Assign: Found backport PR, but failed to find original reviewers: %v. Falling through to normal assignment logic.", err)
	}

	docs, code, err := b.parseChanges(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b.c.Review.Get(b.c.Environment.Author, docs, code), nil
}

func (b *Bot) backportReviewers(ctx context.Context) ([]string, error) {
	// Search inside the PR to find a reference to the original PR.
	original, err := b.findOriginal(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var originalReviewers []string

	// Append list of reviewers that have yet to submit a review.
	reviewers, err := b.c.GitHub.ListReviewers(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		original)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	originalReviewers = append(originalReviewers, reviewers...)

	// Append list of reviews that have submitted a review.
	reviews, err := b.c.GitHub.ListReviews(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		original)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, review := range reviews {
		originalReviewers = append(originalReviewers, review.Author)
	}

	return originalReviewers, nil
}

func (b *Bot) findOriginal(ctx context.Context, organization string, repository string, number int) (int, error) {
	pull, err := b.c.GitHub.GetPullRequest(ctx,
		organization,
		repository,
		number)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	var original string

	// Search inside the title and body for the original PR number.
	title := pattern.FindStringSubmatch(pull.UnsafeTitle)
	body := pattern.FindStringSubmatch(pull.UnsafeBody)
	switch {
	case len(title) == 0 && len(body) == 0:
		return 0, trace.NotFound("no PR referenced in title or body")
	case len(title) == 0 && len(body) == 2:
		original = body[1]
	case len(title) == 2 && len(body) == 0:
		original = title[1]
	case len(title) == 2 && len(body) == 2:
		if title[1] != body[1] {
			return 0, trace.NotFound("different PRs referenced in title and body")
		}
		original = title[1]
	default:
		return 0, trace.NotFound("failed to find reference to PR")
	}

	n, err := strconv.Atoi(original)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Verify the number found is a Pull Request and not an Issue.
	_, err = b.c.GitHub.GetPullRequest(ctx,
		organization,
		repository,
		n)
	if err != nil {
		return 0, trace.NotFound("found Issue %v, not Pull Request", original)
	}

	log.Printf("Assign: Found original PR #%v.", original)
	return n, nil
}

func isBackport(unsafeBase string) bool {
	return strings.HasPrefix(unsafeBase, "branch/v")
}

var pattern = regexp.MustCompile(`#([0-9]+)`)
