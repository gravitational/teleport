package bot

import (
	"bytes"
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
func (c *Bot) Check() error {
	env := c.Environment
	pr := c.Environment.PullRequest
	if c.Environment.IsInternal(pr.Author) {
		err := DismissStaleWorkflowRuns(env.GetToken(), pr.RepoOwner, pr.RepoName, pr.BranchName, env.Client)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	listOpts := github.ListOptions{}
	reviews, _, err := env.Client.PullRequests.ListReviews(context.TODO(), pr.RepoOwner,
		pr.RepoName,
		pr.Number,
		&listOpts)
	if err != nil {
		return trace.Wrap(err)
	}
	currentReviewsSlice := []review{}
	for _, rev := range reviews {
		currReview := review{name: *rev.User.Login, status: *rev.State, commitID: *rev.CommitID, id: *rev.ID, submittedAt: rev.SubmittedAt}
		currentReviewsSlice = append(currentReviewsSlice, currReview)
	}
	return c.check(c.Environment.IsInternal(pr.Author), pr, c.Environment.GetReviewersForAuthor(pr.Author), mostRecent(currentReviewsSlice))
}

// check checks to see if all the required reviewers have approved and invalidates
// approvals for external contributors if a new commit is pushed
func (c *Bot) check(isInternal bool, pr *environment.PullRequestMetadata, required []string, currentReviews []review) error {
	if len(currentReviews) == 0 {
		return trace.BadParameter("pull request has no reviews.")
	}
	log.Printf("checking if %v has approvals from the required reviewers %+v", pr.Author, required)
	for _, requiredReviewer := range required {
		if !containsApprovalReview(requiredReviewer, currentReviews) {
			return trace.BadParameter("all required reviewers have not yet approved.")
		}
	}

	if hasNewCommit(pr.HeadSHA, currentReviews) && !isInternal {
		// Check file changes/commit verification
		err := c.verify(pr.RepoOwner, pr.RepoName, pr.BaseSHA, pr.HeadSHA)
		if err != nil {
			if validationErr := c.invalidate(pr.RepoOwner, pr.RepoName, dismissMessage(pr, required), pr.Number, currentReviews, c.Environment.Client); validationErr != nil {
				return trace.Wrap(validationErr)
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// mostRecent returns a list of the most recent review from each required reviewer
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
	reviews := []review{}
	for _, v := range mostRecentReviews {
		reviews = append(reviews, v)
	}
	return reviews
}

// review is a pull request review
type review struct {
	name        string
	status      string
	commitID    string
	id          int64
	submittedAt *time.Time
}

func containsApprovalReview(reviewer string, reviews []review) bool {
	for _, rev := range reviews {
		if rev.name == reviewer && rev.status == ci.APPROVED {
			return true
		}
	}
	return false
}

// dimissMessage returns the dimiss message when a review is dismissed
func dismissMessage(pr *environment.PullRequestMetadata, required []string) string {
	var buffer bytes.Buffer
	buffer.WriteString("New commit pushed, please rereview ")
	for _, reviewer := range required {
		buffer.WriteString(fmt.Sprintf("@%v ", reviewer))
	}
	return buffer.String()
}

// hasNewCommit sees if the pull request has a new commit
// by comparing commits after the push event
func hasNewCommit(headSHA string, revs []review) bool {
	for _, v := range revs {
		if v.commitID != headSHA {
			return true
		}
	}
	return false
}

// verifyCommit verfies GitHub is the commit author and that the commit is empty
func verifyCommit(repoOwner, repoName, baseSHA, headSHA string) error {
	client := github.NewClient(nil)
	comparison, _, err := client.Repositories.CompareCommits(context.TODO(), repoOwner, repoName, baseSHA, headSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(comparison.Files) != 0 {
		return trace.BadParameter("detected file change")
	}
	commit, _, err := client.Repositories.GetCommit(context.TODO(), repoOwner, repoName, headSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	verification := commit.Commit.Verification
	// Get commit object
	payload := *verification.Payload
	if strings.Contains(payload, ci.GITHUBCOMMIT) && *verification.Verified {
		return nil
	}
	return trace.BadParameter("commit is not verified and/or is not signed by GitHub")
}
