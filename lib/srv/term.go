/*
Copyright 2015 Gravitational, Inc.

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

package srv

import (
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	services "github.com/gravitational/teleport/lib/auth"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/kr/pty"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
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
	Run() error

	// Wait will block until the terminal is complete.
	Wait() (*ExecResult, error)

	// Continue will resume execution of the process after it completes its
	// pre-processing routine (placed in a cgroup).
	Continue()

	// Kill will force kill the terminal.
	Kill() error

	// PTY returns the PTY backing the terminal.
	PTY() io.ReadWriter

	// TTY returns the TTY backing the terminal.
	TTY() *os.File

	// PID returns the PID of the Teleport process that was re-execed.
	PID() int

	// Close will free resources associated with the terminal.
	Close() error

	// GetWinSize returns the window size of the terminal.
	GetWinSize() (*term.Winsize, error)

	// SetWinSize sets the window size of the terminal.
	SetWinSize(params rsession.TerminalParams) error

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
	if services.IsRecordAtProxy(ctx.ClusterConfig.GetSessionRecording()) {
		return newRemoteTerminal(ctx)
	}
	return newLocalTerminal(ctx)
}

// terminal is a local PTY created by Teleport nodes.
type terminal struct {
	wg sync.WaitGroup
	mu sync.Mutex

	log *log.Entry

	cmd *exec.Cmd
	ctx *ServerContext

	pty *os.File
	tty *os.File

	pid int

	termType string
	params   rsession.TerminalParams
}

// NewLocalTerminal creates and returns a local PTY.
func newLocalTerminal(ctx *ServerContext) (*terminal, error) {
	var err error

	t := &terminal{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentLocalTerm,
		}),
		ctx: ctx,
	}

	// Open PTY and corresponding TTY.
	t.pty, t.tty, err = pty.Open()
	if err != nil {
		log.Warnf("Could not start PTY %v", err)
		return nil, err
	}

	// Set the TTY owner. Failure is not fatal, for example Teleport is running
	// on a read-only filesystem, but logging is useful for diagnostic purposes.
	err = t.setOwner()
	if err != nil {
		log.Debugf("Unable to set TTY owner: %v.\n", err)
	}

	return t, nil
}

// AddParty adds another participant to this terminal. We will keep the
// Terminal open until all participants have left.
func (t *terminal) AddParty(delta int) {
	t.wg.Add(delta)
}

// Run will run the terminal.
func (t *terminal) Run() error {
	var err error
	defer t.closeTTY()

	// Create the command that will actually execute.
	t.cmd, err = ConfigureCommand(t.ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// we need the lock here to protect from concurrent calls to Close()
	t.mu.Lock()
	pty, tty := t.pty, t.tty
	t.mu.Unlock()

	// Pass PTY and TTY to child as well since a terminal is attached.
	t.cmd.ExtraFiles = append(t.cmd.ExtraFiles, pty)
	t.cmd.ExtraFiles = append(t.cmd.ExtraFiles, tty)

	// Start the process.
	err = t.cmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	// Save off the PID of the Teleport process under which the shell is executing.
	t.pid = t.cmd.Process.Pid

	return nil
}

// Wait will block until the terminal is complete.
func (t *terminal) Wait() (*ExecResult, error) {
	err := t.cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			return &ExecResult{Code: status.ExitStatus(), Command: t.cmd.Path}, nil
		}
		return nil, err
	}

	status, ok := t.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return nil, trace.Errorf("unknown exit status: %T(%v)", t.cmd.ProcessState.Sys(), t.cmd.ProcessState.Sys())
	}

	return &ExecResult{
		Code:    status.ExitStatus(),
		Command: t.cmd.Path,
	}, nil
}

// Continue will resume execution of the process after it completes its
// pre-processing routine (placed in a cgroup).
func (t *terminal) Continue() {
	t.ctx.contw.Close()
}

// Kill will force kill the terminal.
func (t *terminal) Kill() error {
	if t.cmd.Process != nil {
		if err := t.cmd.Process.Kill(); err != nil {
			if err.Error() != "os: process already finished" {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// PTY returns the PTY backing the terminal.
func (t *terminal) PTY() io.ReadWriter {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pty
}

// TTY returns the TTY backing the terminal.
func (t *terminal) TTY() *os.File {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.tty
}

// PID returns the PID of the Teleport process that was re-execed.
func (t *terminal) PID() int {
	return t.pid
}

// Close will free resources associated with the terminal.
func (t *terminal) Close() error {
	err := t.closeTTY()
	// note, pty is closed in the copying goroutine,
	// not here to avoid data races
	go t.closePTY()
	return trace.Wrap(err)
}

func (t *terminal) closeTTY() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tty == nil {
		return nil
	}

	err := t.tty.Close()
	t.tty = nil

	if err != nil {
		t.log.Warnf("Failed to close TTY: %v", err)
	}

	return trace.Wrap(err)
}

func (t *terminal) closePTY() {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.log.Debugf("Closed PTY")

	// wait until all copying is over (all participants have left)
	t.wg.Wait()

	t.pty.Close()
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
func (t *terminal) SetWinSize(params rsession.TerminalParams) error {
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
		mode = 0620
	} else {
		gid, err = strconv.Atoi(group.Gid)
		if err != nil {
			return 0, 0, 0, trace.Wrap(err)
		}
		mode = 0600
	}

	return uid, gid, mode, nil
}

// setOwner changes the owner and mode of the TTY.
func (t *terminal) setOwner() error {
	uid, gid, mode, err := getOwner(t.ctx.Identity.Login, user.Lookup, user.LookupGroup)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.Chown(t.tty.Name(), uid, gid)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Chmod(t.tty.Name(), mode)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Set permissions on %v to %v:%v with mode %v.", t.tty.Name(), uid, gid, mode)

	return nil
}

type remoteTerminal struct {
	wg sync.WaitGroup
	mu sync.Mutex

	log *log.Entry

	ctx *ServerContext

	session   *ssh.Session
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
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentRemoteTerm,
		}),
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

func (t *remoteTerminal) Run() error {
	// prepare the remote remote session by setting environment variables
	t.prepareRemoteSession(t.session, t.ctx)

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

	if err := t.session.RequestPty(t.termType, t.params.H, t.params.W, t.termModes); err != nil {
		return trace.Wrap(err)
	}

	// we want to run a "exec" command within a pty
	if t.ctx.ExecRequest.GetCommand() != "" {
		t.log.Debugf("Running exec request within a PTY")

		if err := t.session.Start(t.ctx.ExecRequest.GetCommand()); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	// we want an interactive shell
	t.log.Debugf("Requesting an interactive terminal of type %v", t.termType)
	if err := t.session.Shell(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (t *remoteTerminal) Wait() (*ExecResult, error) {
	err := t.session.Wait()
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return &ExecResult{
				Code:    exitErr.ExitStatus(),
				Command: t.ctx.ExecRequest.GetCommand(),
			}, err
		}

		return &ExecResult{
			Code:    teleport.RemoteCommandFailure,
			Command: t.ctx.ExecRequest.GetCommand(),
		}, err
	}

	return &ExecResult{
		Code:    teleport.RemoteCommandSuccess,
		Command: t.ctx.ExecRequest.GetCommand(),
	}, nil
}

// Continue does nothing for remote command execution.
func (t *remoteTerminal) Continue() {}

func (t *remoteTerminal) Kill() error {
	err := t.session.Signal(ssh.SIGKILL)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (t *remoteTerminal) PTY() io.ReadWriter {
	return t.ptyBuffer
}

func (t *remoteTerminal) TTY() *os.File {
	return nil
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

	t.log.Debugf("Closed remote terminal and underlying SSH session")

	return nil
}

func (t *remoteTerminal) GetWinSize() (*term.Winsize, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.params.Winsize(), nil
}

func (t *remoteTerminal) SetWinSize(params rsession.TerminalParams) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	err := t.windowChange(params.W, params.H)
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

func (t *remoteTerminal) windowChange(w int, h int) error {
	type windowChangeRequest struct {
		W   uint32
		H   uint32
		Wpx uint32
		Hpx uint32
	}
	req := windowChangeRequest{
		W:   uint32(w),
		H:   uint32(h),
		Wpx: uint32(w * 8),
		Hpx: uint32(h * 8),
	}
	_, err := t.session.SendRequest(sshutils.WindowChangeRequest, false, ssh.Marshal(&req))
	return err
}

// prepareRemoteSession prepares the more session for execution.
func (t *remoteTerminal) prepareRemoteSession(session *ssh.Session, ctx *ServerContext) {
	envs := map[string]string{
		teleport.SSHTeleportUser:        ctx.Identity.TeleportUser,
		teleport.SSHSessionWebproxyAddr: ctx.ProxyPublicAddress(),
		teleport.SSHTeleportHostUUID:    ctx.srv.ID(),
		teleport.SSHTeleportClusterName: ctx.ClusterName,
		teleport.SSHSessionID:           string(ctx.SessionID()),
	}

	for k, v := range envs {
		if err := session.Setenv(k, v); err != nil {
			t.log.Debugf("Unable to set environment variable: %v: %v", k, v)
		}
	}
}
