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

package sshutils

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// RequestForwarder represents a resource capable of sending
// an ssh request such as an ssh.Channel or ssh.Session.
type RequestForwarder interface {
	SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, error)
}

// ForwardRequest is a helper for forwarding a request across a session or channel.
func ForwardRequest(ctx context.Context, sender RequestForwarder, req *ssh.Request) (bool, error) {
	reply, err := sender.SendRequest(ctx, req.Type, req.WantReply, req.Payload)
	if err != nil || !req.WantReply {
		return reply, trace.Wrap(err)
	}
	return reply, trace.Wrap(req.Reply(reply, nil))
}

// ForwardRequests forwards all ssh requests received from the
// given channel until the channel or context is closed.
func ForwardRequests(ctx context.Context, sin <-chan *ssh.Request, sender RequestForwarder) error {
	for {
		select {
		case sreq, ok := <-sin:
			if !ok {
				// channel closed, stop processing
				return nil
			}
			switch sreq.Type {
			case WindowChangeRequest:
				if _, err := ForwardRequest(ctx, sender, sreq); err != nil {
					return trace.Wrap(err)
				}
			default:
				if sreq.WantReply {
					sreq.Reply(false, nil)
				}
				continue
			}
		case <-ctx.Done():
			if ctx.Err() != context.Canceled {
				return trace.Wrap(ctx.Err())
			}
			return nil
		}
	}
}
