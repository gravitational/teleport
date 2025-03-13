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
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	tracingssh "github.com/gravitational/teleport/api/observability/tracing/ssh"
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
	scx.Logger.DebugContext(ctx, "Terminal has been requested", "terminal", ptyRequest.Env, "width", params.W, "height", params.H)

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
		scx.ttyName = term.TTYName()
	}
	if err := term.SetWinSize(ctx, *params); err != nil {
		scx.Logger.ErrorContext(ctx, "Failed setting window size", "error", err)
	}
	term.SetTermType(ptyRequest.Env)
	term.SetTerminalModes(termModes)

	// update the session
	if err := t.SessionRegistry.NotifyWinChange(ctx, *params, scx); err != nil {
		scx.Logger.ErrorContext(ctx, "Unable to update session", "error", err)
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

// HandleFileTransferDecision handles requests of type "file-transfer-decision@goteleport.com" which will
// approve or deny an existing file transfer request. This response will update an active file transfer request
// accordingly and emit the updated file transfer request state to other members in the party.
func (t *TermHandlers) HandleFileTransferDecision(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	params, err := parseFileTransferDecisionRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}

	session := scx.getSession()
	if session == nil {
		t.SessionRegistry.logger.DebugContext(ctx, "Unable to create file transfer Request, no session found in context.")
		return nil
	}

	if params.Approved {
		return trace.Wrap(session.approveFileTransferRequest(params, scx))
	}

	return trace.Wrap(session.denyFileTransferRequest(params, scx))
}

// HandleFileTransferRequest handles requests of type "file-transfer-request" which will
// create a FileTransferRequest that will be sent to other members in the party to be
// reviewed and approved/denied.
func (t *TermHandlers) HandleFileTransferRequest(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	params, err := parseFileTransferRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}
	params.Requester = scx.Identity.TeleportUser

	session := scx.getSession()
	if session == nil {
		t.SessionRegistry.logger.DebugContext(ctx, "Unable to create file transfer Request, no session found in context.")
		return nil
	}

	return trace.Wrap(session.addFileTransferRequest(params, scx))
}

func (t *TermHandlers) HandleChatMessage(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *ServerContext) error {
	params, err := parseChatMessageRqeuest(req)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("\n\n\n\nRECEIVED MESSAGE: %s\n\n\n\n", params.Message)

	session := scx.getSession()
	if session == nil {
		t.SessionRegistry.logger.DebugContext(ctx, "Unable to create file transfer Request, no session found in context.")
		return nil
	}

	return trace.Wrap(session.addChatMessage(params.Message, scx))
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

func parseFileTransferRequest(req *ssh.Request) (*rsession.FileTransferRequestParams, error) {
	var r tracingssh.FileTransferReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	params := &rsession.FileTransferRequestParams{
		Location: r.Location,
		Filename: r.Filename,
		Download: r.Download,
	}
	return params, nil
}

func parseChatMessageRqeuest(req *ssh.Request) (*rsession.ChatMessageParams, error) {
	var r tracingssh.ChatMessageReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	params := &rsession.ChatMessageParams{
		Message: r.Message,
	}
	return params, nil
}

func parseFileTransferDecisionRequest(req *ssh.Request) (*rsession.FileTransferDecisionParams, error) {
	var r tracingssh.FileTransferDecisionReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	params := &rsession.FileTransferDecisionParams{
		RequestID: r.RequestID,
		Approved:  r.Approved,
	}
	return params, nil
}
