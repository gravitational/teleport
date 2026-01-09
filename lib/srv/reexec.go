package srv

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/srv/reexec"
	"github.com/gravitational/teleport/lib/utils/envutils"
)

type PAMConfig = reexec.PAMConfig
type ExecCommand = reexec.ExecCommand
type UaccMetadata = reexec.UaccMetadata
type ExecLogConfig = reexec.ExecLogConfig

// IsReexec determines if the current process is a teleport reexec command.
// Used by tests to reroute the execution to RunAndExit.
func IsReexec() bool {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case teleport.ExecSubCommand, teleport.NetworkingSubCommand,
			teleport.CheckHomeDirSubCommand,
			teleport.ParkSubCommand, teleport.SFTPSubCommand:
			return true
		}
	}

	return false
}

func RunAndExit(commandType string) {
	reexec.RunAndExit(commandType)
}

func CheckHomeDir(localUser *user.User) (bool, error) {
	return reexec.CheckHomeDir(localUser)
}

func waitForSignal(ctx context.Context, fd *os.File, timeout time.Duration) error {
	return reexec.WaitForSignal(ctx, fd, timeout)
}

// ConfigureCommand creates a command fully configured to execute. This
// function is used by Teleport to re-execute itself and pass whatever data
// is need to the child to actually execute the shell.
func ConfigureCommand(ctx *ServerContext, extraFiles ...*os.File) (*exec.Cmd, error) {
	// Create a os.Pipe and start copying over the payload to execute. While the
	// pipe buffer is quite large (64k) some users have run into the pipe
	// blocking writes on much smaller buffers (7k) leading to Teleport being
	// unable to run some exec commands.
	//
	// To not depend on the OS implementation of a pipe, instead the copy should
	// be non-blocking. The io.Copy will be closed when either when the child
	// process has fully read in the payload or the process exits with an error
	// (and closes all child file descriptors).
	//
	// See the below for details.
	//
	//   https://man7.org/linux/man-pages/man7/pipe.7.html
	cmdmsg, err := ctx.ExecCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cmdmsg.Terminal {
		cmdmsg.ExtraFilesLen = len(extraFiles)
	}

	go copyCommand(ctx, cmdmsg)

	// Find the Teleport executable and its directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The channel/request type determines the subcommand to execute.
	var subCommand string
	switch ctx.ExecType {
	case teleport.NetworkingSubCommand:
		subCommand = teleport.NetworkingSubCommand
	default:
		subCommand = teleport.ExecSubCommand
	}

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable, subCommand}

	// build env for `teleport exec`
	env := &envutils.SafeEnv{}
	env.AddExecEnvironment()

	// Build the "teleport exec" command.
	cmd := &exec.Cmd{
		Path: executable,
		Args: args,
		Env:  *env,
		ExtraFiles: []*os.File{
			ctx.cmdr,
			ctx.logw,
			ctx.contr,
			ctx.readyw,
			ctx.killShellr,
		},
	}
	// Add extra files if applicable.
	if len(extraFiles) > 0 {
		cmd.ExtraFiles = append(cmd.ExtraFiles, extraFiles...)
	}

	// Perform OS-specific tweaks to the command.
	reexec.CommandOSTweaks(cmd)

	return cmd, nil
}

// copyCommand will copy the provided command to the child process over the
// pipe attached to the context.
func copyCommand(ctx *ServerContext, cmdmsg *reexec.ExecCommand) {
	defer func() {
		err := ctx.cmdw.Close()
		if err != nil {
			slog.ErrorContext(ctx.CancelContext(), "Failed to close command pipe", "error", err)
		}

		// Set to nil so the close in the context doesn't attempt to re-close.
		ctx.cmdw = nil
	}()

	// Write command bytes to pipe. The child process will read the command
	// to execute from this pipe.
	if err := json.NewEncoder(ctx.cmdw).Encode(cmdmsg); err != nil {
		slog.ErrorContext(ctx.CancelContext(), "Failed to copy command over pipe", "error", err)
		return
	}
}
