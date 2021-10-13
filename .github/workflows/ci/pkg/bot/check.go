package bot

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
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
	commit, err := c.getValidCommit(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	signature := *commit.Commit.Verification.Signature
	payloadData := *commit.Commit.Verification.Payload

	signatureFileName, err := createAndWriteTempFile(ci.Signature, signature)
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(signatureFileName)

	payloadFileName, err := createAndWriteTempFile(ci.Payload, payloadData)
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(payloadFileName)

	// GPG verification command requires the signature as the first argument
	// Runner must have gpg (GnuPG) installed.
	cmd := exec.Command("gpg", "--verify", signatureFileName, payloadFileName)
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return trace.BadParameter("commit is not verified and/or is not signed by GitHub")
		}
		return trace.Wrap(err)
	}
	return c.hasFileChange(ctx)
}

// hasFileChange compares all of the files that have changes in the PR to the one at the current commit.
// This is used for comparing files when Github makes a auto update branch commit to ensure the merge
// didn't result in changes to the files already in the PR.
func (c *Bot) hasFileChange(ctx context.Context) error {
	pr := c.Environment.Metadata
	clt := c.Environment.Client
	prFiles, _, err := clt.PullRequests.ListFiles(ctx, pr.RepoOwner, pr.RepoName, pr.Number, &github.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	headCommit, _, err := clt.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, pr.HeadSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, headFile := range headCommit.Files {
		for _, prFile := range prFiles {
			if *headFile.Filename == *prFile.Filename {
				return trace.BadParameter("detected file change")
			}
		}
	}
	return nil
}

// getValidCommit returns a valid repository commit to perform GPG signature verification on.
// A valid commit is one that has a signature and a payload.
func (c *Bot) getValidCommit(ctx context.Context) (*github.RepositoryCommit, error) {
	pr := c.Environment.Metadata
	commit, _, err := c.Environment.Client.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, pr.HeadSHA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if commit.Commit.Verification.Signature == nil || commit.Commit.Verification.Payload == nil {
		return nil, trace.BadParameter("commit is not signed")
	}
	return commit, nil
}

// createAndWriteTempFile creates a temp file and write data to it.
// This function returns the file name (string) and error.
func createAndWriteTempFile(prefix, data string) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), prefix)
	if err != nil {
		return "", trace.Wrap(err)
	}
	contents := []byte(data)
	if _, err = file.Write(contents); err != nil {
		return "", trace.Wrap(err)
	}
	if err := file.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	return file.Name(), nil
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
