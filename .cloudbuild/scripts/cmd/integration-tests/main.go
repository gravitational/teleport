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
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/artifacts"
	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/changes"
	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/etcd"
	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/git"
	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/secrets"
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

// innerMain parses the command line, performs the highlevel docs change check
// and creates the marker file if necessary
func innerMain() error {
	args, err := parseCommandLine()
	if err != nil {
		return trace.Wrap(err)
	}

	// If a github deploy key location was supplied...
	var deployKey []byte
	if args.githubKeySrc != "" {
		// fetch the deployment key from the GCB secret manager
		log.Infof("Fetching deploy key from %s", args.githubKeySrc)
		deployKey, err = secrets.Fetch(context.Background(), args.githubKeySrc)
		if err != nil {
			return trace.Wrap(err, "failed fetching deploy key")
		}
	}

	if !args.skipUnshallow {
		unshallowCtx, unshallowCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer unshallowCancel()
		err = git.UnshallowRepository(unshallowCtx, args.workspace, deployKey)
		if err != nil {
			return trace.Wrap(err, "unshallow failed")
		}
	}

	moduleCacheDir := filepath.Join(os.TempDir(), gomodcacheDir)
	gomodcache := fmt.Sprintf("GOMODCACHE=%s", moduleCacheDir)

	log.Println("Analyzing code changes")
	ch, err := changes.Analyze(args.workspace, args.targetBranch, args.commitSHA)
	if err != nil {
		return trace.Wrap(err, "Failed analyzing code")
	}

	if !ch.Code {
		log.Println("No code changes detected. Skipping tests.")
		return nil
	}

	// From this point on, whatever happens we want to upload any artifacts
	// produced by the build
	defer func() {
		prefix := fmt.Sprintf("%s/artifacts", args.buildID)
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		artifacts.FindAndUpload(timeoutCtx, args.bucket, prefix, args.artifactSearchPatterns)
	}()

	log.Printf("Running root-only integration tests...")
	err = runRootIntegrationTests(args.workspace, gomodcache)
	if err != nil {
		return trace.Wrap(err, "Root-only integration tests failed")
	}
	log.Println("Root-only integration tests passed.")

	if !args.skipChown {
		// We run some build steps as root and others as a non user, and we
		// want the nonroot user to be able to manipulate the artifacts
		// created by root, so we `chown -R` the whole workspace & module
		// cache to allow it.

		log.Printf("Reconfiguring workspace for nonroot user")
		err = chownR(args.workspace, nonrootUID, nonrootGID)
		if err != nil {
			return trace.Wrap(err, "failed reconfiguring workspace")
		}

		log.Printf("Reconfiguring module cache for nonroot user")
		err = chownR(moduleCacheDir, nonrootUID, nonrootGID)
		if err != nil {
			return trace.Wrap(err, "failed reconfiguring module cache")
		}
	}

	// Note that we run `etcd` as nonroot here. The files created by etcd live
	// inside the directory searched by `go list ./...` when generating the list
	// of packages to test, and so making them owned by root produces a heap of
	// diagnostic warnings that would pollute the build log and just confuse
	// people when they are trying to work out why their build failed.
	log.Printf("Starting etcd...")
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	etcdSvc, err := etcd.Start(timeoutCtx, args.workspace)
	if err != nil {
		return trace.Wrap(err, "failed starting etcd")
	}
	defer etcdSvc.Stop()

	log.Printf("Running nonroot integration tests...")
	err = runNonrootIntegrationTests(args.workspace, nonrootUID, nonrootGID, gomodcache)
	if err != nil {
		return trace.Wrap(err, "Nonroot integration tests failed")
	}

	log.Printf("Non-root integration tests passed.")

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
