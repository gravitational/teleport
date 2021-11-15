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

package environment

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"os"
	"strings"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/google/go-github/v37/github"
)

// Config is used to configure Environment
type Config struct {
	// Context is the context for Environment
	Context context.Context
	// Client is the authenticated Github client.
	Client *github.Client
	// Reviewers is a map that maps authors to their respective
	// required reviewers.
	Reviewers map[string][]string
	// EventPath is the path of the file with the complete
	// webhook event payload on the runner.
	EventPath string
}

// PullRequestEnvironment contains information about the environment
type PullRequestEnvironment struct {
	// Client is the authenticated Github client
	Client *github.Client
	// Metadata is the pull request in the
	// current context.
	Metadata *Metadata
	// reviewers is a map of reviewers where the key
	// is the user name of the author and the value is a list
	// of required reviewers.
	reviewers map[string][]string
	// defaultReviewers is a list of reviewers used for authors whose
	// usernames are not a key in `reviewers`
	defaultReviewers []string
	// action is the action that triggered the workflow.
	action string
	// HasDocsChanges tells if the pull request has changes to `docs/`.
	HasDocsChanges bool
	// HasCodeChanges tells if the pull request has changes in any directory besides `docs/`.
	HasCodeChanges bool
}

// Metadata is the current pull request metadata
type Metadata struct {
	// Author is the pull request author.
	Author string
	// RepoName is the repository name that the
	// current pull request is trying to merge into.
	RepoName string
	// RepoOwner is the owner of the repository the
	// author is trying to merge into.
	RepoOwner string
	// Number is the pull request number.
	Number int
	// HeadSHA is the commit sha of the author's branch.
	HeadSHA string
	// BaseSHA is the commit sha of the base branch.
	BaseSHA string
	// Reviewer is the reviewer's Github username.
	// Only used for pull request review events.
	Reviewer string
	// BranchName is the name of the branch the author
	// is trying to merge in.
	BranchName string
}

// CheckAndSetDefaults verifies configuration and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if c.Reviewers == nil {
		return trace.BadParameter("missing parameter Reviewers")
	}
	if _, ok := c.Reviewers[ci.AnyAuthor]; !ok {
		return trace.BadParameter(`default reviewers are not set in reviewers map. set default reviewers with a wildcard (*) as a key`)
	}
	if c.EventPath == "" {
		c.EventPath = os.Getenv(ci.GithubEventPath)
	}
	return nil
}

// New creates a new instance of Environment.
func New(c Config) (*PullRequestEnvironment, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pr, err := GetMetadata(c.EventPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Check if the pull request has changes to the `docs/` directory as that will affect who
	// the required reviewers are.
	docChanges, codeChanges, err := hasChanges(c.Context, pr, c.Client)
	if err != nil {
		// Log the error, don't fail the whole run because it couldn't detect if the pull request has docs.
		log.Errorf("Error while detecting pull request for docs changes: %v, skipping assigning docs reviewers", err)
	}

	return &PullRequestEnvironment{
		Client:           c.Client,
		reviewers:        c.Reviewers,
		defaultReviewers: c.Reviewers[ci.AnyAuthor],
		Metadata:         pr,
		HasDocsChanges:   docChanges,
		HasCodeChanges:   codeChanges,
	}, nil
}

// hasChanges determines if the pull request has docs and/or code changes.
func hasChanges(ctx context.Context, pr *Metadata, clt *github.Client) (hasDocsChanges bool, hasCodeChanges bool, err error) {
	files, err := getPullRequestFiles(ctx, pr, clt)
	if err != nil {
		return false, true, trace.Wrap(err)
	}
	for _, file := range files {
		if file.Filename == nil {
			continue
		}
		if hasDocChanges(*file.Filename) {
			hasDocsChanges = true
		} else {
			hasCodeChanges = true
		}

	}
	return hasDocsChanges, hasCodeChanges, nil
}

// hasDocChanges checks if the file name contains a prefix or suffix that would suggest a change to docs.
// Prefix of "rfd/" or "docs/".
// Suffix of ".mdx", or ".md".
func hasDocChanges(filename string) bool {
	if strings.HasPrefix(filename, ci.VendorPrefix) {
		return false
	}
	return strings.HasPrefix(filename, ci.DocsPrefix) ||
		strings.HasSuffix(filename, ci.MdSuffix) ||
		strings.HasSuffix(filename, ci.MdxSuffix) ||
		strings.HasPrefix(filename, ci.RfdPrefix)
}

// getPullRequestFiles gets all the files in the pull request.
func getPullRequestFiles(ctx context.Context, pr *Metadata, clt *github.Client) ([]*github.CommitFile, error) {
	// Can only request 100 files per page according to the docs.
	// https://docs.github.com/en/free-pro-team@latest/rest/reference/pulls/#list-pull-requests-files
	opt := &github.ListOptions{PerPage: 100}

	var allFiles []*github.CommitFile
	for {
		files, resp, err := clt.PullRequests.ListFiles(ctx, pr.RepoOwner, pr.RepoName, pr.Number, opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allFiles = append(allFiles, files...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allFiles, nil
}

// GetReviewersForAuthor gets the required reviewers for the current user.
func (e *PullRequestEnvironment) GetReviewersForAuthor(user string) []string {
	var reviewers []string
	if e.HasDocsChanges {
		reviewers = append(reviewers, ci.DocReviewers...)
		if !e.HasCodeChanges {
			return reviewers
		}
	}
	rr, ok := e.reviewers[user]
	if ok {
		reviewers = append(reviewers, rr...)
	} else {
		reviewers = append(reviewers, e.defaultReviewers...)
	}
	return reviewers
}

// IsInternal determines if an author is an internal contributor.
func (e *PullRequestEnvironment) IsInternal(author string) bool {
	_, ok := e.reviewers[author]
	return ok
}

// GetMetadata gets the pull request metadata in the current context.
func GetMetadata(path string) (*Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()
	body, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var actionType action
	err = json.Unmarshal(body, &actionType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return getMetadata(body, actionType.Action)
}

func getMetadata(body []byte, action string) (*Metadata, error) {
	var pr *Metadata

	switch action {
	case ci.Synchronize:
		var push PushEvent
		err := json.Unmarshal(body, &push)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pr, err = push.toMetadata()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case ci.Assigned, ci.Opened, ci.Reopened, ci.Ready:
		var pull PullRequestEvent
		err := json.Unmarshal(body, &pull)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pr, err = pull.toMetadata()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case ci.Submitted, ci.Created:
		var rev ReviewEvent
		err := json.Unmarshal(body, &rev)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pr, err = rev.toMetadata()
		if err != nil {
			return nil, err
		}
	default:
		return nil, trace.BadParameter("unknown action %s", action)
	}
	return pr, nil
}

func (r *ReviewEvent) toMetadata() (*Metadata, error) {
	m := &Metadata{
		Number:     r.PullRequest.Number,
		Author:     r.PullRequest.Author.Login,
		RepoOwner:  r.Repository.Owner.Name,
		RepoName:   r.Repository.Name,
		HeadSHA:    r.PullRequest.Head.SHA,
		BaseSHA:    r.PullRequest.Base.SHA,
		BranchName: r.PullRequest.Head.BranchName,
		Reviewer:   r.Review.User.Login,
	}
	if m.Reviewer == "" {
		return nil, trace.BadParameter("missing reviewer username")
	}
	if err := m.validateFields(); err != nil {
		return nil, err
	}
	return m, nil
}

func (p *PullRequestEvent) toMetadata() (*Metadata, error) {
	m := &Metadata{
		Number:     p.Number,
		Author:     p.PullRequest.User.Login,
		RepoOwner:  p.Repository.Owner.Name,
		RepoName:   p.Repository.Name,
		HeadSHA:    p.PullRequest.Head.SHA,
		BaseSHA:    p.PullRequest.Base.SHA,
		BranchName: p.PullRequest.Head.BranchName,
	}
	if err := m.validateFields(); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *PushEvent) toMetadata() (*Metadata, error) {
	m := &Metadata{
		Number:     s.Number,
		Author:     s.PullRequest.User.Login,
		RepoOwner:  s.Repository.Owner.Name,
		RepoName:   s.Repository.Name,
		HeadSHA:    s.CommitSHA,
		BaseSHA:    s.BeforeSHA,
		BranchName: s.PullRequest.Head.BranchName,
	}
	if err := m.validateFields(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Metadata) validateFields() error {
	switch {
	case m.Number == 0:
		return trace.BadParameter("missing pull request number")
	case m.Author == "":
		return trace.BadParameter("missing user login")
	case m.RepoOwner == "":
		return trace.BadParameter("missing repository owner")
	case m.RepoName == "":
		return trace.BadParameter("missing repository name")
	case m.HeadSHA == "":
		return trace.BadParameter("missing head commit sha")
	case m.BaseSHA == "":
		return trace.BadParameter("missing base commit sha")
	case m.BranchName == "":
		return trace.BadParameter("missing branch name")
	}
	return nil
}
