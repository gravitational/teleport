/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/build.assets/tooling/lib/git"
)

var (
	baseBranch = kingpin.Flag(
		"base-branch",
		"The base release branch to generate the changelog for.  It will be of the form branch/v*",
	).Envar("BASE_BRANCH").String()

	baseTag = kingpin.Flag(
		"base-tag",
		"The tag/version to generate the changelog from. It will be of the form vXX.Y.Z, e.g. v15.1.1",
	).Envar("BASE_TAG").String()
)

func main() {
	kingpin.Parse()

	if err := prereqCheck(); err != nil {
		log.Fatal(err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal(trace.Wrap(err, "failed to get working directory"))
	}

	topDir, err := git.RunCmd(workDir, "rev-parse", "--show-toplevel")
	if err != nil {
		log.Fatal(err)
	}
	entDir := filepath.Join(topDir, "e")

	// Figure out the branch and last version released for that branch
	branch, err := getBranch(*baseBranch, workDir)
	if err != nil {
		log.Fatal(err)
	}

	lastVersion, err := getLastVersion(*baseTag, workDir)
	if err != nil {
		log.Fatal(trace.Wrap(err, "failed to determine last version"))
	}

	// Determine timestamps of releases which is used to limit Github search
	timeLastRelease, timeLastEntRelease, timeLastEntMod, err := getTimestamps(topDir, entDir, lastVersion)
	if err != nil {
		log.Fatal(err)
	}

	// Generate changelogs
	ossCLGen := &changelogGenerator{
		isEnt: false,
		dir:   topDir,
	}
	entCLGen := &changelogGenerator{
		isEnt: true,
		dir:   entDir,
	}
	ossCL, err := ossCLGen.generateChangelog(branch, timeLastRelease, timeNow)
	if err != nil {
		log.Fatal(err)
	}
	entCL, err := entCLGen.generateChangelog(branch, timeLastEntRelease, timeLastEntMod)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(ossCL)
	if entCL != "" {
		fmt.Println("Enterprise:")
		fmt.Println(entCL)
	}
}

func prereqCheck() error {
	if err := git.IsAvailable(); err != nil {
		return trace.Wrap(err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getBranch will return branch if parsed otherwise will attempt to find it
// Branch should be in the format "branch/v*"
func getBranch(branch, dir string) (string, error) {
	if branch == "" { // flag and env var not set, attempt to find
		// get ref
		ref, err := git.RunCmd(dir, "symbolic-ref", "HEAD")
		if err != nil {
			return "", trace.Wrap(err, "not on a branch")
		}

		// remove prefix and ensure that branch is in expected format
		branch, _ = strings.CutPrefix(ref, "refs/heads/")
		if branch == ref {
			return "", trace.Errorf("not on a branch: %s", ref)
		}

		// if the branch is not in the branch/v* format then check it's root
		if !strings.HasPrefix(branch, "branch/v") {
			fbranch, err := getForkedBranch(dir)
			if err != nil {
				return "", trace.Wrap(err, "could not determine a root branch")
			}
			branch = fbranch
		}
	}

	if !strings.HasPrefix(branch, "branch/v") {
		return "", trace.Errorf("not on a release branch, expected 'branch/v*', got %s", branch)
	}

	return branch, nil
}

// getForkedBranch will attempt to find a root branch for the current one that is in the format branch/v*
func getForkedBranch(dir string) (string, error) {
	forkPointRef, err := git.RunCmd(dir, "merge-base", "--fork-point", "HEAD")
	if err != nil {
		return "", trace.Wrap(err)
	}
	fbranch, err := git.RunCmd(dir, "branch", "--list", "branch/v*", "--contains", forkPointRef, "--format", "%(refname:short)")
	if err != nil {
		return "", trace.Wrap(err)
	}
	if fbranch == "" { // stdout is empty indicating the search failed
		return "", trace.Errorf("could not find a valid root branch")
	}
	return fbranch, nil
}

func getLastVersion(baseTag, dir string) (string, error) {
	if baseTag != "" {
		return baseTag, nil
	}

	// get root dir of repo
	topDir, err := git.RunCmd(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", trace.Wrap(err)
	}
	lastVersion, err := makePrintVersion(topDir)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return lastVersion, nil
}

// makePrintVersion will run 'make -s print-version'
func makePrintVersion(dir string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("make", "-s", "print-version")
	cmd.Dir = dir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()

	if err != nil {
		return strings.TrimSpace(stderr.String()), trace.Wrap(err, "can't get last released version")
	}
	out := strings.TrimSpace(stdout.String())

	return "v" + out, nil
}

func getTimestamps(dir string, entDir string, lastVersion string) (lastRelease, lastEnterpriseRelease, lastEnterpriseModify string, err error) {
	// get timestamp since last release
	since, err := git.RunCmd(dir, "show", "-s", "--date=format:%Y-%m-%dT%H:%M:%S%z", "--format=%cd", lastVersion)
	if err != nil {
		return "", "", "", trace.Wrap(err, "can't get timestamp of last release")
	}
	// get timestamp of last enterprise release
	sinceEnt, err := git.RunCmd(dir, "-C", "e", "show", "-s", "--date=format:%Y-%m-%dT%H:%M:%S%z", "--format=%cd", lastVersion)
	if err != nil {
		return "", "", "", trace.Wrap(err, "can't get timestamp of last enterprise release")
	}
	// get timestamp of last commit of enterprise
	entTime, err := git.RunCmd(entDir, "log", "-n", "1", "--date=format:%Y-%m-%dT%H:%M:%S%z", "--format=%cd")
	if err != nil {
		return "", "", "", trace.Wrap(err, "can't get last modified time of e")
	}
	return since, sinceEnt, entTime, nil
}
