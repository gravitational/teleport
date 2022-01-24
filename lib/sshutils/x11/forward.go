// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package X11 contains contains the ssh client/server helper functions
// for performing X11 forwarding.
package x11

import (
	"context"
	"io"
	"sync"

	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// forwardIO forwards io between two XServer connections until
// one of the connections is closed. If the ctx is closed early,
// the function will return, but forwarding will continue until
// the XServer connnections are closed.
func Forward(ctx context.Context, client, server XServerConn) error {
	errs := make(chan error)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(client, server)
		errs <- trace.Wrap(err)
		// Send other goroutine an EOF
		err = client.CloseWrite()
		errs <- trace.Wrap(err)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(server, client)
		errs <- trace.Wrap(err)
		// Send other goroutine an EOF
		err = server.CloseWrite()
		errs <- trace.Wrap(err)
	}()

	go func() {
		wg.Wait()
		close(errs)
	}()

	return trace.NewAggregateFromChannel(errs, ctx)
}

// ForwardRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type ForwardRequestPayload struct {
	// SingleConnection determines whether any connections will be forwarded
	// after the first connection, or after the session is closed. In OpenSSH
	// and Teleport SSH clients, SingleConnection is always set to false.
	SingleConnection bool
	// AuthProtocol is the name of the X11 authentication protocol being used.
	AuthProtocol string
	// AuthCookie is a hexadecimal encoded X11 authentication cookie. This should
	// be a fake, random cookie, which will be checked and replaced by the real
	// cookie once the connection request is received.
	AuthCookie string
	// ScreenNumber determines which screen will be.
	ScreenNumber uint32
}

// RequestForwarding sends an "x11-req" to the server to set up X11 forwarding for the given session.
// authProto and authCookie are required to set up authentication with the Server. screenNumber is used
// by the server to determine which screen should be connected to for X11 forwarding. singleConnection is
// an optional argument to request X11 forwarding for a single connection.
func RequestForwarding(sess *ssh.Session, xauthEntry *XAuthEntry) error {
	payload := ForwardRequestPayload{
		AuthProtocol: xauthEntry.Proto,
		AuthCookie:   xauthEntry.Cookie,
		ScreenNumber: uint32(xauthEntry.Display.ScreenNumber),
	}

	ok, err := sess.SendRequest(sshutils.X11ForwardRequest, true, ssh.Marshal(payload))
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("X11 forward request failed")
	}

	return nil
}

// ChannelRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type ChannelRequestPayload struct {
	// OriginatorAddress is the address of the server requesting an X11 channel
	OriginatorAddress string
	// OriginatorPort is the port of the server requesting an X11 channel
	OriginatorPort uint32
}

type x11ChannelHandler func(ctx context.Context, nch ssh.NewChannel)

// ServeChannelRequests opens an X11 channel handler and starts a
// goroutine to serve any channels received with the handler provided.
func ServeChannelRequests(ctx context.Context, clt *ssh.Client, handler x11ChannelHandler) error {
	channels := clt.HandleChannelOpen(sshutils.X11ChannelRequest)
	if channels == nil {
		return trace.AlreadyExists("X11 forwarding channel already open")
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

// ServerConfig is a server configuration for X11 forwarding
type ServerConfig struct {
	// Enabled controls whether X11 forwarding requests can be granted by the server.
	Enabled bool `yaml:"enabled"`
	// DisplayOffset tells the server what X11 display number to start from.
	DisplayOffset int `yaml:"display_offset,omitempty"`
}
