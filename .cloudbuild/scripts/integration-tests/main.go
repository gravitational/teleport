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
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gravitational/teleport/.cloudbuild/scripts/changes"
	"github.com/gravitational/trace"
)

const (
	gomodcache = ".gomodcache-ci"
	nonrootUID = 1000
	nonrootGID = 1000
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
	err = runRootIntegrationTests(args.workspace)
	if err != nil {
		return trace.Wrap(err, "Root-only integration tests failed")
	}

	if !args.skipChown {
		log.Printf("Reconfiguring workspace for nonroot user")
		err = reconfigureForNonrootUser(args.workspace, nonrootUID, nonrootGID)
		if err != nil {
			return trace.Wrap(err, "failed reconfiguring workspace")
		}
	}

	log.Printf("Starting etcd...")
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = startEtcd(cancelCtx, args.workspace, nonrootUID, nonrootGID)
	if err != nil {
		return trace.Wrap(err, "failed etcd")
	}

	log.Printf("Running nonroot integration tests...")
	err = runNonrootIntegrationTests(args.workspace, nonrootUID, nonrootGID)
	if err != nil {
		return trace.Wrap(err, "Nonroot integration tests failed")
	}

	log.Printf("PASS")

	return nil
}

func runRootIntegrationTests(workspace string) error {
	gomodcache := fmt.Sprintf("GOMODCACHE=%s", path.Join(workspace, gomodcache))
	log.Printf("Using %s", gomodcache)

	// Run root integration tests
	cmd := exec.Command("make", "rdpclient", "integration-root")
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), gomodcache)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func runNonrootIntegrationTests(workspace string, uid, gid int) error {
	gomodcache := fmt.Sprintf("GOMODCACHE=%s", path.Join(workspace, gomodcache))

	cmd := exec.Command("make", "integration")
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), gomodcache, "TELEPORT_ETCD_TEST=yes")
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

func startEtcd(ctx context.Context, workspace string, uid, gid int) error {
	cmd := exec.CommandContext(ctx, "make", "run-etcd")
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// make etcd run under the supplied nonroot account
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	log.Printf("Launching etcd")
	go cmd.Run()

	log.Printf("Waiting for etcd to start...")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ticker.C:
			log.Printf(".")
			d := net.Dialer{Timeout: 100 * time.Millisecond}
			_, err := d.Dial("tcp", "127.0.0.1:2379")
			if err == nil {
				log.Printf("Etcd is up")
				return nil
			}

		case <-timeoutCtx.Done():
			return trace.Errorf("Timed out waiting for etcd to start")
		}
	}

	return nil
}

func reconfigureForNonrootUser(workspace string, uid, gid int) error {
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		return os.Chown(path, uid, gid)
	})

	return trace.Wrap(err, "Failed changing file owner")
}
