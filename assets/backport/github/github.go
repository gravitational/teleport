/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package github

import (
	"context"
	"fmt"
	"strings"

	go_github "github.com/google/go-github/v41/github"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
)

type Client struct {
	Client *go_github.Client
	c      Config
}

type Config struct {
	// Token is the Github auth token.
	Token string

	// Repository is the name of the repository to create
	// the backport pull requests in.
	Repository string

	// Organization is the organization/owner name of the
	// repository.
	Organization string
}

// New returns a new GitHub client.
func New(ctx context.Context, c *Config) (*Client, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.Token},
	)
	return &Client{
		Client: go_github.NewClient(oauth2.NewClient(ctx, ts)),
		c:      *c,
	}, nil
}

// Check validates config.
func (c *Config) Check() error {
	if c.Token == "" {
		return trace.BadParameter("missing parameter Token")
	}
	if c.Organization == "" {
		return trace.BadParameter("missing parameter Organization")
	}
	if c.Repository == "" {
		return trace.BadParameter("missing parameter Repository")
	}
	return nil
}

// Backport backports changes from backportBranchName to a new branch based
// off baseBranchName.
//
// A new branch is created with the name in the format of
// auto-backport/[pull number]-to-[base branch], and
// cherry-picks commits onto the new branch.
func (c *Client) Backport(ctx context.Context, baseBranchName string, pullNumber int) (string, error) {
	newBranchName := fmt.Sprintf("auto-backport/%v-to-%s", pullNumber, baseBranchName)
	// Create a new branch off of the target branch.
	err := c.createBranchFrom(ctx, baseBranchName, newBranchName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	fmt.Printf("Created a new branch: %s.\n", newBranchName)

	commits, err := c.getPullRequestCommits(ctx, pullNumber)
	if err != nil {
		return "", trace.Wrap(err)
	}
	fmt.Printf("Found %v commits. \n", len(commits))

	// Cherry pick commits.
	err = c.cherryPickCommitsOnBranch(ctx, newBranchName, commits)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return newBranchName, nil
}

// CreatePullRequest creates a pull request.
func (c *Client) CreatePullRequest(ctx context.Context, baseBranch string, headBranch string, originalPrNumber int) error {
	pr, _, err := c.Client.PullRequests.Get(ctx, c.c.Organization, c.c.Repository, originalPrNumber)
	if err != nil {
		return trace.Wrap(err)
	}

	major := strings.TrimPrefix(baseBranch, "branch/")
	body := pr.GetBody()
	if len(body) > 0 {
		body += "\n\n"
	}
	body += fmt.Sprintf("Backports #%v", originalPrNumber)

	newPR := &go_github.NewPullRequest{
		Title: go_github.String(fmt.Sprintf("[%v] %v", major, pr.GetTitle())),
		Head:  go_github.String(headBranch),
		Base:  go_github.String(baseBranch),
		Body:  go_github.String(body),
	}
	_, _, err = c.Client.PullRequests.Create(ctx, c.c.Organization, c.c.Repository, newPR)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getPullRequestCommits gets the commits for a pull request.
func (c *Client) getPullRequestCommits(ctx context.Context, number int) (commits []string, err error) {
	var commitSHAs []string
	opts := go_github.ListOptions{
		Page:    0,
		PerPage: perPage,
	}
	for {
		currCommits, resp, err := c.Client.PullRequests.ListCommits(ctx,
			c.c.Organization,
			c.c.Repository,
			number, &go_github.ListOptions{})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, commit := range currCommits {
			commitSHAs = append(commitSHAs, commit.GetSHA())
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
	return commitSHAs, nil
}

// cherryPickCommitsOnBranch cherry picks a list of commits onto the given branch.
func (c *Client) cherryPickCommitsOnBranch(ctx context.Context, branchName string, commits []string) error {
	branch, _, err := c.Client.Repositories.GetBranch(ctx, c.c.Organization, c.c.Repository, branchName, true)
	if err != nil {
		return trace.Wrap(err)
	}
	// Get the branch's HEAD.
	headCommit, _, err := c.Client.Git.GetCommit(ctx,
		c.c.Organization,
		c.c.Repository,
		branch.GetCommit().GetSHA())
	if err != nil {
		return trace.Wrap(err)
	}

	for _, commit := range commits {
		cherryCommit, _, err := c.Client.Git.GetCommit(ctx, c.c.Organization, c.c.Repository, commit)
		if err != nil {
			return trace.Wrap(err)
		}
		// Skip merge commits. The commit to cherry-pick MUST have only 1 parent.
		if len(cherryCommit.Parents) != 1 {
			fmt.Printf("Skipping merge commit: %s\n", cherryCommit.GetMessage())
			continue
		}
		fmt.Printf("%s %s\n", cherryCommit.GetSHA(), cherryCommit.GetMessage())
		tree, sha, err := c.cherryPickCommit(ctx, branchName, cherryCommit, headCommit)
		if err != nil {
			fmt.Printf("failed to cherry pick commit: %s %s\n", cherryCommit.GetSHA(), cherryCommit.GetMessage())
			return trace.Errorf("please manually delete branch %s: %v", branchName, err)
		}
		headCommit.SHA = &sha
		headCommit.Tree = tree
	}
	return nil
}

// cherryPickCommit cherry picks a single commit on a branch.
func (c *Client) cherryPickCommit(ctx context.Context, branchName string, cherryCommit, headBranchCommit *go_github.Commit) (*go_github.Tree, string, error) {
	cherryParent := cherryCommit.Parents[0]

	// Temporarily set the parent of the branch HEAD to the parent of the commit
	// to cherry-pick so they are siblings.
	err := c.createSiblingCommit(ctx, branchName, headBranchCommit, cherryParent)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// When git performs the merge, it detects that the parent of the branch commit that is
	// being merged onto matches the parent of the cherry pick commit, and merges a tree of size 1.
	// The merge commit will contain the delta between the file tree in target branch and the
	// commit to cherry-pick.
	merge, err := c.merge(ctx, branchName, cherryCommit.GetSHA())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	mergeTree := merge.GetTree()

	updatedCommit, _, err := c.Client.Git.GetCommit(ctx,
		c.c.Organization,
		c.c.Repository,
		headBranchCommit.GetSHA())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	// Create the actual cherry-pick commit on the target branch containing the merge commit tree.
	commit, _, err := c.Client.Git.CreateCommit(ctx, c.c.Organization, c.c.Repository, &go_github.Commit{
		Message: cherryCommit.Message,
		Tree:    mergeTree,
		Parents: []*go_github.Commit{
			updatedCommit,
		},
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Overwrite the merge commit and its parent on the branch by the newly created commit.
	// The result will be equivalent to what would have happened with a fast-forward merge.
	sha := commit.GetSHA()
	refName := fmt.Sprintf("%s%s", branchRefPrefix, branchName)
	_, _, err = c.Client.Git.UpdateRef(ctx, c.c.Organization, c.c.Repository, &go_github.Reference{
		Ref: go_github.String(refName),
		Object: &go_github.GitObject{
			SHA: go_github.String(sha),
		},
	}, true)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return mergeTree, sha, nil
}

// createSiblingCommit creates a commit with the passed in commit's tree and parent
// and updates the passed in branch to point at that commit.
func (c *Client) createSiblingCommit(ctx context.Context, branchName string, branchHeadCommit *go_github.Commit, cherryParent *go_github.Commit) error {
	tree := branchHeadCommit.GetTree()

	// This sibling commit is temporary commit to later merge for a tree size of 1.
	// The commit message does not matter as this commit will not be in the final
	// branch.
	commit, _, err := c.Client.Git.CreateCommit(ctx, c.c.Organization, c.c.Repository, &go_github.Commit{
		Message: go_github.String("field-not-required"),
		Tree:    tree,
		Parents: []*go_github.Commit{
			cherryParent,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	sha := commit.GetSHA()

	refName := fmt.Sprintf("%s%s", branchRefPrefix, branchName)
	_, _, err = c.Client.Git.UpdateRef(ctx, c.c.Organization, c.c.Repository, &go_github.Reference{
		Ref: go_github.String(refName),
		Object: &go_github.GitObject{
			SHA: go_github.String(sha),
		},
	}, true)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// createBranchFrom creates a new branch pointing at the same commit as the supplied branch.
func (c *Client) createBranchFrom(ctx context.Context, branchFromName string, newBranchName string) error {
	baseBranch, _, err := c.Client.Repositories.GetBranch(ctx, c.c.Organization, c.c.Repository, branchFromName, true)
	if err != nil {
		return trace.Wrap(err)
	}
	newRefBranchName := fmt.Sprintf("%s%s", branchRefPrefix, newBranchName)
	baseBranchSHA := baseBranch.GetCommit().GetSHA()

	ref := &go_github.Reference{
		Ref: go_github.String(newRefBranchName),
		Object: &go_github.GitObject{
			SHA: go_github.String(baseBranchSHA), /* SHA to branch from */
		},
	}
	_, _, err = c.Client.Git.CreateRef(ctx, c.c.Organization, c.c.Repository, ref)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// merge merges a branch at `headCommitSHA` into branch `base`
func (c *Client) merge(ctx context.Context, base string, headCommitSHA string) (*go_github.Commit, error) {
	merge, _, err := c.Client.Repositories.Merge(ctx, c.c.Organization, c.c.Repository, &go_github.RepositoryMergeRequest{
		Base: go_github.String(base),
		Head: go_github.String(headCommitSHA),
	})
	if err != nil {
		return nil, trace.Errorf("err: %v. failed to merge %s into %s", err, headCommitSHA, base)
	}
	mergeCommit, _, err := c.Client.Git.GetCommit(ctx,
		c.c.Organization,
		c.c.Repository,
		merge.GetSHA())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return mergeCommit, nil
}

const (
	// perPage is the number of items per page to request.
	perPage = 100

	// branchRefPrefix is the prefix for a reference that is
	// pointing to a branch.
	branchRefPrefix = "refs/heads/"
)
