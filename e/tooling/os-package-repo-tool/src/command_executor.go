package main

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// Builds an runs a command with the provided arguments. Extensively logs command
// details to the debug log. Returns stdout and stderr combined, along with an
// error iff one occurred.
func BuildAndRunCommand(command string, args ...string) (string, error) {
	ctx := context.Background()
	cmd := exec.Command(command, args...)
	slog.DebugContext(ctx, "Running command", "command", command, "args", strings.Join(args, "' '"))
	output, err := cmd.CombinedOutput()

	if output != nil {
		slog.DebugContext(ctx, "Command complete with output", "output", string(output))
	}

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode := exitError.ExitCode()
			slog.DebugContext(ctx, "Command failed with an exit code", "exit_code", exitCode)
		} else {
			slog.DebugContext(ctx, "Command failed without an exit code")
		}
		return "", trace.Wrap(err, "Command failed, see debug output for additional details")
	}

	slog.DebugContext(ctx, "Command exited successfully")
	return string(output), nil
}
