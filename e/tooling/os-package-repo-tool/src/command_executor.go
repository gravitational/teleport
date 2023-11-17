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
