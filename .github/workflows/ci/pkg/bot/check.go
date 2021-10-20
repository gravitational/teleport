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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
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
	log.Printf("Checking if %v has approvals from the required reviewers %+v", pr.Author, c.Environment.GetReviewersForAuthor(pr.Author))
	err = hasRequiredApprovals(mostRecentReviews, c.Environment.GetReviewersForAuthor(pr.Author))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	// manually.
	if err = c.isValidGithubBranchUpdate(ctx); err != nil {
		err = c.invalidateApprovals(ctx, obsoleteReviews)
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
	sb.WriteString("new commit pushed, please re-review ")
	for _, reviewer := range required {
		sb.WriteString(fmt.Sprintf("@%s", reviewer))
	}
	return sb.String()
}

// isValidGithubBranchUpdate validates a merge into the current branch from master.
func (c *Bot) isValidGithubBranchUpdate(ctx context.Context) error {
	commits, err := c.getAllCommits(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	betweenCommits, err := c.getInBetweenCommits(ctx, commits)
	if err != nil {
		return trace.Wrap(err)
	}
	// Comparing all commits in between HEAD (in this case `web-flow`)and last non
	// Github-committed commit to ensure an attacker didn't slip in a malicious commit.
	for _, currCommit := range betweenCommits {
		err = c.compareCommits(ctx, currCommit.SHA)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// compareCommits compares all of the files that have changes in the passed in commit to the one at the current commit (HEAD).
// This is used for comparing files when Github makes a auto update branch commit to ensure the merge
// didn't result in changes to the files already under review.
func (c *Bot) compareCommits(ctx context.Context, otherSHA string) error {
	clt := c.Environment.Client
	pr := c.Environment.Metadata
	// non Github-committed commit
	nonGithubCommitted, _, err := clt.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, otherSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	// Github-committed commit
	headCommit, _, err := clt.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, pr.HeadSHA)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, headFile := range headCommit.Files {
		for _, prFile := range nonGithubCommitted.Files {
			if *headFile.Filename == *prFile.Filename || *headFile.SHA == *prFile.SHA {
				return trace.BadParameter("detected file change")
			}
		}
	}
	return nil
}

// invalidateApprovals dismisses all approved reviews on a pull request.
func (c *Bot) invalidateApprovals(ctx context.Context, reviews map[string]review) error {
	pr := c.Environment.Metadata
	msg := dismissMessage(pr, c.Environment.GetReviewersForAuthor(pr.Author))
	for _, v := range reviews {
		if pr.HeadSHA != v.commitID {
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

// getInBetweenCommits gets the commits in between the current HEAD commit and the last
// commit that was not committed by Github's committer `web-flow` and the last commit not committed
// by Github is included in the return.
func (c *Bot) getInBetweenCommits(ctx context.Context, commits []githubCommit) ([]githubCommit, error) {
	inBetweenCommits := []githubCommit{}

	// Sorting commits by time from newest to oldest.
	sort.Slice(commits, func(i, j int) bool {
		time1, time2 := commits[i].Commit.Committer.Date, commits[j].Commit.Committer.Date
		return time2.Before(time1)
	})
	if len(commits) < 2 {
		return nil, trace.BadParameter("no commits to check against HEAD")
	}
	// Pop off HEAD (most recent commit) because we will be comparing the "in between" commits against it.
	commits = commits[1:]

	for _, commit := range commits {
		if commit.Committer.Login != ci.WebFlowUserName {
			// Include most recent non Github-committed commit beacause
			// HEAD should also not have a diff against it.
			inBetweenCommits = append(inBetweenCommits, commit)
			return inBetweenCommits, nil
		}
		inBetweenCommits = append(inBetweenCommits, commit)
	}
	// If this method doesn't return in the loop that means the only type of commits in the
	// PR are committed by `web-flow`.
	return nil, trace.BadParameter("commits were only committed by web-flow")
}

// getAllCommits gets all the commits for the current pull request in context.
func (c *Bot) getAllCommits(ctx context.Context) ([]githubCommit, error) {
	pr := c.Environment.Metadata
	clt := c.Environment.Client
	pullRequestNumberString := strconv.Itoa(pr.Number)
	commitsURL := url.URL{
		Scheme: scheme,
		Host:   githubAPIHostname,
		Path:   path.Join("repos", pr.RepoOwner, pr.RepoName, "pulls", pullRequestNumberString, "commits"),
	}
	// Creating an HTTP request instead of using the `go-github` directly to list commits
	// because they don't support a way to get all the commits for a pull request.
	// It seems they only support getting as many commits as a the GH API can return per page.
	// In this case, 100.
	req, err := clt.NewRequest(http.MethodGet, commitsURL.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var allCommits githubCommits
	currentPage := 1
	// Using pagination in the event there is a pull request with many (>100) commits.
	for currentPage != 0 {
		pageNumberString := strconv.Itoa(currentPage)
		req.URL.RawQuery = populateQuery(pr.BranchName, pageNumberString).Encode()

		// BareDo() is a method on github.Client where the caller
		// can handle the response instead of `go-github` handling it.
		res, err := clt.BareDo(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Unmarshalling the response and appending
		// current page's commits to allCommits.
		var currentList githubCommits
		err = json.Unmarshal(body, &currentList)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allCommits = append(allCommits, currentList...)
		currentPage = res.NextPage
	}
	return allCommits, nil
}

func populateQuery(branchName, pageNumber string) url.Values {
	return url.Values{
		"ref":      []string{branchName},
		"per_page": []string{"100"},
		"page":     []string{pageNumber},
	}
}

type githubCommits []githubCommit

type githubCommit struct {
	Committer struct {
		Login string `json:"login"`
	} `json:"committer"`
	Commit struct {
		Committer struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
	SHA string `json:"sha"`
}
