/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

const (
	// git error when file does not exists in revision
	gitFileNotExistsError = "exists on disk, but not in"
)

// gitIsAvailable returns status of git
func gitIsAvailable() error {
	_, err := exec.LookPath("git")
	return err
}

// git runs git and returns output (stdout/stderr, depends on the cmd result) and error
func git(path string, args ...string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = path

	err := cmd.Run()

	if err != nil {
		return strings.TrimSpace(stderr.String()), trace.Wrap(err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// gitMergeBase returns git ref of fork point
//
// Runs: git merge-base --fork-point <branch>
func gitMergeBase(path string, branch string) (string, error) {
	ref, err := git(path, "merge-base", "--fork-point", branch)
	if err != nil {
		return ref, trace.NewAggregate(
			err,
			trace.Errorf("Error returned by `git merge-base --fork-point %s`: %s", branch, ref),
		)
	}

	return ref, nil
}

// gitChanges returns git diff with a ref of fork point with base branch
//
// Runs: git diff $(git merge-base --fork-point <branch>)
func gitChanges(path string, ref string) (string, error) {
	diff, err := git(path, "diff", ref, "--", path)
	if err != nil {
		return diff, trace.NewAggregate(err, trace.Errorf("Error returned by `git diff %s`: %s", ref, diff))
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
		return content, trace.NewAggregate(err, trace.Errorf("Error returned by `git show %s:%s : %s", ref, filename, content))
	}

	return content, nil
}
