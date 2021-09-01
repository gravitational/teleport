package bot

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
)

// Config is used to configure Bot
type Config struct {
	Environment *environment.Environment
}

// Bot assigns reviewers and checks assigned reviewers for a pull request
type Bot struct {
	Environment *environment.Environment
	invalidate  invalidate
	verify      verify
}

type invalidate func(string, string, string, int, []review, *github.Client) error
type verify func(string, string, string, string) error

// New returns a new instance of  Bot
func New(c Config) (*Bot, error) {
	var ch Bot
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ch.Environment = c.Environment
	ch.invalidate = invalidateApprovals
	ch.verify = verifyCommit
	return &ch, nil
}

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Environment == nil {
		return trace.BadParameter("missing parameter Environment.")
	}
	return nil
}

// invalidateApprovals dismisses all reviews on a pull request
func invalidateApprovals(repoOwner, repoName, msg string, number int, reviews []review, clt *github.Client) error {
	for _, v := range reviews {
		_, _, err := clt.PullRequests.DismissReview(context.TODO(), repoOwner, repoName, number, v.id, &github.PullRequestReviewDismissalRequest{Message: &msg})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DismissStaleWorkflowRuns dismisses stale Check workflow runs
func DismissStaleWorkflowRuns(token, owner, repoName, branch string, cl *github.Client) error {
	var targetWorkflow *github.Workflow
	workflows, _, err := cl.Actions.ListWorkflows(context.TODO(), owner, repoName, &github.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, w := range workflows.Workflows {
		if *w.Name == ci.CHECKWORKFLOW {
			targetWorkflow = w
			break
		}
	}
	list, _, err := cl.Actions.ListWorkflowRunsByID(context.TODO(), owner, repoName, *targetWorkflow.ID, &github.ListWorkflowRunsOptions{Branch: branch})
	if err != nil {
		return trace.Wrap(err)
	}
	sort.Sort(ByTime(list.WorkflowRuns))

	// Deleting all runs except the most recently started one.
	for i := 0; i < len(list.WorkflowRuns)-1; i++ {
		run := list.WorkflowRuns[i]
		err := deleteRun(token, owner, repoName, *run.ID)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// deleteRun deletes a workflow run.
// Note: the go-github client library does not support this endpoint.
func deleteRun(token, owner, repo string, runID int64) error {
	// Creating and authenticating the client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	client := oauth2.NewClient(context.Background(), ts)
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%v", owner, repo, runID)
	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Println(string(bodyBytes))
	return nil
}

type ByTime []*github.WorkflowRun

func (s ByTime) Len() int {
	return len(s)
}

func (s ByTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByTime) Less(i, j int) bool {
	time1, time2 := s[i].CreatedAt, s[j].CreatedAt
	return time1.Time.Before(time2.Time)
}
