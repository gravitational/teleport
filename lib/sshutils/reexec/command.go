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

	// Cmd is the reexec command to be executed.
	// TODO(Joerger): make this a local field once external setup (stderr, etc) is internalized to this package.
	Cmd *exec.Cmd

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

// NewReexecCommand allocates a ReexecCommand with the common reexec pipes.
func NewReexecCommand(cfg *Config) (*Command, error) {
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

	c := &Command{
		logger: slog.Default(),
		Cmd:    cmd,
		cfg:    cfg,
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
func (c *Command) Start(ctx context.Context) error {
	if c.Cmd == nil {
		return trace.BadParameter("missing exec command")
	}

	// Prepare the child pipes and ensure they are closed after the command starts.
	c.Cmd.ExtraFiles = c.childPipes
	c.childPipes = nil
	defer closePipes(filesToClosers(c.Cmd.ExtraFiles)...)

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

	if err := c.Cmd.Start(); err != nil {
		closeErr := c.Close()
		return trace.NewAggregate(err, closeErr)
	}

	return nil
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

// The child does not signal until completing PAM setup, which can take an arbitrary
// amount of time, so we use a reasonably long timeout to avoid dubious lockouts.
const childReadyWaitTimeout = 3 * time.Minute

func (c *Command) WaitReady(ctx context.Context) error {
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
		c.logger.WarnContext(ctx, "failed to close the continue pipe")
	}
}

// Terminate attempts to kill the shell/bash process.
func (c *Command) Terminate(ctx context.Context) {
	if err := c.termW.Close(); err != nil {
		c.logger.WarnContext(ctx, "failed to close the terminate pipe")
	}
}

// Wait for the underlying exec.Cmd to complete.
func (c *Command) Wait(ctx context.Context) error {
	if c.Cmd == nil {
		return trace.BadParameter("missing exec command")
	}

	defer c.Close()

	return trace.Wrap(c.Cmd.Wait())
}

// Close closes any open pipes used by the command during execution.
func (c *Command) Close() error {
	var errs []error

	// Close the parent side of the common parent-to-child named pipes.
	// These may already be closed in the normal execution of the command,
	// so the close errors can be ignored.
	closePipes(c.cfgW, c.termW, c.contW)

	if err := closePipes(filesToClosers(c.childPipes)...); err != nil {
		errs = append(errs, trace.Wrap(err, "failed to close child pipes"))
	}

	if err := closePipes(c.parentReadPipes...); err != nil {
		errs = append(errs, trace.Wrap(err, "failed to close parent pipes"))
	}

	return trace.NewAggregate(errs...)
}

func closePipes(files ...io.Closer) error {
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
