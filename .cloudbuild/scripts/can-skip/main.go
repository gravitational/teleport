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

// Package main implements a script to test if a PR being tested contains only
// documentation changes. If so, it will create a signal file in a specified
// location, the existence of which subsequent build steps can use to determine
// if they should run or not.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// main is just a stub that prints out an error message and sets a nonzero exit
// code on failure.All of the work happens in `innerMain()`.
func main() {
	if err := innerMain(); err != nil {
		fmt.Printf("FAILED: %s\n", err.Error())
		os.Exit(-1)
	}
}

// innerMain parses the command line, performs the highlevel docs change check
// and creates the marker file if necessary
func innerMain() error {
	var workspace string
	var skipFile string
	var targetBranch string
	var commitSHA string

	flag.StringVar(&workspace, "w", "", "Fully-qualified path to the build workspace")
	flag.StringVar(&skipFile, "s", "", "File to be created if tests should be skipped")
	flag.StringVar(&targetBranch, "t", "", "The PR's target branch")
	flag.StringVar(&commitSHA, "c", "", "The PR's latest commit SHA")
	flag.Parse()

	if workspace == "" {
		return fmt.Errorf("workspace path must be set")
	}

	if skipFile == "" {
		return fmt.Errorf("skip file path must be set")
	}

	if targetBranch == "" {
		return fmt.Errorf("target branch must be set")
	}

	if commitSHA == "" {
		return fmt.Errorf("commit must be set")
	}

	fmt.Printf("Opening workspace %q as git repo\n", workspace)
	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return fmt.Errorf("failed opening workspace as a repo: %w", err)
	}

	changes, err := getChanges(repo, targetBranch, commitSHA)
	if err != nil {
		return fmt.Errorf("failed extracting changes: %w", err)
	}

	if hasOnlyDocChanges(changes) {
		fmt.Println("No non-docs changes detected. Creating skipfile.")
		err = touch(skipFile)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Non-docs changes detected. Tests will run as normal.")
	}

	return nil
}

// hasOnlyDocChanges examines a collection fo changes to check that there are
// only documentation related changes. Doc changes are assumed to be related
// to files with given prefices or extensions.
func hasOnlyDocChanges(changes object.Changes) bool {
	for _, change := range changes {
		path := changePath(change)
		if path == "" ||
			strings.HasPrefix(path, "docs/") ||
			strings.HasSuffix(path, ".mdx") ||
			strings.HasSuffix(path, ".md") {
			continue
		}

		return false
	}

	return true
}

// getChanges resolves the head of target branch and compares the trees at the
// the target branch and the supplied commit SHA.
func getChanges(repo *git.Repository, targetBranch, commit string) (object.Changes, error) {
	fmt.Printf("Getting worktree for target branch %s\n", targetBranch)
	targetTree, err := getBranchTree(repo, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("failed getting target branch worktree: %w", err)
	}

	fmt.Printf("Getting worktree for commit %s\n", commit)
	commitTree, err := getCommitTree(repo, commit)
	if err != nil {
		return nil, fmt.Errorf("failed getting target branch worktree: %w", err)
	}

	fmt.Printf("Comparing commit %s with target %s\n", commit, targetBranch)
	changes, err := targetTree.Diff(commitTree)
	if err != nil {
		return nil, fmt.Errorf("failed diffing target branch and latest commit: %w", err)
	}

	fmt.Printf("There are %d changes:\n", changes.Len())
	return changes, nil
}

func getBranchTree(repo *git.Repository, name string) (*object.Tree, error) {
	refname := plumbing.NewRemoteReferenceName("origin", name)
	fmt.Printf("Treating target branch as: %s\n", refname)
	sha, err := repo.ResolveRevision(plumbing.Revision(refname))
	if err != nil {
		return nil, fmt.Errorf("failed resolving branch %q: %w", name, err)
	}

	fmt.Printf("Branch %s resolves to %s\n", name, sha)

	return getTreeForSHA(repo, *sha)
}

func getCommitTree(repo *git.Repository, ref string) (*object.Tree, error) {
	sha, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("failed resolving commit %q: %w", ref, err)
	}

	fmt.Printf("Ref %s resolves to %s\n", ref, sha)

	return getTreeForSHA(repo, *sha)
}

func getTreeForSHA(repo *git.Repository, sha plumbing.Hash) (*object.Tree, error) {
	commit, err := repo.CommitObject(sha)
	if err != nil {
		return nil, fmt.Errorf("failed extracting commit %q: %w", sha, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed getting commit %q worktree: %w", sha, err)
	}

	return tree, nil
}

func touch(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed touching file %s: %w", filename, err)
	}
	f.Close()
	return nil
}

func changePath(c *object.Change) string {
	if c.From.Name != "" {
		return c.From.Name
	}

	return c.To.Name
}
