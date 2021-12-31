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
	"io/fs"
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
		return args, trace.Wrap(err, "Unable to resole absolute path to workspace")
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

	gomodcache := fmt.Sprintf("GOMODCACHE=%s", path.Join(args.workspace, gomodcacheDir))

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

	log.Printf("Running root-only integration tests...")
	err = runRootIntegrationTests(args.workspace, gomodcache)
	if err != nil {
		return trace.Wrap(err, "Root-only integration tests failed")
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
	err = etcd.Start(cancelCtx, args.workspace, nonrootUID, nonrootGID, gomodcache)
	if err != nil {
		return trace.Wrap(err, "failed starting etcd")
	}

	log.Printf("Running nonroot integration tests...")
	err = runNonrootIntegrationTests(args.workspace, nonrootUID, nonrootGID, gomodcache)
	if err != nil {
		return trace.Wrap(err, "Nonroot integration tests failed")
	}

	log.Printf("PASS")

	return nil
}

func runRootIntegrationTests(workspace string, env ...string) error {
	// Run root integration tests
	cmd := exec.Command("make", "rdpclient", "integration-root")
	cmd.Dir = workspace
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func runNonrootIntegrationTests(workspace string, uid, gid int, env ...string) error {
	cmd := exec.Command("make", "integration")
	cmd.Dir = workspace
	cmd.Env = append(append(os.Environ(), "TELEPORT_ETCD_TEST=yes"), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// make the command run under the supplied nonroot account
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	return cmd.Run()
}

// chownR changes the owner of each file in the workspace to the supplied
// uid:guid combo.
func chownR(workspace string, uid, gid int) error {
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		return os.Chown(path, uid, gid)
	})

	return trace.Wrap(err, "Failed changing file owner")
}
