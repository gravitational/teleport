package common

import (
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

func UpdateCompletionsInBackground() error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command(executable, "update-completions")
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(cmd.Process.Release())
}
