/*
Copyright 2017 Gravitational, Inc.

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
	"context"
	"encoding/json"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
)

// TermHandlers are common terminal handling functions used by both the
// regular and forwarding server.
type TermHandlers struct {
	SessionRegistry *SessionRegistry
}

// HandleExec handles requests of type "exec" which can execute with or
// without a TTY. Result of execution is propagated back on the ExecResult
// channel of the context.
func (t *TermHandlers) HandleExec(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	// Save the request within the context.
	if err := scx.SetSSHRequest(req); err != nil {
		return trace.Wrap(err)
	}

	// Parse the exec request and store it in the context.
	if _, err := parseExecRequest(req, scx); err != nil {
		return trace.Wrap(err)
	}

	// If a terminal was previously allocated for this command, run command in
	// an interactive session. Otherwise run it in an exec session.
	if scx.GetTerm() != nil {
		return t.SessionRegistry.OpenSession(ctx, ch, scx)
	}
	return t.SessionRegistry.OpenExecSession(ctx, ch, scx)
}

// HandlePTYReq handles requests of type "pty-req" which allocate a TTY for
// "exec" or "shell" requests. The "pty-req" includes the size of the TTY as
// well as the terminal type requested.
func (t *TermHandlers) HandlePTYReq(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	// parse and extract the requested window size of the pty
	ptyRequest, err := parsePTYReq(req)
	if err != nil {
		return trace.Wrap(err)
	}

	termModes, err := ptyRequest.TerminalModes()
	if err != nil {
		return trace.Wrap(err)
	}

	params, err := rsession.NewTerminalParamsFromUint32(ptyRequest.W, ptyRequest.H)
	if err != nil {
		return trace.Wrap(err)
	}
	scx.Debugf("Requested terminal %q of size %v", ptyRequest.Env, *params)

	// get an existing terminal or create a new one
	term := scx.GetTerm()
	if term == nil {
		// a regular or forwarding terminal will be allocated
		term, err = NewTerminal(scx)
		if err != nil {
			return trace.Wrap(err)
		}
		scx.SetTerm(term)
		scx.termAllocated = true
		if term.TTY() != nil {
			scx.ttyName = term.TTY().Name()
		}
	}
	if err := term.SetWinSize(ctx, *params); err != nil {
		scx.Errorf("Failed setting window size: %v", err)
	}
	term.SetTermType(ptyRequest.Env)
	term.SetTerminalModes(termModes)

	// update the session
	if err := t.SessionRegistry.NotifyWinChange(ctx, *params, scx); err != nil {
		scx.Errorf("Unable to update session: %v", err)
	}

	return nil
}

// HandleShell handles requests of type "shell" which request a interactive
// shell be created within a TTY.
func (t *TermHandlers) HandleShell(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	var err error

	// Save the request within the context.
	if err := scx.SetSSHRequest(req); err != nil {
		return trace.Wrap(err)
	}

	// Creating an empty exec request implies a interactive shell was requested.
	execRequest, err := NewExecRequest(scx, "")
	if err != nil {
		return trace.Wrap(err)
	}
	if err := scx.SetExecRequest(execRequest); err != nil {
		return trace.Wrap(err)
	}
	if err := t.SessionRegistry.OpenSession(ctx, ch, scx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// HandleWinChange handles requests of type "window-change" which update the
// size of the PTY running on the server and update any other members in the
// party.
func (t *TermHandlers) HandleWinChange(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	params, err := parseWinChange(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// Update any other members in the party that the window size has changed
	// and to update their terminal windows accordingly.
	err = t.SessionRegistry.NotifyWinChange(ctx, *params, scx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (t *TermHandlers) HandleForceTerminate(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	err := t.SessionRegistry.ForceTerminate(ctx)
	return trace.Wrap(err)
}

func (t *TermHandlers) HandleTerminalSize(req *ssh.Request) error {
	sessionID := string(req.Payload)
	size, err := t.SessionRegistry.GetTerminalSize(sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	payload, err := json.Marshal(size)
	if err != nil {
		return trace.Wrap(err)
	}

	req.Reply(true, payload)
	return nil
}

func parseExecRequest(req *ssh.Request, ctx *ServerContext) (Exec, error) {
	var err error

	var r sshutils.ExecReq
	if err = ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	execRequest, err := NewExecRequest(ctx, r.Command)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ctx.SetExecRequest(execRequest); err != nil {
		return nil, trace.Wrap(err)
	}

	return execRequest, nil
}

func parsePTYReq(req *ssh.Request) (*sshutils.PTYReqParams, error) {
	var r sshutils.PTYReqParams
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	// if the caller asked for an invalid sized pty (like ansible
	// which asks for a 0x0 size) update the request with defaults
	if err := r.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &r, nil
}

func parseWinChange(req *ssh.Request) (*rsession.TerminalParams, error) {
	var r sshutils.WinChangeReqParams
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	params, err := rsession.NewTerminalParamsFromUint32(r.W, r.H)
	return params, trace.Wrap(err)
}
