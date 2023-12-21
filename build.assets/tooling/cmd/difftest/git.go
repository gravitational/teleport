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

package main

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

const (
	// gitFileNotExistsError is error message returned from git when the file
	// does not exists in this revision
	gitFileNotExistsError = "exists on disk, but not in"
)

// gitIsAvailable returns status of git
func gitIsAvailable() error {
	_, err := exec.LookPath("git")
	return err
}

// git runs git and returns output (stdout/stderr, depends on the cmd result) and error
func git(dir string, args ...string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir

	err := cmd.Run()

	if err != nil {
		return strings.TrimSpace(stderr.String()), trace.Wrap(err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// gitLatestCommit returns sha of the latest commit
//
// Runs: git log -1 --format=%H
func gitLatestCommitSha(path string) (string, error) {
	sha, err := git(path, "log", "--pretty=oneline", "-1", "--format=%H")
	if err != nil {
		return sha, trace.Errorf("%w : Error returned by `git log -1 --format=%%H", err)
	}

	return sha, nil
}

// gitMergeBase returns git ref of fork point
//
// Runs: git merge-base --fork-point <branch>
func gitMergeBase(path string, branch string) (string, error) {
	sha, err := gitLatestCommitSha(path)
	if err != nil {
		return sha, trace.Wrap(err)
	}

	ref, err := git(path, "merge-base", branch, sha)
	if err != nil {
		return ref, trace.Errorf("%w : Error returned by `git merge-base %s %s`: %s", err, branch, sha, ref)
	}

	return ref, nil
}

// gitChanges returns git diff with a ref of fork point with base branch
//
// Runs: git diff $(git merge-base --fork-point <branch>)
func gitChanges(path string, ref string) (string, error) {
	diff, err := git(path, "diff", ref, "--", path)
	if err != nil {
		return diff, trace.Errorf("%w : Error returned by `git diff %s`: %s", err, ref, diff)
	}

	return diff, nil
}

// gitGetFileFromRevision returns file contents from revision
//
// Runs: git show -b <ref>:<filename>
func gitGetFileFromRevision(path string, filename string, ref string) (string, error) {
	// -b means skip spaces
	content, err := git(path, "show", "-b", ref+":"+filename)
	if err != nil {
		// file does not exists in revision
		if strings.Contains(content, gitFileNotExistsError) {
			return "", nil
		}
		return content, trace.Errorf("%w : Error returned by `git show %s:%s : %s", err, ref, filename, content)
	}

	return content, nil
}
