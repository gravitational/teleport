// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/session/envutils"
	"github.com/gravitational/teleport/session/networking/x11"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
)

// FileFD is a file descriptor passed down from a parent process when
// Teleport is re-executing itself.
type FileFD = uintptr

// FileFDs used by all re-exec subcommands.
const (
	// CommandFile is used to pass the command and arguments that the
	// child process should execute from the parent process.
	CommandFile FileFD = 3 + iota
	// LogFile is used to emit logs from the child process to the parent
	// process.
	LogFile
	// ContinueFile is used to communicate to the child process that
	// it can continue after the parent process starts monitoring the
	// child's audit login session ID when Enhanced Session Recording
	// is enabled. Otherwise it isn't used.
	ContinueFile
	// ReadyFile is used to communicate to the parent process that the
	// child has changed its auid and is ready to be monitored when
	// Enhanced Session Recording is enabled. Otherwise it isn't used.
	ReadyFile
	// TerminateFile is used to communicate to the child process that
	// the interactive terminal should be killed as the client ended the
	// SSH session and without termination the terminal process will be assigned
	// to pid 1 and "live forever". Killing the shell should not prevent processes
	// preventing SIGHUP to be reassigned (ex. processes running with nohup).
	TerminateFile
	// FirstExtraFile is the first file descriptor that will be valid when
	// extra files are passed to child processes without a terminal.
	FirstExtraFile
)

// FileFDs for terminal based exec sessions.
const (
	// TTYFile is a TTY the parent process passes to the child process.
	TTYFile = FirstExtraFile + iota
)

// FileFDs for non-terminal based exec sessions.
const (
	// StdinFile is used to capture the stdin stream of the shell (grandchild) process.
	StdinFile = FirstExtraFile + iota
	// StdoutFile is used to capture the stdout stream of the shell (grandchild) process.
	StdoutFile
	// StderrFile is used to capture the stderr stream of the shell (grandchild) process.
	StderrFile
)

// FileFDs for SFTP sessions.
const (
	// FileTransferOutFile is used to pass write transfer data to the sftp (grandchild) process.
	FileTransferOutFile = FirstExtraFile + iota
	// FileTransferInFile is used to pass read transfer data from the sftp (grandchild) process.
	FileTransferInFile
	// AuditInFile is used to read audit events from the sftp (grandchild) process.
	AuditInFile
)

// FileFDs for networking sessions.
const (
	// ListenerFile is a unix datagram socket listener.
	ListenerFile = FirstExtraFile + iota
)

// CommandExecutor is wrapper around *exec.Cmd that handles creating and closing pipes
// used to communicate with child process when reexecuting teleport
type CommandExecutor struct {
	*exec.Cmd

	ctx context.Context

	// cont is used to send the continue signal from the parent process
	// to the child process.
	cont *os.File

	// ready is used to send the ready signal from the child process
	// to the parent process. If ESR is enabled, the child signals after
	// the audit session login ID (auid) is received.
	ready *os.File

	// killShell is used to send kill signal to the child process
	// to terminate the shell.
	killShell *os.File

	childFiles  []*os.File
	parentFiles []io.Closer

	bpfEnabled bool
	logger     *slog.Logger
}

func (e *CommandExecutor) childToParentPipe(fd FileFD) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if e.childFiles, err = addFile(e.childFiles, w, fd); err != nil {
		r.Close()
		w.Close()
		return nil, trace.Wrap(err)
	}
	e.parentFiles = append(e.parentFiles, r)
	return r, nil
}

func (e *CommandExecutor) parentToChildPipe(fd FileFD) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if e.childFiles, err = addFile(e.childFiles, r, fd); err != nil {
		r.Close()
		w.Close()
		return nil, trace.Wrap(err)
	}
	e.parentFiles = append(e.parentFiles, w)
	return w, nil
}

func addFile(slice []*os.File, file *os.File, fd FileFD) ([]*os.File, error) {
	idx := int(fd)
	if idx >= len(slice) {
		slice = slices.Grow(slice, idx+1-len(slice))
		clear(slice[len(slice) : idx+1])
		slice = slice[:idx+1]
	}
	if slice[idx] != nil {
		return nil, trace.BadParameter("file already exists")
	}
	slice[idx] = file
	return slice, nil
}

func (e *CommandExecutor) Close() error {
	var errs []error
	for _, closer := range e.parentFiles {
		if err := closer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
	}
	for _, closer := range e.childFiles {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

func (e *CommandExecutor) Start() error {
	if err := e.Cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	for i, file := range e.childFiles {
		if file == nil {
			continue
		}
		if err := file.Close(); err != nil {
			e.logger.WarnContext(e.ctx, "Failed to close child fd", "error", err, "fd", i)
		}
	}
	e.childFiles = nil
	return nil
}

// The child does not signal until it completes PAM setup, which can take an arbitrary
// amount of time, so we use a reasonably long timeout to avoid dubious lockouts.
const childReadyWaitTimeout = 3 * time.Minute

func (e *CommandExecutor) WaitForChild() error {
	if e.ready == nil {
		return nil
	}
	var waitErr error
	if e.bpfEnabled {
		if waitErr = WaitForSignal(e.ctx, e.ready, childReadyWaitTimeout); waitErr != nil {
			e.logger.ErrorContext(e.ctx, "Child process never became ready.", "error", waitErr)
		}
	}

	closeErr := e.ready.Close()
	e.ready = nil

	return trace.NewAggregate(waitErr, closeErr)
}

// Continue will resume execution of the process after it completes its
// pre-processing routine if Enhanced Session Recording is enabled.
// Otherwise, this method is a no-op.
func (e *CommandExecutor) Continue() error {
	if e.cont == nil {
		return nil
	}
	err := e.cont.Close()
	e.cont = nil
	return trace.Wrap(err)
}

// Kill will send signal to the child process that it should terminate the command
func (e *CommandExecutor) Kill() error {
	if e.killShell == nil {
		return nil
	}
	err := e.killShell.Close()
	e.killShell = nil
	return trace.Wrap(err)
}

// ConfigureCommand creates a command fully configured to execute. This
// function is used by Teleport to re-execute itself and pass whatever data
// is need to the child to actually execute the shell.
// Context passed to this function is used only for logging and waiting for
// the ready signal from child, the returned command will not be terminated
// when it's done
func ConfigureCommand(ctx context.Context, logger *slog.Logger, childLogWriter io.Writer, command *ExecCommand, execType string, extraFiles map[FileFD]*os.File) (_ *CommandExecutor, err error) {
	executor := &CommandExecutor{
		ctx:    ctx,
		logger: logger,
	}
	defer func() {
		if err != nil {
			if closeErr := executor.Close(); closeErr != nil {
				err = trace.NewAggregate(err, closeErr)
			}
			executor = nil
		}
	}()

	logFileWriter, canReuseLogWriter := childLogWriter.(*os.File)
	if !canReuseLogWriter {
		// Create a pipe so we can pass the writing side as an *os.File to the child process.
		// Then we can copy from the reading side to the log writer (e.g. syslog, log file w/ concurrency protection).
		r, err := executor.childToParentPipe(LogFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Copy logs from the child process to the parent process over
		// the pipe until it is closed by the child context.
		go func() {
			if _, err := io.Copy(childLogWriter, r); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				slog.ErrorContext(ctx, "Failed to copy logs over pipe", "error", err)
			}
		}()
	}
	cmd, err := executor.parentToChildPipe(CommandFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if executor.cont, err = executor.parentToChildPipe(ContinueFile); err != nil {
		return nil, trace.Wrap(err)
	}
	if executor.killShell, err = executor.parentToChildPipe(TerminateFile); err != nil {
		return nil, trace.Wrap(err)
	}
	if executor.ready, err = executor.childToParentPipe(ReadyFile); err != nil {
		return nil, trace.Wrap(err)
	}

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
	buffer := &bytes.Buffer{}
	if err := json.NewEncoder(buffer).Encode(command); err != nil {
		return nil, trace.Wrap(err)
	}
	go copyCommand(ctx, cmd, buffer)

	// Find the Teleport executable and its directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build env for `teleport exec`.
	env := &envutils.SafeEnv{}
	env.AddExecEnvironment()

	// The channel/request type determines the subcommand to execute.
	var subCommand string
	switch execType {
	case reexecconstants.NetworkingSubCommand:
		subCommand = reexecconstants.NetworkingSubCommand

		// Unset XAUTHORITY for the networking command as the SSH session
		// process given to the user will not have it set which can cause
		// issues with the X11 forwarding.
		env.Remove(x11.XAuthFileEnvVar)
	default:
		subCommand = reexecconstants.ExecSubCommand
	}

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable, subCommand}

	executor.bpfEnabled = command.RecordWithBPF

	childFiles := slices.Clone(executor.childFiles)

	if canReuseLogWriter {
		childFiles, err = addFile(childFiles, logFileWriter, LogFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	for fd, file := range extraFiles {
		childFiles, err = addFile(childFiles, file, fd)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Build the "teleport exec" command.
	executor.Cmd = &exec.Cmd{
		Path:       executable,
		Args:       args,
		Env:        *env,
		ExtraFiles: childFiles[3:],
	}

	if childFiles[0] != nil {
		executor.Cmd.Stdin = childFiles[0]
	}
	if childFiles[1] != nil {
		executor.Cmd.Stdout = childFiles[1]
	}
	if childFiles[2] != nil {
		executor.Cmd.Stderr = childFiles[2]
	}

	// Perform OS-specific tweaks to the command.
	CommandOSTweaks(executor.Cmd)

	return executor, nil
}

// copyCommand will copy the provided command to the child process over the
// pipe attached to the context.
func copyCommand(ctx context.Context, cmdw *os.File, buffer *bytes.Buffer) {
	// Write command bytes to pipe. The child process will read the command
	// to execute from this pipe.
	if _, err := io.Copy(cmdw, buffer); err != nil {
		slog.ErrorContext(ctx, "Failed to copy command over pipe", "error", err)
	}

	if err := cmdw.Close(); err != nil {
		slog.ErrorContext(ctx, "Failed to close command pipe", "error", err)
	}
}

// WaitForSignal will wait for the other side of the pipe to signal, if not
// received, it will stop waiting and exit.
func WaitForSignal(ctx context.Context, fd *os.File, timeout time.Duration) error {
	waitCh := make(chan error, 1)
	go func() {
		// Reading from the file descriptor will block until it's closed.
		_, err := fd.Read(make([]byte, 1))
		if errors.Is(err, io.EOF) {
			err = nil
		}
		waitCh <- err
	}()

	// Timeout if no signal has been sent within the provided duration.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "got context error while waiting for continue signal")
	case <-timer.C:
		return trace.LimitExceeded("timed out waiting for continue signal")
	case err := <-waitCh:
		return trace.Wrap(err)
	}
}
