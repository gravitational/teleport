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
	"errors"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Builds an runs a command with the provided arguments. Extensively logs command
// details to the debug log. Returns stdout and stderr combined, along with an
// error iff one occurred.
func BuildAndRunCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	logrus.Debugf("Running command \"%s '%s'\"", command, strings.Join(args, "' '"))
	output, err := cmd.CombinedOutput()

	if output != nil {
		logrus.Debugf("Command output: %s", string(output))
	}

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode := exitError.ExitCode()
			logrus.Debugf("Command exited with exit code %d", exitCode)
		} else {
			logrus.Debugln("Command failed without an exit code")
		}
		return "", trace.Wrap(err, "Command failed, see debug output for additional details")
	}

	logrus.Debugln("Command exited successfully")
	return string(output), nil
}
