package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/trace"
)

// Check checks if all the reviewers have approved the pull request in the current context.
func (c *Bot) Check(ctx context.Context) error {
	// Check if the assigned reviewers have approved this PR.
	err := c.check(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// check checks to see if all the required reviewers have approved and invalidates
// approvals for external contributors if a new commit is pushed
func (c *Bot) check(ctx context.Context) error {
	pr := c.Environment.Metadata
	if c.Environment.IsInternal(pr.Author) {
		return c.checkInternal(ctx)
	}
	return c.checkExternal(ctx)
}

func (c *Bot) checkInternal(ctx context.Context) error {
	pr := c.Environment.Metadata
	// Remove any stale workflow runs. As only the current workflow run should
	// be shown because it is the workflow that reflects the correct state of the pull request.
	//
	// Note: This is run for all workflow runs triggered by an event from an internal contributor,
	// but has to be run again in cron workflow because workflows triggered by external contributors do not
	// grant the Github actions token the correct permissions to dismiss workflow runs.
	err := c.DismissStaleWorkflowRuns(ctx, pr.RepoOwner, pr.RepoName, pr.BranchName)
	if err != nil {
		return trace.Wrap(err)
	}
	mostRecentReviews, err := c.getMostRecentReviews(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Printf("Checking if %v has approvals from the required reviewers %+v", pr.Author, c.Environment.GetReviewersForAuthor(pr.Author))
	err = hasRequiredApprovals(mostRecentReviews, c.Environment.GetReviewersForAuthor(pr.Author))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Bot) checkExternal(ctx context.Context) error {
	var obsoleteReviews []review
	var validReviews []review

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
	// manually.
	if err = c.isGithubCommit(ctx); err != nil {
		msg := dismissMessage(pr, c.Environment.GetReviewersForAuthor(pr.Author))
		err = c.invalidateApprovals(ctx, msg, obsoleteReviews)
		if err != nil {
			return trace.Wrap(err)
		}
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
func splitReviews(headSHA string, reviews []review) (valid, obsolete []review) {
	for _, r := range reviews {
		if r.commitID == headSHA {
			valid = append(valid, r)
		} else {
			obsolete = append(obsolete, r)
		}
	}
	return
}

// hasRequiredApprovals determines if all required reviewers have approved.
func hasRequiredApprovals(mostRecentReviews []review, required []string) error {
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

func (c *Bot) getMostRecentReviews(ctx context.Context) ([]review, error) {
	env := c.Environment
	pr := c.Environment.Metadata
	reviews, _, err := env.Client.PullRequests.ListReviews(ctx, pr.RepoOwner,
		pr.RepoName,
		pr.Number,
		&github.ListOptions{})
	if err != nil {
		return []review{}, trace.Wrap(err)
	}
	currentReviewsSlice := []review{}
	for _, rev := range reviews {
		err := checkReviewFields(rev)
		if err != nil {
			return []review{}, trace.Wrap(err)
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

func checkReviewFields(review *github.PullRequestReview) error {
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
	return nil
}

// mostRecent returns a list of the most recent review from each required reviewer.
func mostRecent(currentReviews []review) []review {
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
	reviews := make([]review, 0, len(mostRecentReviews))
	for _, v := range mostRecentReviews {
		reviews = append(reviews, v)
	}
	return reviews
}

// hasApproved determines if the reviewer has submitted an approval
// for the pull request.
func hasApproved(reviewer string, reviews []review) bool {
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
	sb.WriteString("new commit pushed, please re-review ")
	for _, reviewer := range required {
		sb.WriteString(fmt.Sprintf("@%s", reviewer))
	}
	return sb.String()
}

// isGithubCommit verfies GitHub is the commit author and that the commit is empty.
// Commits are checked for verification and emptiness specifically to determine if a
// pull request's reviews should be invalidated. If a commit is signed by Github and is empty
// there is no need to invalidate commits because the branch is just being updated.
func (c *Bot) isGithubCommit(ctx context.Context) error {
	pr := c.Environment.Metadata
	comparison, _, err := c.Environment.Client.Repositories.CompareCommits(
		ctx,
		pr.RepoOwner,
		pr.RepoName,
		pr.BaseSHA,
		pr.HeadSHA,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(comparison.Files) != 0 {
		return trace.BadParameter("detected file change")
	}
	commit, _, err := c.Environment.Client.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, pr.HeadSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	verification := commit.Commit.Verification
	if verification != nil {
		if verification.Payload != nil && verification.Verified != nil {
			payload := *verification.Payload
			// If commit is empty (no file changes) and the commit is signed by Github,
			// there is no need to invalidate the commit.
			if strings.Contains(payload, ci.GithubCommit) && *verification.Verified {
				return nil
			}
		}
	}
	return trace.BadParameter("commit is not verified and/or is not signed by GitHub")
}

// invalidateApprovals dismisses all approved reviews on a pull request.
func (c *Bot) invalidateApprovals(ctx context.Context, msg string, reviews []review) error {
	pr := c.Environment.Metadata
	for _, v := range reviews {
		if v.status == ci.Approved && pr.HeadSHA != v.commitID {
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
