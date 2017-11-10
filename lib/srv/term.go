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
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"

	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	"github.com/kr/pty"
	"github.com/moby/moby/pkg/term"
	log "github.com/sirupsen/logrus"
)

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

	// Kill will force kill the terminal.
	Kill() error

	// PTY returns the PTY backing the terminal.
	PTY() io.ReadWriter

	// TTY returns the TTY backing the terminal.
	TTY() *os.File

	// Close will free resources associated with the terminal.
	Close() error

	// GetWinSize returns the window size of the terminal.
	GetWinSize() (*term.Winsize, error)

	// SetWinSize sets the window size of the terminal.
	SetWinSize(params rsession.TerminalParams) error

	// GetTerminalParams is a fast call to get cached terminal parameters
	// and avoid extra system call.
	GetTerminalParams() rsession.TerminalParams

	// SetTermType sets the terminal type from "req-pty"
	SetTermType(string)
}

// NewTerminal returns a new terminal. Terminal can be local or remote
// depending on cluster configuration.
func NewTerminal(ctx *ServerContext) (Terminal, error) {
	return NewLocalTerminal(ctx)
}

// terminal is a local PTY created by Teleport nodes.
type terminal struct {
	wg sync.WaitGroup
	mu sync.Mutex

	cmd *exec.Cmd
	ctx *ServerContext

	pty *os.File
	tty *os.File

	params rsession.TerminalParams
}

// NewLocalTerminal creates and returns a local PTY.
func NewLocalTerminal(ctx *ServerContext) (*terminal, error) {
	pty, tty, err := pty.Open()
	if err != nil {
		log.Warnf("could not start pty (%s)", err)
		return nil, err
	}
	return &terminal{
		ctx: ctx,
		pty: pty,
		tty: tty,
	}, nil
}

// AddParty adds another participant to this terminal. We will keep the
// Terminal open until all participants have left.
func (t *terminal) AddParty(delta int) {
	t.wg.Add(delta)
}

// Run will run the terminal.
func (t *terminal) Run() error {
	defer t.closeTTY()

	cmd, err := prepareInteractiveCommand(t.ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	t.cmd = cmd

	cmd.Stdout = t.tty
	cmd.Stdin = t.tty
	cmd.Stderr = t.tty
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true

	err = cmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}

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
	return t.pty
}

// TTY returns the TTY backing the terminal.
func (t *terminal) TTY() *os.File {
	return t.tty
}

// Close will free resources associated with the terminal.
func (t *terminal) Close() error {
	var err error
	// note, pty is closed in the copying goroutine,
	// not here to avoid data races
	if t.tty != nil {
		if e := t.tty.Close(); e != nil {
			err = e
		}
	}
	go t.closePTY()
	return trace.Wrap(err)
}

func (t *terminal) closeTTY() {
	if err := t.tty.Close(); err != nil {
		log.Warnf("failed to close TTY: %v", err)
	}
	t.tty = nil
}

func (t *terminal) closePTY() {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer log.Debugf("PTY is closed")

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

// SetTermType sets the terminal type from "req-pty" request.
func (t *terminal) SetTermType(term string) {
	if term == "" {
		term = defaultTerm
	}
	t.cmd.Env = append(t.cmd.Env, "TERM="+term)
}

func ParsePTYReq(req *ssh.Request) (*sshutils.PTYReqParams, error) {
	var r sshutils.PTYReqParams
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		log.Warnf("failed to parse PTY request: %v", err)
		return nil, err
	}

	// if the caller asked for an invalid sized pty (like ansible
	// which asks for a 0x0 size) update the request with defaults
	if err := r.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	return &r, nil
}
