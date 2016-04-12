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
	"os"
	"os/exec"
	"sync"

	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
	"github.com/gravitational/trace"
	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"
)

// terminal provides handy functions for managing PTY, usch as resizing windows
// execing processes with PTY and cleaning up
type terminal struct {
	sync.WaitGroup
	sync.Mutex
	pty    *os.File
	tty    *os.File
	err    error
	done   bool
	params rsession.TerminalParams
}

func parsePTYReq(req *ssh.Request) (*sshutils.PTYReqParams, error) {
	var r sshutils.PTYReqParams
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		log.Infof("failed to parse PTY request: %v", err)
		return nil, err
	}
	return &r, nil
}

func newTerminal() (*terminal, error) {
	// Create new PTY
	pty, tty, err := pty.Open()
	if err != nil {
		log.Infof("could not start pty (%s)", err)
		return nil, err
	}
	return &terminal{pty: pty, tty: tty, err: err}, nil
}

func requestPTY(req *ssh.Request) (*terminal, *rsession.TerminalParams, error) {
	var r sshutils.PTYReqParams
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		log.Infof("failed to parse PTY request: %v", err)
		return nil, nil, trace.Wrap(err)
	}
	log.Infof("Parsed pty request pty(enn=%v, w=%v, h=%v)", r.Env, r.W, r.H)
	t, err := newTerminal()
	if err != nil {
		log.Infof("failed to create term: %v", err)
		return nil, nil, trace.Wrap(err)
	}
	params, err := rsession.NewTerminalParamsFromUint32(r.W, r.H)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	t.setWinsize(*params)
	return t, params, nil
}

func (t *terminal) getWinsize() (*term.Winsize, error) {
	t.Lock()
	defer t.Unlock()
	if t.pty == nil {
		return nil, trace.NotFound("no pty")
	}
	ws, err := term.GetWinsize(t.pty.Fd())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ws, nil
}

func (t *terminal) setWinsize(params rsession.TerminalParams) error {
	t.Lock()
	defer t.Unlock()
	if t.pty == nil {
		return trace.NotFound("no pty")
	}
	log.Infof("resizing terminal to %v", &params)
	if err := term.SetWinsize(t.pty.Fd(), params.Winsize()); err != nil {
		return trace.Wrap(err)
	}
	t.params = params
	return nil
}

// getTerminalParams is a fast call to get cached terminal parameters
// and avoid extra system call
func (t *terminal) getTerminalParams() rsession.TerminalParams {
	t.Lock()
	defer t.Unlock()
	return t.params
}

func (t *terminal) closeTTY() {
	if err := t.tty.Close(); err != nil {
		log.Infof("failed to close TTY: %v", err)
	}
	t.tty = nil
}

func (t *terminal) run(c *exec.Cmd) error {
	defer t.closeTTY()
	c.Stdout = t.tty
	c.Stdin = t.tty
	c.Stderr = t.tty
	c.SysProcAttr.Setctty = true
	c.SysProcAttr.Setsid = true
	return trace.Wrap(c.Start())
}

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

func (t *terminal) closePTY() {
	t.Lock()
	defer t.Unlock()

	// wait until all copying is over
	log.Infof("Terminal wait for copy to be over")
	t.Wait()
	log.Infof("Terminal copy is over")

	t.pty.Close()
	t.pty = nil
}

func parseWinChange(req *ssh.Request) (*rsession.TerminalParams, error) {
	var r sshutils.WinChangeReqParams
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}
	params, err := rsession.NewTerminalParamsFromUint32(r.W, r.H)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return params, nil
}
