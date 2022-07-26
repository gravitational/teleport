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
	"log"
	"strings"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/review"
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

	if !b.c.Review.IsInternal(b.c.Environment.Author) {
		if err := b.c.Review.CheckExternal(b.c.Environment.Author, reviews); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// Remove stale "Check" status badges inline for internal reviews.
	err = b.dismiss(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.UnsafeHead)
	if err != nil {
		return trace.Wrap(err)
	}

	files, err := b.c.GitHub.ListFiles(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return trace.Wrap(err)
	}

	docs, code, err := classifyChanges(files)
	if err != nil {
		return trace.Wrap(err)
	}

	large := isLargePR(files)
	if large {
		comment := fmt.Sprintf("@%v - this PR is large and will require admin approval to merge. "+
			"Consider breaking it up into a series smaller changes.", b.c.Environment.Author)
		b.c.GitHub.CreateComment(ctx,
			b.c.Environment.Organization,
			b.c.Environment.Repository,
			b.c.Environment.Number,
			comment,
		)
	}

	if err := b.c.Review.CheckInternal(b.c.Environment.Author, reviews, docs, code, large); err != nil {
		return trace.Wrap(err)
	}

	// if we have passed our checks we can try to dismiss other requested reviews
	if err := b.dismissReviewers(ctx); err != nil {
		log.Printf("Check: Failed to dismiss reviews: %v", err)
	}

	return nil
}

// dismissReviewers removes stale review requests from an approved pull request.
func (b *Bot) dismissReviewers(ctx context.Context) error {
	r, err := b.reviewersToDismiss(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(r) == 0 {
		return nil
	}

	log.Printf("Check: Dismissing reviews for: %v", strings.Join(r, ", "))
	return trace.Wrap(b.c.GitHub.DismissReviewers(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number,
		r,
	))
}

// reviewersToDismiss determines which (if any) reviewers can be removed
// from an *already approved* pull request.
// Precondition: the pull request must already pass required approvers checks.
func (b *Bot) reviewersToDismiss(ctx context.Context) ([]string, error) {
	reviewers, err := b.c.GitHub.ListReviewers(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reviews, err := b.c.GitHub.ListReviews(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	internalApprovals := 0
	reviewedBy := make(map[string]struct{})

	// only count each reviewer's latest review (so we start from the end)
	for i := len(reviews) - 1; i >= 0; i-- {
		r := reviews[i]

		// if we've already seen this reviewer then we're looking at an older review - skip it
		if _, ok := reviewedBy[r.Author]; ok {
			continue
		}
		reviewedBy[r.Author] = struct{}{}
		if r.State == review.Approved && b.c.Review.IsInternal(r.Author) {
			internalApprovals++
		}
	}

	// Our internal checks could have passed with an admin approval, even though
	// we only have a single approval. Ensure we have at least two internal approvals
	// before we decide to dismiss reviewers.
	if internalApprovals < 2 {
		return nil, nil
	}

	var reviewersToDismiss []string
	for _, reviewer := range reviewers {
		if _, ok := reviewedBy[reviewer]; !ok {
			reviewersToDismiss = append(reviewersToDismiss, reviewer)
		}
	}

	return reviewersToDismiss, nil
}
