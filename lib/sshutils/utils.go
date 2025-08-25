/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// JoinHostPort is a wrapper for net.JoinHostPort that takes a uint32 port.
func JoinHostPort(host string, port uint32) string {
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}

// SplitHostPort is a wrapper for net.SplitHostPort that returns a uint32 port.
// Note that unlike net.SplitHostPort, a missing port is valid and will return
// a zero port.
func SplitHostPort(addrString string) (string, uint32, error) {
	addr, err := utils.ParseHostPortAddr(addrString, 0)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}
	return addr.Host(), uint32(addr.Port(0)), nil
}

// SSHConnMetadataWithUser overrides an ssh.ConnMetadata with provided user.
type SSHConnMetadataWithUser struct {
	ssh.ConnMetadata
	user string
}

// NewSSHConnMetadataWithUser overrides an ssh.ConnMetadata with provided user.
func NewSSHConnMetadataWithUser(conn ssh.ConnMetadata, user string) SSHConnMetadataWithUser {
	return SSHConnMetadataWithUser{
		ConnMetadata: conn,
		user:         user,
	}
}

// User returns the user ID for this connection.
func (s SSHConnMetadataWithUser) User() string {
	return s.user
}

// SessionIDStatus indicates whether the session ID was received from
// the server or not, and if not why
type SessionIDStatus int

const (
	// SessionIDReceived indicates the the session ID was received
	SessionIDReceived SessionIDStatus = iota + 1
	// SessionIDNotSent indicates that the server set the session ID
	// but didn't send it to us
	SessionIDNotSent
	// SessionIDNotModified indicates that the server used the session
	// ID that was set by us
	SessionIDNotModified
)

// PrepareToReceiveSessionID configures the TeleportClient to listen for
// the server to send the session ID it's using. The returned function
// will return the current session ID from the server or a reason why
// one wasn't received.
func PrepareToReceiveSessionID(ctx context.Context, log *slog.Logger, client *tracessh.Client) func() (session.ID, SessionIDStatus) {
	// send the session ID received from the server
	var gotSessionID atomic.Bool
	sessionIDFromServer := make(chan session.ID, 1)

	client.HandleSessionRequest(ctx, teleport.CurrentSessionIDRequest, func(ctx context.Context, req *ssh.Request) {
		// only handle the first session ID request
		if gotSessionID.Load() {
			return
		}

		sid, err := session.ParseID(string(req.Payload))
		if err != nil {
			log.WarnContext(ctx, "Unable to parse session ID", "error", err)
			return
		}

		if gotSessionID.CompareAndSwap(false, true) {
			sessionIDFromServer <- *sid
		}
	})

	// If the session is about to close and we haven't received a session
	// ID yet, ask if the server even supports sending one. Send the
	// request in a new goroutine so session establishment won't be
	// blocked on making this request
	serverWillSetSessionID := make(chan bool, 1)
	go func() {
		resp, _, err := client.SendRequest(ctx, teleport.SessionIDQueryRequest, true, nil)
		if err != nil {
			log.WarnContext(ctx, "Failed to send session ID query request", "error", err)
			serverWillSetSessionID <- false
		} else {
			serverWillSetSessionID <- resp
		}
	}()

	waitForSessionID := func() (session.ID, SessionIDStatus) {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		for {
			select {
			case sessionID := <-sessionIDFromServer:
				return sessionID, SessionIDReceived
			case sessionIDIsComing := <-serverWillSetSessionID:
				if !sessionIDIsComing {
					return "", SessionIDNotModified
				}
				// the server will send the session ID, continue
				// waiting for it
			case <-ctx.Done():
				return "", SessionIDNotSent
			case <-timer.C:
				return "", SessionIDNotSent
			}
		}
	}

	return waitForSessionID
}
