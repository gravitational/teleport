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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"

	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/changes"
	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/etcd"
	"github.com/gravitational/trace"
)

const (
	gomodcacheDir = ".gomodcache-ci"
	nonrootUID    = 1000
	nonrootGID    = 1000
)

// main is just a stub that prints out an error message and sets a nonzero exit
// code on failure. All of the work happens in `innerMain()`.
func main() {
	if err := innerMain(); err != nil {
		log.Fatalf("FAILED: %s", err.Error())
	}
}

type commandlineArgs struct {
	workspace    string
	targetBranch string
	commitSHA    string
	skipChown    bool
}

func parseCommandLine() (commandlineArgs, error) {
	args := commandlineArgs{}

	flag.StringVar(&args.workspace, "w", "", "Fully-qualified path to the build workspace")
	flag.StringVar(&args.targetBranch, "t", "", "The PR's target branch")
	flag.StringVar(&args.commitSHA, "c", "", "The PR's latest commit SHA")
	flag.BoolVar(&args.skipChown, "skip-chown", false, "Skip reconfiguring the workspace for a nonroot user.")

	flag.Parse()

	if args.workspace == "" {
		return args, trace.Errorf("workspace path must be set")
	}

	var err error
	args.workspace, err = filepath.Abs(args.workspace)
	if err != nil {
		return args, trace.Wrap(err, "Unable to resolve absolute path to workspace")
	}

	if args.targetBranch == "" {
		return args, trace.Errorf("target branch must be set")
	}

	if args.commitSHA == "" {
		return args, trace.Errorf("commit must be set")
	}

	return args, nil
}

// innerMain parses the command line, performs the highlevel docs change check
// and creates the marker file if necessary
func innerMain() error {
	args, err := parseCommandLine()
	if err != nil {
		return trace.Wrap(err)
	}

	log.Println("Analysing code changes")
	ch, err := changes.Analyze(args.workspace, args.targetBranch, args.commitSHA)
	if err != nil {
		return trace.Wrap(err, "Failed analyzing code")
	}

	hasOnlyDocChanges := ch.Docs && (!ch.Code)
	if hasOnlyDocChanges {
		log.Println("No non-docs changes detected. Skipping tests.")
		return nil
	}

	if !args.skipChown {
		// We run some build steps as root and others as a non user, and we
		// want the nonroot user to be able to manipulate the artifacts
		// created by root, so we `chown -R` the whole workspace to allow it.
		log.Printf("Reconfiguring workspace for nonroot user")
		err = chownR(args.workspace, nonrootUID, nonrootGID)
		if err != nil {
			return trace.Wrap(err, "failed reconfiguring workspace")
		}
	}

	log.Printf("Starting etcd...")
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = etcd.Start(cancelCtx, args.workspace, nonrootUID, nonrootUID)
	if err != nil {
		return trace.Wrap(err, "failed starting etcd")
	}

	log.Printf("Running unit tests...")
	err = runUnitTests(args.workspace)
	if err != nil {
		return trace.Wrap(err, "unit tests failed")
	}

	log.Printf("PASS")

	return nil
}

func runUnitTests(workspace string) error {
	cmd := exec.Command("make", "test")
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = append(os.Environ(),
		"TELEPORT_ETCD_TEST=yes",

		// We want the go module cache to be in a place we control, so force it
		// to be in the workspace
		fmt.Sprintf("GOMODCACHE=%s", path.Join(workspace, gomodcacheDir)),

		// Some unit tests are not well isolated and will use the current $HOME
		// to store tsh profile data. GCB protects the real builder homedir, so
		// we tell the tests to use somewhere else that we know we can write to.
		"HOME=/tmp/.home")

	// make the command run under a nonroot account
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: nonrootUID,
			Gid: nonrootGID,
		},
	}

	return cmd.Run()
}

// chownR changes the owner of each file in the workspace to the supplied
// uid:guid combo.
func chownR(workspace string, uid, gid int) error {
	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return os.Chown(path, uid, gid)
	})

	return trace.Wrap(err, "Failed changing file owner")
}
