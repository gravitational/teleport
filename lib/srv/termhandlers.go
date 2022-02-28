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
	"golang.org/x/crypto/ssh"

	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
)

// TermHandlers are common terminal handling functions used by both the
// regular and forwarding server.
type TermHandlers struct {
	SessionRegistry *SessionRegistry
}

// HandleExec handles requests of type "exec" which can execute with or
// without a TTY. Result of execution is propagated back on the ExecResult
// channel of the context.
func (t *TermHandlers) HandleExec(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	// Save the request within the context.
	ctx.request = req

	// Parse the exec request and store it in the context.
	_, err := parseExecRequest(req, ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// If a terminal was previously allocated for this command, run command in
	// an interactive session. Otherwise run it in an exec session.
	if ctx.GetTerm() != nil {
		return t.SessionRegistry.OpenSession(ch, req, ctx)
	}
	return t.SessionRegistry.OpenExecSession(ch, req, ctx)
}

// HandlePTYReq handles requests of type "pty-req" which allocate a TTY for
// "exec" or "shell" requests. The "pty-req" includes the size of the TTY as
// well as the terminal type requested.
func (t *TermHandlers) HandlePTYReq(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
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
	ctx.Debugf("Requested terminal %q of size %v", ptyRequest.Env, *params)

	// get an existing terminal or create a new one
	term := ctx.GetTerm()
	if term == nil {
		// a regular or forwarding terminal will be allocated
		term, err = NewTerminal(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.SetTerm(term)
		ctx.termAllocated = true
	}
	if err := term.SetWinSize(*params); err != nil {
		ctx.Errorf("Failed setting window size: %v", err)
	}
	term.SetTermType(ptyRequest.Env)
	term.SetTerminalModes(termModes)

	// update the session
	if err := t.SessionRegistry.NotifyWinChange(*params, ctx); err != nil {
		ctx.Errorf("Unable to update session: %v", err)
	}

	return nil
}

// HandleShell handles requests of type "shell" which request a interactive
// shell be created within a TTY.
func (t *TermHandlers) HandleShell(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	var err error

	// Save the request within the context.
	ctx.request = req

	// Creating an empty exec request implies a interactive shell was requested.
	ctx.ExecRequest, err = NewExecRequest(ctx, "")
	if err != nil {
		return trace.Wrap(err)
	}
	if err := t.SessionRegistry.OpenSession(ch, req, ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// HandleWinChange handles requests of type "window-change" which update the
// size of the PTY running on the server and update any other members in the
// party.
func (t *TermHandlers) HandleWinChange(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	params, err := parseWinChange(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// Update any other members in the party that the window size has changed
	// and to update their terminal windows accordingly.
	err = t.SessionRegistry.NotifyWinChange(*params, ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (t *TermHandlers) HandleForceTerminate(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	err := t.SessionRegistry.ForceTerminate(ctx)
	return trace.Wrap(err)
}

func parseExecRequest(req *ssh.Request, ctx *ServerContext) (Exec, error) {
	var err error

	var r sshutils.ExecReq
	if err = ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx.ExecRequest, err = NewExecRequest(ctx, r.Command)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ctx.ExecRequest, nil
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
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return params, nil
}
