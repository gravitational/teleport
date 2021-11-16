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
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/trace"
)

// Check checks if all the reviewers have approved the pull request in the current context.
func (c *Bot) Check(ctx context.Context) error {
	pr := c.Environment.Metadata
	if c.Environment.IsInternal(pr.Author) {
		return c.checkInternal(ctx)
	}
	return c.checkExternal(ctx)
}

// checkInternal is called to check if a PR reviewed and approved by the
// required reviewers for internal contributors. Unlike approvals for
// external contributors, approvals from internal team members will not be
// invalidated when new changes are pushed to the PR.
func (c *Bot) checkInternal(ctx context.Context) error {
	pr := c.Environment.Metadata
	// Remove any stale workflow runs. As only the current workflow run should
	// be shown because it is the workflow that reflects the correct state of the pull request.
	//
	// Note: This is run for all workflow runs triggered by an event from an internal contributor,
	// but has to be run again in cron workflow because workflows triggered by external contributors do not
	// grant the Github actions token the correct permissions to dismiss workflow runs.
	err := c.dismissStaleWorkflowRuns(ctx, pr.RepoOwner, pr.RepoName, pr.BranchName)
	if err != nil {
		return trace.Wrap(err)
	}
	mostRecentReviews, err := c.getMostRecentReviews(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// If an admin approves a pull request, allow the check workflow run to pass.
	// This is used in the event the pull request needs the bypassing
	// of the required reviewers.
	if repoAdminHasApproved(mostRecentReviews) {
		return nil
	}
	log.Printf("Checking if %v has approvals from the required reviewers %+v", pr.Author, c.Environment.GetReviewersForAuthor(pr.Author))
	err = hasRequiredApprovals(mostRecentReviews, c.Environment.GetReviewersForAuthor(pr.Author))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// repoAdminHasApproved checks that at least one listed admin has approved.
func repoAdminHasApproved(reviews map[string]review) bool {
	for _, adminName := range ci.RepoAdmins {
		if hasApproved(adminName, reviews) {
			return true
		}
	}
	return false
}

// checkExternal is called to check if a PR reviewed and approved by the
// required reviewers for external contributors. Approvals for external
// contributors are dismissed when new changes are pushed to the PR. The only
// case in which reviews are not dismissed is if they are from GitHub and
// only update the PR.
func (c *Bot) checkExternal(ctx context.Context) error {
	var obsoleteReviews map[string]review
	var validReviews map[string]review

	pr := c.Environment.Metadata
	mostRecentReviews, err := c.getMostRecentReviews(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	validReviews, obsoleteReviews = splitReviews(pr.HeadSHA, mostRecentReviews)
	// External contributions require tighter scrutiny than team
	// contributions. As such reviews from previous pushes must
	// not carry over to when new changes are added. Github does
	// not do this automatically, so we must dismiss the reviews
	// manually if there is a file change.
	if err = c.hasFileChangeFromLastApprovedReview(ctx); err != nil {
		err = c.invalidateApprovals(ctx, obsoleteReviews)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// If there are no file changes between current commit and commit where all
		// reviewers have approved, then all most recent reviews are valid.
		validReviews = mostRecentReviews
	}
	log.Printf("Checking if %v has approvals from the required reviewers %+v", pr.Author, c.Environment.GetReviewersForAuthor(pr.Author))
	err = hasRequiredApprovals(validReviews, c.Environment.GetReviewersForAuthor(pr.Author))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// splitReviews splits a list of reviews into two lists: `valid` (those reviews that refer to
// the current PR head revision) and `obsolete` (those that do not)
func splitReviews(headSHA string, reviews map[string]review) (valid, obsolete map[string]review) {
	valid = make(map[string]review)
	obsolete = make(map[string]review)
	for _, r := range reviews {
		if r.commitID == headSHA {
			valid[r.name] = r
		} else {
			obsolete[r.name] = r
		}
	}
	return valid, obsolete
}

// hasRequiredApprovals determines if all required reviewers have approved.
func hasRequiredApprovals(mostRecentReviews map[string]review, required []string) error {
	if len(mostRecentReviews) == 0 {
		return trace.BadParameter("pull request has no approvals")
	}
	var waitingOnApprovalsFrom []string
	for _, requiredReviewer := range required {
		ok := hasApproved(requiredReviewer, mostRecentReviews)
		if !ok {
			waitingOnApprovalsFrom = append(waitingOnApprovalsFrom, requiredReviewer)
		}
	}
	if len(waitingOnApprovalsFrom) > 0 {
		return trace.BadParameter("required reviewers have not yet approved, waiting on approval(s) from %v", waitingOnApprovalsFrom)
	}
	return nil
}

func (c *Bot) getMostRecentReviews(ctx context.Context) (map[string]review, error) {
	env := c.Environment
	pr := c.Environment.Metadata
	reviews, _, err := env.Client.PullRequests.ListReviews(ctx, pr.RepoOwner,
		pr.RepoName,
		pr.Number,
		&github.ListOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentReviewsSlice := []review{}
	for _, rev := range reviews {
		// Because PRs can be submitted by anyone, input here is attacker controlled
		// and do strict validation of input.
		err := validateReviewFields(rev)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		currReview := review{
			name:        *rev.User.Login,
			status:      *rev.State,
			commitID:    *rev.CommitID,
			id:          *rev.ID,
			submittedAt: rev.SubmittedAt,
		}
		currentReviewsSlice = append(currentReviewsSlice, currReview)
	}
	return mostRecent(currentReviewsSlice), nil
}

// review is a pull request review
type review struct {
	name        string
	status      string
	commitID    string
	id          int64
	submittedAt *time.Time
}

// validateReviewFields validates required fields exist and passes them
// through a restrictive allow list (alphanumerics only). This is done to
// mitigate impact of attacker controlled input (the PR).
func validateReviewFields(review *github.PullRequestReview) error {
	switch {
	case review.ID == nil:
		return trace.Errorf("review ID is nil. review: %+v", review)
	case review.State == nil:
		return trace.Errorf("review State is nil. review: %+v", review)
	case review.CommitID == nil:
		return trace.Errorf("review CommitID is nil. review: %+v", review)
	case review.SubmittedAt == nil:
		return trace.Errorf("review SubmittedAt is nil. review: %+v", review)
	case review.User.Login == nil:
		return trace.Errorf("reviewer User.Login is nil. review: %+v", review)
	}
	if err := validateField(*review.State); err != nil {
		return trace.Errorf("review ID err: %v", err)
	}
	if err := validateField(*review.CommitID); err != nil {
		return trace.Errorf("commit ID err: %v", err)
	}
	if err := validateField(*review.User.Login); err != nil {
		return trace.Errorf("user login err: %v", err)
	}
	return nil
}

// mostRecent returns a list of the most recent review from each required reviewer.
func mostRecent(currentReviews []review) map[string]review {
	mostRecentReviews := make(map[string]review)
	for _, rev := range currentReviews {
		val, ok := mostRecentReviews[rev.name]
		if !ok {
			mostRecentReviews[rev.name] = rev
		} else {
			setTime := val.submittedAt
			currTime := rev.submittedAt
			if currTime.After(*setTime) {
				mostRecentReviews[rev.name] = rev
			}
		}
	}
	return mostRecentReviews
}

// hasApproved determines if the reviewer has submitted an approval
// for the pull request.
func hasApproved(reviewer string, reviews map[string]review) bool {
	for _, rev := range reviews {
		if rev.name == reviewer && rev.status == ci.Approved {
			return true
		}
	}
	return false
}

// dimissMessage returns the dimiss message when a review is dismissed
func dismissMessage(pr *environment.Metadata, required []string) string {
	var sb strings.Builder
	sb.WriteString("New commit pushed, please re-review ")
	for _, reviewer := range required {
		sb.WriteString(fmt.Sprintf("@%s ", reviewer))
	}
	return strings.TrimSpace(sb.String())
}

// hasFileChangeFromLastApproved checks if there is a file change from the last commit all
// reviewers approved (if all reviewers approved at a commit) to the current HEAD.
func (c *Bot) hasFileChangeFromLastApprovedReview(ctx context.Context) error {
	pr := c.Environment.Metadata
	lastReviewCommitID, err := c.getLastApprovedReviewCommitID(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	mostRecent, err := c.getMostRecentReviews(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// Make sure all approvals are at the same commit.
	err = hasAllRequiredApprovalsAtCommit(lastReviewCommitID, mostRecent, c.Environment.GetReviewersForAuthor(pr.Author))
	if err != nil {
		return trace.Wrap(err)
	}
	// Check for any differences
	err = c.hasFileDiff(ctx, lastReviewCommitID, pr.HeadSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getLastApprovedReviewCommitID gets the last review's commit ID (last review where a commit was approved).
func (c *Bot) getLastApprovedReviewCommitID(ctx context.Context) (string, error) {
	pr := c.Environment.Metadata
	clt := c.Environment.Client
	reviews, _, err := clt.PullRequests.ListReviews(ctx, pr.RepoOwner, pr.RepoName, pr.Number, &github.ListOptions{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(reviews) == 0 {
		return "", trace.NotFound("pull request has no reviews")
	}

	// Sort reviews from newest to oldest.
	sort.Slice(reviews, func(i, j int) bool {
		time1, time2 := reviews[i].SubmittedAt, reviews[j].SubmittedAt
		return time2.Before(*time1)
	})
	var lastApprovedReview *github.PullRequestReview
	// Find last approved review.
	for _, review := range reviews {
		if review.State == nil {
			continue
		}
		if *review.State == ci.Approved {
			lastApprovedReview = review
			break
		}
	}
	if lastApprovedReview == nil {
		return "", trace.NotFound("no approved reviews found")
	}
	if lastApprovedReview.CommitID == nil {
		return "", trace.NotFound("commit ID not found")
	}
	return *lastApprovedReview.CommitID, nil
}

// hasFileDiff compares two commits and checks if there are changes.
func (c *Bot) hasFileDiff(ctx context.Context, base, head string) error {
	pr := c.Environment.Metadata
	clt := c.Environment.Client
	comparison, _, err := clt.Repositories.CompareCommits(ctx, pr.RepoOwner, pr.RepoName, base, head)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(comparison.Files) != 0 {
		return trace.Errorf("detected file change")
	}
	return nil
}

func hasAllRequiredApprovalsAtCommit(commitSHA string, reviews map[string]review, required []string) error {
	for _, requiredReviewer := range required {
		review, ok := reviews[requiredReviewer]
		if !ok {
			return trace.BadParameter("all reviewers have not approved")
		}
		if review.commitID != commitSHA {
			return trace.Errorf("all reviewers have not approved at %s", commitSHA)
		}
	}
	return nil
}

// invalidateApprovals dismisses all approved reviews on a pull request.
func (c *Bot) invalidateApprovals(ctx context.Context, reviews map[string]review) error {
	pr := c.Environment.Metadata
	msg := dismissMessage(pr, c.Environment.GetReviewersForAuthor(pr.Author))
	for _, v := range reviews {
		if pr.HeadSHA != v.commitID && v.status != ci.Commented {
			_, _, err := c.Environment.Client.PullRequests.DismissReview(ctx,
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				v.id,
				&github.PullRequestReviewDismissalRequest{Message: &msg},
			)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}
