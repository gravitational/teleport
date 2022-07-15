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

// Package changes implements a script to analyze the changes between
// a commit and a given branch. It is designed for use when comparing
// the tip of a PR against the merge target
package changes

import (
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Changes describes the kind of changes found in the analyzed workspace.
type Changes struct {
	Docs       bool
	Code       bool
	Enterprise bool
}

// Analyze examines the workspace for specific changes using its git history,
// and then collates and returns a report.
func Analyze(workspaceDir string, targetBranch string, commitSHA string) (Changes, error) {
	log.Printf("Opening workspace %q as git repo\n", workspaceDir)
	repo, err := git.PlainOpen(workspaceDir)
	if err != nil {
		return Changes{}, trace.Wrap(err, "failed opening workspace as a repo")
	}

	changes, err := getChanges(repo, targetBranch, commitSHA)
	if err != nil {
		return Changes{}, trace.Wrap(err, "failed extracting changes")
	}

	report := Changes{}

	for _, change := range changes {
		path := getChangePath(change)
		switch {
		case path == "":
			continue

		case path == "e":
			report.Enterprise = true

		case isDocChange(path):
			report.Docs = true

		default:
			report.Code = true
		}

		if report.Docs && report.Code && report.Enterprise {
			// There's no sense in exhaustively listing all the changes if
			// the answer won't change, so bail early.
			break
		}
	}

	return report, nil
}

func isDocChange(path string) bool {
	path = strings.ToLower(path)
	return strings.HasPrefix(path, "docs/") ||
		strings.HasSuffix(path, ".mdx") ||
		strings.HasSuffix(path, ".md") ||
		strings.HasPrefix(path, "rfd/")
}

// getChanges resolves the head of target branch and compares the trees at the
// the target branch and the supplied commit SHA.
func getChanges(repo *git.Repository, targetBranch, commit string) (object.Changes, error) {
	log.Printf("Getting worktree for target branch %s", targetBranch)
	targetTree, err := getBranchTree(repo, targetBranch)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting target branch worktree")
	}

	log.Printf("Getting filetree for commit %s", commit)
	commitTree, err := getCommitTree(repo, commit)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting target branch worktree")
	}

	log.Printf("Comparing commit %q with target branch %q", commit, targetBranch)
	changes, err := targetTree.Diff(commitTree)
	if err != nil {
		return nil, trace.Wrap(err, "failed diffing target branch and latest commit")
	}

	log.Printf("There are %d changes:", changes.Len())
	return changes, nil
}

func getBranchTree(repo *git.Repository, name string) (*object.Tree, error) {
	refname := plumbing.NewRemoteReferenceName("origin", name)
	log.Printf("Treating target branch as: %s", refname)
	sha, err := repo.ResolveRevision(plumbing.Revision(refname))
	if err != nil {
		return nil, trace.Wrap(err, "failed resolving branch %q", name)
	}

	log.Printf("Branch %s resolves to %s", name, sha)

	return getTreeForSHA(repo, *sha)
}

func getCommitTree(repo *git.Repository, ref string) (*object.Tree, error) {
	sha, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, trace.Wrap(err, "failed resolving commit %q", ref)
	}

	log.Printf("Ref %s resolves to %s", ref, sha)

	return getTreeForSHA(repo, *sha)
}

func getTreeForSHA(repo *git.Repository, sha plumbing.Hash) (*object.Tree, error) {
	commit, err := repo.CommitObject(sha)
	if err != nil {
		return nil, trace.Wrap(err, "failed extracting commit %q", sha)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, trace.Wrap(err, "failed getting commit %q worktree", sha)
	}

	return tree, nil
}

func getChangePath(c *object.Change) string {
	if c.From.Name != "" {
		return c.From.Name
	}

	return c.To.Name
}
