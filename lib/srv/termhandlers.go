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
	execRequest, err := parseExecRequest(req, ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// a terminal has been previously allocate for this command.
	// run this inside an interactive session
	if ctx.GetTerm() != nil {
		return t.SessionRegistry.OpenSession(ch, req, ctx)
	}

	// otherwise, regular execution
	result, err := execRequest.Start(ch)
	if err != nil {
		return trace.Wrap(err)
	}
	// if the program failed to start, we should send that result back
	if result != nil {
		ctx.Debugf("Exec request (%v) result: %v", execRequest, result)
		ctx.SendExecResult(*result)
	}

	// in case if result is nil and no error, this means that program is
	// running in the background
	go func() {
		result, err = execRequest.Wait()
		if err != nil {
			ctx.Errorf("Exec request (%v) wait failed: %v", execRequest, err)
		}
		if result != nil {
			ctx.SendExecResult(*result)
		}
	}()

	return nil
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
	ctx.Debugf("Requested terminal of size %v", *params)

	// get an existing terminal or create a new one
	term := ctx.GetTerm()
	if term == nil {
		// a regular or forwarding terminal will be allocated
		term, err = NewTerminal(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.SetTerm(term)
	}
	term.SetWinSize(*params)
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

	// creating an empty exec request implies a interactive shell was requested
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
// size of the TTY running on the server.
func (t *TermHandlers) HandleWinChange(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	params, err := parseWinChange(req)
	if err != nil {
		ctx.Error(err)
		return trace.Wrap(err)
	}

	term := ctx.GetTerm()
	if term != nil {
		err = term.SetWinSize(*params)
		if err != nil {
			ctx.Errorf("Unable to set window size: %v", err)
		}
	}

	err = t.SessionRegistry.NotifyWinChange(*params, ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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
