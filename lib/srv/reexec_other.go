// +build !linux

package srv

import (
	"os/exec"
)

func reexecCommandOSTweaks(cmd *exec.Cmd) {}

func userCommandOSTweaks(cmd *exec.Cmd) {}
