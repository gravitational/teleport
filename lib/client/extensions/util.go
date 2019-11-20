/*
Copyright 2019 Gravitational, Inc.

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

package extensions

import (
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

// ensureSymlinks recreates the provided symlinks.
func ensureSymlinks(symlinks map[string]string) error {
	for source, destination := range symlinks {
		if err := ensureSymlink(source, destination); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// verifySymlinks verifies a set of symlinks.
func verifySymlinks(symlinks map[string]string) (bool, error) {
	for source, destination := range symlinks {
		if ok, err := verifySymlink(source, destination); !ok || err != nil {
			return false, trace.Wrap(err)
		}
	}
	return true, nil
}

// ensureSymlinks recreates the specified symlink.
func ensureSymlink(source, destination string) error {
	log.Debugf("Symlinking %v -> %v.", source, destination)
	if err := os.RemoveAll(destination); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := os.Symlink(source, destination); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// verifySymlink verifies that the symlink exists and points to the expected location.
func verifySymlink(source, destination string) (bool, error) {
	fileInfo, err := os.Lstat(destination)
	err = trace.ConvertSystemError(err)
	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return false, nil
	}
	// If the file exists, it must be a symlink and point to the specified location.
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	actualSource, err := os.Readlink(destination)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	if actualSource != source {
		return false, nil
	}
	return true, nil
}

// runCommand executes the specified command.
func runCommand(cmd string, args ...string) error {
	command := exec.Command(cmd, args...)
	log.Debugf("Executing %v.", command.Args)
	if out, err := command.CombinedOutput(); err != nil {
		return trace.Wrap(err, "failed to execute %v: %v", command.Args, string(out))
	}
	return nil
}

// hasDocker returns true if docker binary is available.
func hasDocker() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// hasHelm returns true if helm binary is available.
func hasHelm() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}
