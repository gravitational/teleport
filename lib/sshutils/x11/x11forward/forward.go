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

// Package X11 contains contains the ssh client/server helper functions
// for performing X11 forwarding.
package x11forward

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/sshutils/x11"
)

type Session interface {
	SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, error)
}

// RequestForwarding sends an "x11-req" to the server to set up X11 forwarding for the given session.
// authProto and authCookie are required to set up authentication with the Server. screenNumber is used
// by the server to determine which screen should be connected to for X11 forwarding. singleConnection is
// an optional argument to request X11 forwarding for a single connection.
func RequestForwarding(ctx context.Context, sess Session, xauthEntry *x11.XAuthEntry) error {
	payload := x11.ForwardRequestPayload{
		AuthProtocol: xauthEntry.Proto,
		AuthCookie:   xauthEntry.Cookie,
		ScreenNumber: uint32(xauthEntry.Display.ScreenNumber),
	}

	ok, err := sess.SendRequest(ctx, x11.ForwardRequest, true, ssh.Marshal(payload))
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("X11 forward request failed")
	}

	return nil
}

type x11ChannelHandler func(ctx context.Context, nch ssh.NewChannel)

// ServeChannelRequests opens an X11 channel handler and starts a
// goroutine to serve any channels received with the handler provided.
func ServeChannelRequests(ctx context.Context, clt *ssh.Client, handler x11ChannelHandler) error {
	channels := clt.HandleChannelOpen(x11.ChannelRequest)
	if channels == nil {
		return trace.Wrap(trace.AlreadyExists("X11 forwarding channel already open"))
	}

	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for {
			select {
			case nch := <-channels:
				if nch == nil {
					return
				}
				go handler(ctx, nch)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}
