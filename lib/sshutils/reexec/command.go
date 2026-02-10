/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reexec

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/envutils"
)

// Command wraps an exec.Cmd with common reexec pipes and helpers to
// manage their lifecycle.
type Command struct {
	cfg    *Config
	logger *slog.Logger

	cmd *exec.Cmd
	// done is set when the command starts and closed the command completes.
	done    chan struct{}
	exitErr error

	// parent side of config pipe.
	cfgW io.WriteCloser
	// parent side of logger pipe.
	logR io.Reader
	// parent side of continue pipe.
	contW io.WriteCloser
	// parent side of ready pipe.
	rdyR io.Reader
	// parent side of terminate pipe.
	termW io.WriteCloser

	// childPipes are child-side pipes that will be passed to the child process as extra files.
	// This command is always responsible for closing after the command is started or closed.
	childPipes []*os.File
	// parentReadPipes are the parent-side of child-to-parent pipes. It is the responsibility of this
	// Command to close them when the command completes or is closed.
	parentReadPipes []io.Closer
}

// CommandOpt if a command option.
// TODO(Joerger): Any changes to the underlying exec.Cmd should be made internally, once more
// logic is migrated here.
type CommandOpt func(*exec.Cmd)

// NewCommand allocates a [Command] with the common reexec pipes.
// The caller must ensure the command is closed once it's no longer needed.
func NewCommand(cfg *Config, opts ...CommandOpt) (*Command, error) {
	if cfg == nil {
		return nil, trace.BadParameter("missing config")
	}

	// Set the log writer to /dev/null to prevent child logging from blocking.
	if cfg.LogWriter == nil {
		cfg.LogWriter = io.Discard
	}

	// Build the "teleport <reexec-command>" command.
	cmd, err := newTeleportReexecCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, opt := range opts {
		opt(cmd)
	}

	c := &Command{
		logger: slog.Default(),
		cmd:    cmd,
		cfg:    cfg,
		done:   make(chan struct{}),
	}

	// Prepare common pipes which should always appear as the first extra files.
	c.cfgW, err = c.AddParentToChildPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.logR, err = c.AddChildToParentPipe()
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}
	c.contW, err = c.AddParentToChildPipe()
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}
	c.rdyR, err = c.AddChildToParentPipe()
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}
	c.termW, err = c.AddParentToChildPipe()
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// newTeleportReexecCommand builds an exec.Cmd to re-exec teleport with the given subcommand.
func newTeleportReexecCommand(cfg *Config) (*exec.Cmd, error) {
	// Find the Teleport executable and its directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	env := &envutils.SafeEnv{}
	env.AddExecEnvironment()

	return &exec.Cmd{
		Path: executable,
		Args: []string{executable, cfg.ReexecCommand},
		Env:  *env,
	}, nil
}

// Start starts the underlying exec.Cmd and closes the child side of reexec pipes.
// The provided closerOnExit will be closed when the command exits.
func (c *Command) Start(ctx context.Context, closerOnExit ...io.Closer) error {
	if c.cmd == nil {
		return trace.BadParameter("missing exec command")
	}

	// Prepare the child pipes and ensure they are closed after the command starts.
	c.cmd.ExtraFiles = c.childPipes
	c.childPipes = nil
	defer closeAll(filesToClosers(c.cmd.ExtraFiles)...)

	// Start copying the reexec config payload over the pipe. While the
	// pipe buffer is quite large (64k) some users have run into the pipe
	// blocking writes on much smaller buffers (7k) leading to Teleport being
	// unable to run some exec commands.
	//
	// To not depend on the OS implementation of a pipe, instead the copy should
	// be non-blocking. The io.Copy will be closed either when the child
	// process has fully read in the payload or the process exits with an error
	// (and closes all child file descriptors).
	//
	// See the below for details.
	//
	//   https://man7.org/linux/man-pages/man7/pipe.7.html
	go func() {
		defer c.cfgW.Close()
		if err := json.NewEncoder(c.cfgW).Encode(c.cfg); err != nil {
			c.logger.ErrorContext(ctx, "Failed to copy config over pipe", "error", err)
		}
	}()

	go func() {
		if _, err := io.Copy(c.cfg.LogWriter, c.logR); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
			slog.ErrorContext(context.Background(), "Failed to copy logs over pipe", "error", err)
		}
	}()

	if err := c.cmd.Start(); err != nil {
		closeErr := c.Close()
		return trace.NewAggregate(err, closeErr)
	}

	go func() {
		c.exitErr = c.cmd.Wait()
		close(c.done)
		closeAll(closerOnExit...)
		c.Close()
	}()

	return nil
}

// WithStdio sets stdout and stderr and returns a pipe for stdin.
func (c *Command) WithStdio(stdout, stderr io.Writer) (io.WriteCloser, error) {
	c.cmd.Stderr = stderr
	c.cmd.Stdout = stdout

	inputWriter, err := c.cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return inputWriter, nil
}

// AddChildPipe appends the child-side of a pipe as an extra file to the command.
// This must be called before starting the command to have any effect.
func (c *Command) AddChildPipe(file *os.File) {
	c.childPipes = append(c.childPipes, file)
}

// AddChildToParentPipe creates a pipe for child-to-parent writes and returns the read side.
// This must be called before starting the command to have any effect.
func (c *Command) AddChildToParentPipe() (io.Reader, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.parentReadPipes = append(c.parentReadPipes, reader)
	c.AddChildPipe(writer)

	return reader, nil
}

// AddParentToChildPipe creates a pipe for parent-to-child writes and returns the write side.
// This must be called before starting the command to have any effect.
// The caller is responsible for closing the returned WriteCloser.
func (c *Command) AddParentToChildPipe() (io.WriteCloser, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.AddChildPipe(reader)

	return writer, nil
}

// WaitReady waits for the child to signal when initial setup is complete.
func (c *Command) WaitReady(ctx context.Context) error {
	// The child does not signal until completing PAM setup, which can take an arbitrary
	// amount of time, so we use a reasonably long timeout to avoid dubious lockouts.
	const childReadyWaitTimeout = 3 * time.Minute

	err := WaitForSignal(ctx, c.rdyR, childReadyWaitTimeout)
	if err != nil {
		c.logger.ErrorContext(ctx, "Child process never became ready.", "error", err)
	}
	return trace.Wrap(err)
}

// Continue will resume execution of the process after it completes its
// pre-processing routine (placed in a cgroup).
func (c *Command) Continue(ctx context.Context) {
	if err := c.contW.Close(); err != nil {
		c.logger.WarnContext(ctx, "failed to close the continue pipe", "error", err)
	}
}

// Close frees up resources associated with the command.
func (c *Command) Close() error {
	var errs []error

	// Stop the process if it is running.
	if err := c.stop(5 * time.Second); err != nil {
		slog.WarnContext(context.Background(), "Unexpected error stopping reexec process", "err", err)
	}

	// Close the parent side of the common parent-to-child named pipes.
	// These may already be closed in the normal execution of the command,
	// so the close errors can be ignored.
	closeAll(c.cfgW, c.termW, c.contW)

	if err := closeAll(filesToClosers(c.childPipes)...); err != nil {
		errs = append(errs, trace.Wrap(err, "failed to close child pipes"))
	}

	if err := closeAll(c.parentReadPipes...); err != nil {
		errs = append(errs, trace.Wrap(err, "failed to close parent pipes"))
	}

	return trace.NewAggregate(errs...)
}

// stop attempts to stop the reexec process, first with a graceful termination signal
// before falling back to a kill signal after 5 seconds.
func (c *Command) stop(gracefulTimeout time.Duration) error {
	if !c.isRunning() {
		return nil
	}

	// First attempt graceful termination by signaling through the terminate pipe.
	c.termW.Close()
	select {
	case <-time.After(gracefulTimeout):
		slog.DebugContext(context.Background(), "Failed to stop reexec process gracefully, sending kill signal.", "command", c.Command)
	case <-c.done:
		return nil
	}

	err := c.cmd.Process.Kill()

	// Wait for the kill signal to result in the termination of process, otherwise tests
	// that create a temporary user may fail to delete the user at the end of the test
	// while the kill signal is propagating.
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		slog.DebugContext(context.Background(), "Reexec process still running after kill signal.", "command", c.Command)
	}

	return trace.Wrap(err)
}

func (c *Command) isRunning() bool {
	select {
	case <-c.done:
		return true
	default:
		// If the process is set (started), then it must be running.
		return c.cmd.Process != nil
	}
}

// Wait for the command to complete.
// Must not be called without a call to Start.
func (c *Command) Wait() error {
	<-c.done
	return trace.Wrap(c.exitErr)
}

// Done return a channel that returns when the command completes.
// Must not be called without a call to Start.
func (c *Command) Done() <-chan struct{} {
	return c.done
}

// PID returns the command PID.
func (c *Command) PID() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

// Command returns the command to run.
func (c *Command) Command() string {
	if c.cmd == nil {
		return ""
	}
	return c.cmd.String()
}

// Command returns the command to run.
func (c *Command) Env() []string {
	if c.cmd == nil {
		return nil
	}
	return c.Env()
}

// ExitCode returns the exit code.
func (c *Command) ExitCode() int {
	if c.cmd == nil || c.cmd.ProcessState == nil {
		return 0
	}
	return c.cmd.ProcessState.ExitCode()
}

func closeAll(files ...io.Closer) error {
	var errs []error
	for _, f := range files {
		// Nil pipes may be set as placeholders, e.g. for deprecated fds.
		if f == nil {
			continue

		}
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

func filesToClosers(files []*os.File) []io.Closer {
	closers := make([]io.Closer, len(files))
	for i, f := range files {
		closers[i] = f
	}
	return closers
}
