/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package srv

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/user"
	"strconv"
	"sync"

	"github.com/creack/pty"
	"github.com/gravitational/trace"
	"github.com/moby/term"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils/reexec"
)

// LookupUser is used to mock the value returned by user.Lookup(string).
type LookupUser func(string) (*user.User, error)

// LookupGroup is used to mock the value returned by user.LookupGroup(string).
type LookupGroup func(string) (*user.Group, error)

// Terminal defines an interface of handy functions for managing a (local or
// remote) PTY, such as resizing windows, executing commands with a PTY, and
// cleaning up.
type Terminal interface {
	// AddParty adds another participant to this terminal. We will keep the
	// Terminal open until all participants have left.
	AddParty(delta int)

	// Run will run the terminal.
	Run(ctx context.Context) error

	// Wait will block until the terminal is complete.
	Wait() (*ExecResult, error)

	// WaitForChild blocks until the child process has completed any required
	// setup operations before proceeding with execution.
	WaitForChild(ctx context.Context) error

	// Continue will resume execution of the process after it completes its
	// pre-processing routine (placed in a cgroup).
	Continue()

	// PTY returns the PTY backing the terminal.
	PTY() io.ReadWriter

	// TTYName returns the name of TTY backing the terminal.
	TTYName() string

	// PID returns the PID of the Teleport process that was re-execed.
	PID() int

	// Close will free resources associated with the terminal.
	Close() error

	// GetWinSize returns the window size of the terminal.
	GetWinSize() (*term.Winsize, error)

	// SetWinSize sets the window size of the terminal.
	SetWinSize(ctx context.Context, params rsession.TerminalParams) error

	// GetTerminalParams is a fast call to get cached terminal parameters
	// and avoid extra system call.
	GetTerminalParams() rsession.TerminalParams

	// SetTerminalModes sets the terminal modes from "pty-req"
	SetTerminalModes(ssh.TerminalModes)

	// GetTermType gets the terminal type set in "pty-req"
	GetTermType() string

	// SetTermType sets the terminal type from "pty-req"
	SetTermType(string)
}

// NewTerminal returns a new terminal. Terminal can be local or remote
// depending on cluster configuration.
func NewTerminal(ctx *ServerContext) (Terminal, error) {
	// It doesn't matter what mode the cluster is in, if this is a Teleport node
	// return a local terminal.
	if ctx.srv.Component() == teleport.ComponentNode {
		return newLocalTerminal(ctx)
	}

	// If this is not a Teleport node, find out what mode the cluster is in and
	// return the correct terminal.
	if ctx.srv.Component() == teleport.ComponentForwardingNode {
		return newRemoteTerminal(ctx)
	}
	return newLocalTerminal(ctx)
}

// terminal is a local PTY created by Teleport nodes.
type terminal struct {
	wg sync.WaitGroup
	mu sync.Mutex

	log *slog.Logger

	reexecCmd     *reexec.Command
	serverContext *ServerContext

	pty     *os.File
	tty     *os.File
	ttyName string

	termType string
	params   rsession.TerminalParams
}

// NewLocalTerminal creates and returns a local PTY.
func newLocalTerminal(ctx *ServerContext) (*terminal, error) {
	logger := ctx.Logger.With(teleport.ComponentKey, teleport.ComponentLocalTerm)

	// Open PTY and corresponding TTY.
	pty, tty, err := pty.Open()
	if err != nil {
		logger.WarnContext(ctx.CancelContext(), "Could not start PTY", "error", err)
		return nil, err
	}

	t := &terminal{
		log:           logger,
		serverContext: ctx,
		pty:           pty,
		tty:           tty,
		ttyName:       tty.Name(),
	}

	// Set the TTY owner. Failure is not fatal, for example Teleport is running
	// on a read-only filesystem, but logging is useful for diagnostic purposes.
	err = t.setOwner()
	if err != nil {
		t.log.DebugContext(ctx.CancelContext(), "Unable to set TTY owner", "error", err)
	}

	return t, nil
}

// AddParty adds another participant to this terminal. We will keep the
// Terminal open until all participants have left.
func (t *terminal) AddParty(delta int) {
	t.wg.Add(delta)
}

// Run will run the terminal.
func (t *terminal) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create the command that will actually execute.
	cmd, err := t.serverContext.ConfigureCommand()
	if err != nil {
		return trace.Wrap(err)
	}
	t.reexecCmd = cmd

	// Intentionally passing a nil value instead of the PTY. The child
	// process does not need the PTY, but for compatibility purposes the
	// first ExtraFiles is left for the PTY descriptor.
	t.reexecCmd.AddChildPipe(nil)
	// Pass the TTY to the child since a terminal is attached.
	t.reexecCmd.AddChildPipe(t.takeTTY())

	// Start the process.
	if err := t.reexecCmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Wait will block until the terminal is complete.
func (t *terminal) Wait() (*ExecResult, error) {
	exitCode, exitErr := t.reexecCmd.Wait()
	return &ExecResult{
		Code:    exitCode,
		Command: t.reexecCmd.Path(),
		Error:   exitErr,
	}, nil
}

func (t *terminal) WaitForChild(ctx context.Context) error {
	return t.serverContext.WaitForChild(ctx, t.reexecCmd)
}

// Continue will resume execution of the process after it completes its
// pre-processing routine (placed in a cgroup).
func (t *terminal) Continue() {
	t.reexecCmd.Continue()
}

// PTY returns the PTY backing the terminal.
func (t *terminal) PTY() io.ReadWriter {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pty
}

// TTYName returns the name of the TTY backing the terminal.
func (t *terminal) TTYName() string {
	return t.ttyName
}

// PID returns the PID of the Teleport process that was re-execed.
func (t *terminal) PID() int {
	if t.reexecCmd == nil {
		return 0
	}
	return t.reexecCmd.PID()
}

// Close will free resources associated with the terminal.
func (t *terminal) Close() error {
	var errs []error
	if t.reexecCmd != nil {
		if err := t.reexecCmd.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if tty := t.takeTTY(); tty != nil {
		if err := tty.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// note, pty is closed in the copying goroutine,
	// not here to avoid data races
	go t.closePTY()

	return trace.NewAggregate(errs...)
}

func (t *terminal) takeTTY() *os.File {
	t.mu.Lock()
	defer t.mu.Unlock()
	tty := t.tty
	t.tty = nil
	return tty
}

func (t *terminal) closePTY() {
	defer t.log.DebugContext(t.serverContext.CancelContext(), "Closed PTY")

	// wait until all copying is over (all participants have left)
	t.wg.Wait()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.pty == nil {
		return
	}

	if err := t.pty.Close(); err != nil {
		t.log.WarnContext(t.serverContext.CancelContext(), "Failed to close PTY", "error", err)
	}
	t.pty = nil
}

// GetWinSize returns the window size of the terminal.
func (t *terminal) GetWinSize() (*term.Winsize, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pty == nil {
		return nil, trace.NotFound("no pty")
	}
	ws, err := term.GetWinsize(t.pty.Fd())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ws, nil
}

// SetWinSize sets the window size of the terminal.
func (t *terminal) SetWinSize(ctx context.Context, params rsession.TerminalParams) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pty == nil {
		return trace.NotFound("no pty")
	}
	if err := term.SetWinsize(t.pty.Fd(), params.Winsize()); err != nil {
		return trace.Wrap(err)
	}
	t.params = params
	return nil
}

// GetTerminalParams is a fast call to get cached terminal parameters
// and avoid extra system call.
func (t *terminal) GetTerminalParams() rsession.TerminalParams {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.params
}

// GetTermType gets the terminal type set in "pty-req"
func (t *terminal) GetTermType() string {
	return t.termType
}

// SetTermType sets the terminal type from "req-pty" request.
func (t *terminal) SetTermType(term string) {
	if term == "" {
		term = defaultTerm
	}
	t.termType = term
}

func (t *terminal) SetTerminalModes(termModes ssh.TerminalModes) {}

// getOwner determines the uid, gid, and mode of the TTY similar to OpenSSH:
// https://github.com/openssh/openssh-portable/blob/ddc0f38/sshpty.c#L164-L215
func getOwner(login string, lookupUser LookupUser, lookupGroup LookupGroup) (int, int, os.FileMode, error) {
	var err error
	var uid int
	var gid int
	var mode os.FileMode

	// Lookup the Unix login for the UID and fallback GID.
	u, err := lookupUser(login)
	if err != nil {
		return 0, 0, 0, trace.Wrap(err)
	}
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, 0, trace.Wrap(err)
	}

	// If the tty group exists, use that as the gid of the TTY and set mode to
	// be u+rw. Otherwise use the group of the user with mode u+rw g+w.
	group, err := lookupGroup("tty")
	if err != nil {
		gid, err = strconv.Atoi(u.Gid)
		if err != nil {
			return 0, 0, 0, trace.Wrap(err)
		}
		mode = 0o620
	} else {
		gid, err = strconv.Atoi(group.Gid)
		if err != nil {
			return 0, 0, 0, trace.Wrap(err)
		}
		mode = 0o600
	}

	return uid, gid, mode, nil
}

// setOwner changes the owner and mode of the TTY.
func (t *terminal) setOwner() error {
	uid, gid, mode, err := getOwner(t.serverContext.Identity.Login, user.Lookup, user.LookupGroup)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.Lchown(t.tty.Name(), uid, gid)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Chmod(t.tty.Name(), mode)
	if err != nil {
		return trace.Wrap(err)
	}

	t.log.DebugContext(t.serverContext.CancelContext(), "Set permissions on tty", "tty_name", t.tty.Name(), "uid", uid, "gid", gid, "mode", mode)

	return nil
}

type remoteTerminal struct {
	wg sync.WaitGroup
	mu sync.Mutex

	log *slog.Logger

	ctx *ServerContext

	session   *tracessh.Session
	params    rsession.TerminalParams
	termModes ssh.TerminalModes
	ptyBuffer *ptyBuffer
	termType  string
}

func newRemoteTerminal(ctx *ServerContext) (*remoteTerminal, error) {
	if ctx.RemoteSession == nil {
		return nil, trace.BadParameter("remote session required")
	}

	t := &remoteTerminal{
		log:       ctx.Logger.With(teleport.ComponentKey, teleport.ComponentRemoteTerm),
		ctx:       ctx,
		session:   ctx.RemoteSession,
		ptyBuffer: &ptyBuffer{},
	}

	return t, nil
}

func (t *remoteTerminal) AddParty(delta int) {
	t.wg.Add(delta)
}

type ptyBuffer struct {
	r io.Reader
	w io.Writer
}

func (b *ptyBuffer) Read(p []byte) (n int, err error) {
	return b.r.Read(p)
}

func (b *ptyBuffer) Write(p []byte) (n int, err error) {
	return b.w.Write(p)
}

func (t *remoteTerminal) Run(ctx context.Context) error {
	// prepare the remote session by setting environment variables
	t.prepareRemoteSession(ctx, t.session, t.ctx)

	// combine stdout and stderr
	stdout, err := t.session.StdoutPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	t.session.Stderr = t.session.Stdout
	stdin, err := t.session.StdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	// create a pty buffer that stdin and stdout are hooked up to
	t.ptyBuffer = &ptyBuffer{
		r: stdout,
		w: stdin,
	}

	// if a specific term type was not requested, then pick the default one and request a pty
	if t.termType == "" {
		t.termType = defaultTerm
	}

	if err := t.session.RequestPty(ctx, t.termType, t.params.H, t.params.W, t.termModes); err != nil {
		return trace.Wrap(err)
	}

	// we want to run a "exec" command within a pty
	if execRequest, err := t.ctx.GetExecRequest(); err == nil && execRequest.GetCommand() != "" {
		t.log.DebugContext(ctx, "Running exec request within a PTY")

		if err := t.session.Start(ctx, execRequest.GetCommand()); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	// we want an interactive shell
	t.log.DebugContext(ctx, "Requesting an interactive terminal", "term_type", t.termType)
	if err := t.session.Shell(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (t *remoteTerminal) Wait() (*ExecResult, error) {
	execRequest, err := t.ctx.GetExecRequest()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = t.session.Wait()
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return &ExecResult{
				Code:    exitErr.ExitStatus(),
				Command: execRequest.GetCommand(),
				Error:   err,
			}, nil
		}

		return &ExecResult{
			Code:    teleport.RemoteCommandFailure,
			Command: execRequest.GetCommand(),
			Error:   err,
		}, nil
	}

	return &ExecResult{
		Code:    teleport.RemoteCommandSuccess,
		Command: execRequest.GetCommand(),
	}, nil
}

func (t *remoteTerminal) WaitForChild(context.Context) error {
	return nil
}

// Continue does nothing for remote command execution.
func (t *remoteTerminal) Continue() {}

func (t *remoteTerminal) PTY() io.ReadWriter {
	return t.ptyBuffer
}

func (t *remoteTerminal) TTYName() string {
	return ""
}

// PID returns the PID of the Teleport process that was re-execed. Always
// returns 0 for remote terminals.
func (t *remoteTerminal) PID() int {
	return 0
}

func (t *remoteTerminal) Close() error {
	// this closes the underlying stdin,stdout,stderr which is what ptyBuffer is
	// hooked to directly
	err := t.session.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	// Wait for parties to be relased after closing the remote session. This
	// avoid cases where the parties are blocked, reading from the remote
	// session.
	t.wg.Wait()

	t.log.DebugContext(t.ctx.CancelContext(), "Closed remote terminal and underlying SSH session")
	return nil
}

func (t *remoteTerminal) GetWinSize() (*term.Winsize, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.params.Winsize(), nil
}

func (t *remoteTerminal) SetWinSize(ctx context.Context, params rsession.TerminalParams) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	err := t.windowChange(ctx, params.W, params.H)
	if err != nil {
		return trace.Wrap(err)
	}
	t.params = params

	return nil
}

func (t *remoteTerminal) GetTerminalParams() rsession.TerminalParams {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.params
}

// GetTermType gets the terminal type set in "pty-req"
func (t *remoteTerminal) GetTermType() string {
	return t.termType
}

func (t *remoteTerminal) SetTermType(term string) {
	if term == "" {
		term = defaultTerm
	}
	t.termType = term
}

func (t *remoteTerminal) SetTerminalModes(termModes ssh.TerminalModes) {
	t.termModes = termModes
}

func (t *remoteTerminal) windowChange(ctx context.Context, w int, h int) error {
	return trace.Wrap(t.session.WindowChange(ctx, h, w))
}

// prepareRemoteSession prepares the remote session with env vars provided by the forwarding server or client.
func (t *remoteTerminal) prepareRemoteSession(ctx context.Context, session *tracessh.Session, scx *ServerContext) {
	envs := map[string]string{
		teleport.SSHTeleportUser:        scx.Identity.TeleportUser,
		teleport.SSHTeleportHostUUID:    scx.srv.ID(),
		teleport.SSHTeleportClusterName: scx.ClusterName,
		teleport.SSHSessionID:           scx.SessionID(),
	}

	if scx.GetSessionParams().WebProxyAddr != "" {
		envs[teleport.SSHSessionWebProxyAddr] = scx.GetSessionParams().WebProxyAddr
	}

	if err := session.SetEnvs(ctx, envs); err != nil {
		t.log.DebugContext(ctx, "Unable to set environment variables", "error", err)
	}
}
