/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sshutils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/ssh"
)

// SSHRequest defines an interface for ssh.Request.
type SSHRequest interface {
	// Reply sends a response to a request.
	Reply(ok bool, payload []byte) error
}

func sshRequestType(r SSHRequest) string {
	if sshReq, ok := r.(*ssh.Request); ok {
		return sshReq.Type
	}
	return "unknown"
}

// Reply is a helper to handle replying/rejecting and log messages when needed.
type Reply struct {
	log *slog.Logger
}

// NewReply creates a new reply helper for SSH servers.
func NewReply(log *slog.Logger) *Reply {
	return &Reply{log: log}
}

// RejectChannel rejects the channel with provided message.
func (r *Reply) RejectChannel(ctx context.Context, nch ssh.NewChannel, reason ssh.RejectionReason, msg string) {
	if err := nch.Reject(reason, msg); err != nil {
		r.log.WarnContext(ctx, "Failed to reject channel", "error", err)
	}
}

// RejectUnknownChannel rejects the channel with reason ssh.UnknownChannelType.
func (r *Reply) RejectUnknownChannel(ctx context.Context, nch ssh.NewChannel) {
	channelType := nch.ChannelType()
	r.log.WarnContext(ctx, "Unknown channel type", "channel", channelType)
	r.RejectChannel(ctx, nch, ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
}

// RejectWithAcceptError rejects the channel when ssh.NewChannel.Accept fails.
func (r *Reply) RejectWithAcceptError(ctx context.Context, nch ssh.NewChannel, err error) {
	r.log.WarnContext(ctx, "Unable to accept channel", "channel", nch.ChannelType(), "error", err)
	r.RejectChannel(ctx, nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
}

// RejectWithNewRemoteSessionError rejects the channel when the corresponding
// remote session fails to create.
func (r *Reply) RejectWithNewRemoteSessionError(ctx context.Context, nch ssh.NewChannel, remoteError error) {
	r.log.WarnContext(ctx, "Remote session open failed", "error", remoteError)
	reason, msg := ssh.ConnectionFailed, fmt.Sprintf("remote session open failed: %v", remoteError)
	var e *ssh.OpenChannelError
	if errors.As(remoteError, &e) {
		reason, msg = e.Reason, e.Message
	}
	r.RejectChannel(ctx, nch, reason, msg)
}

// ReplyError replies an error to an ssh.Request.
func (r *Reply) ReplyError(ctx context.Context, req SSHRequest, err error) {
	r.log.WarnContext(ctx, "failure handling SSH request", "request_type", sshRequestType(req), "error", err)
	if err := req.Reply(false, []byte(err.Error())); err != nil {
		r.log.WarnContext(ctx, "failed sending error Reply on SSH channel", "error", err)
	}
}

// ReplyRequest replies to an ssh.Request with provided ok and payload.
func (r *Reply) ReplyRequest(ctx context.Context, req SSHRequest, ok bool, payload []byte) {
	if err := req.Reply(ok, payload); err != nil {
		r.log.WarnContext(ctx, "failed replying OK to SSH request", "request_type", sshRequestType(req), "error", err)
	}
}

// SendExitStatus sends an exit-status.
func (r *Reply) SendExitStatus(ctx context.Context, ch ssh.Channel, code int) {
	_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(code)}))
	if err != nil {
		r.log.InfoContext(ctx, "Failed to send exit status", "error", err)
	}
}
