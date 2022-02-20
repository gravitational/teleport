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

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"

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

		// Check if PR has received required approvals.
		if err := b.c.Review.CheckInternal(b.c.Environment.Author, reviews, docs, code); err != nil {
			return trace.Wrap(err)
		}

		// Check if PR has test coverage or has admin approval to bypass.
		if err := b.checkTests(ctx, b.c.Environment.Author, reviews); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	// PRs from external authors require two admin approvals to merge.
	if err := b.c.Review.CheckAdmin(b.c.Environment.Author, reviews, 2); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *Bot) checkTests(ctx context.Context, author string, reviews map[string]*github.Review) error {
	// If an admin has approved, bypass the test coverage check.
	if err := b.c.Review.CheckAdmin(author, reviews, 1); err == nil {
		return nil
	}

	if err := b.hasTestCoverage(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *Bot) hasTestCoverage(ctx context.Context) error {
	files, err := b.c.GitHub.ListFiles(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return trace.Wrap(err)
	}

	var code bool
	var tests bool

	for _, file := range files {
		// Remove after "branch/v7" and "branch/v8" go out of support.
		if strings.HasPrefix(file, "vendor/") {
			continue
		}

		switch {
		case strings.HasSuffix(file, "_test.go"):
			tests = true
		case strings.HasSuffix(file, ".go"):
			code = true
		}
	}

	// Fail if code was added without test coverage.
	if code && !tests {
		return trace.BadParameter("missing test coverage, add test coverage or request admin override")
	}
	return nil
}
