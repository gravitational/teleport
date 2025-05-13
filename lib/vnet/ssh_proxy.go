// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/utils"
)

// sshConn represents an established SSH client or server connection.
type sshConn struct {
	conn  ssh.Conn
	chans <-chan ssh.NewChannel
	reqs  <-chan *ssh.Request
}

// proxySSHConnection transparently proxies SSH channels and requests
// between 2 established SSH connections. serverConn represents an incoming SSH
// connection where this proxy acts as a server, client represents an outgoing
// SSH connection where this proxy acts as a client.
func proxySSHConnection(
	ctx context.Context,
	serverConn sshConn,
	clientConn sshConn,
) {
	closeConnections := func() {
		clientConn.conn.Close()
		serverConn.conn.Close()
	}

	// Avoid leaking goroutines by tracking them with a waitgroup.
	var wg sync.WaitGroup
	wg.Add(5)

	// Close both connections if the context is canceled.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer wg.Done()
		<-ctx.Done()
		closeConnections()
	}()

	// Proxy channels initiated by either connection.
	go func() {
		defer wg.Done()
		proxyChannels(ctx, serverConn.conn, clientConn.chans, closeConnections)
	}()
	go func() {
		defer wg.Done()
		proxyChannels(ctx, clientConn.conn, serverConn.chans, closeConnections)
	}()

	// Proxy global requests in both directions.
	go func() {
		defer wg.Done()
		proxyGlobalRequests(ctx, serverConn.conn, clientConn.reqs, closeConnections)
	}()
	go func() {
		defer wg.Done()
		proxyGlobalRequests(ctx, clientConn.conn, serverConn.reqs, closeConnections)
	}()

	wg.Wait()
}

func proxyChannels(
	ctx context.Context,
	targetConn ssh.Conn,
	chans <-chan ssh.NewChannel,
	closeConnections func(),
) {
	// Proxy each SSH channel in its own goroutine, make sure they don't leak by
	// tracking with a WaitGroup.
	var wg sync.WaitGroup
	for newChan := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proxyChannel(ctx, targetConn, newChan, closeConnections)
		}()
	}
	// We get here when the source SSH connection was closed, make sure to close
	// the opposite connection too.
	closeConnections()
	wg.Wait()
}

func proxyChannel(
	ctx context.Context,
	targetConn ssh.Conn,
	newChan ssh.NewChannel,
	closeConnections func(),
) {
	log := log.With("channel_type", newChan.ChannelType())
	log.DebugContext(ctx, "Proxying new SSH channel")

	// Try to open a corresponding channel on the target.
	targetChan, targetChanRequests, err := targetConn.OpenChannel(
		newChan.ChannelType(), newChan.ExtraData())
	if err != nil {
		var openChannelErr *ssh.OpenChannelError
		var rejectErr error
		if errors.As(err, &openChannelErr) {
			// The target rejected the channel, this is totally expected in some
			// cases, just reject the incoming channel request.
			rejectErr = trace.Wrap(newChan.Reject(openChannelErr.Reason, openChannelErr.Message))
		} else {
			// We got an unexpected error trying to open the channel on the
			// target, this is fatal, log and kill the connection.
			log.DebugContext(ctx, "Unexpected error opening SSH channel on target",
				"error", err)
			msg := "unexpected error opening channel on target: " + err.Error()
			rejectErr = trace.Wrap(newChan.Reject(ssh.ConnectionFailed, msg))
			closeConnections()
		}
		if rejectErr != nil {
			// We failed to reject the incoming channel, this is fatal, log and
			// kill the connection.
			log.DebugContext(ctx, "Failed to reject SSH channel request",
				"error", err)
			closeConnections()
		}
		return
	}

	// Now that the target accepted the channel, accept the incoming channel
	// request.
	incomingChan, incomingChanRequests, err := newChan.Accept()
	if err != nil {
		// Failing to accept an incoming channel request that the target already
		// accepted is fatal. Log, close the channel we just opened on the
		// target, and kill the connection.
		log.DebugContext(ctx, "Failed to accept SSH channel request already accepted by the target, killing the connection",
			"error", err)
		if err := targetChan.Close(); err != nil {
			log.DebugContext(ctx, "Failed to close SSH channel on target",
				"error", err)
		}
		closeConnections()
		return
	}

	// Use 2 goroutines to proxy channel requests bidirectionally. The
	// goroutines will terminate after the channels has been closed, and either
	// will close the channels upon any unrecoverable error.
	closeChannels := func() {
		incomingChan.Close()
		targetChan.Close()
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		proxyChannelRequests(ctx, log, targetChan, incomingChanRequests, closeChannels)
	}()
	go func() {
		defer wg.Done()
		proxyChannelRequests(ctx, log, incomingChan, targetChanRequests, closeChannels)
	}()

	// Proxy channel data bidirectionally.
	if err := utils.ProxyConn(ctx, incomingChan, targetChan); err != nil &&
		!utils.IsOKNetworkError(err) && !errors.Is(err, context.Canceled) {
		log.DebugContext(ctx, "Unexpected error proxying channel data", "error", err)
	}

	// utils.ProxyConn will close both channels before returning, causing the
	// channel request goroutines to terminate.
	wg.Wait()
}

func proxyChannelRequests(
	ctx context.Context,
	log *slog.Logger,
	targetChan ssh.Channel,
	reqs <-chan *ssh.Request,
	closeChannels func(),
) {
	log = log.With("request_layer", "channel")
	sendRequest := func(name string, wantReply bool, payload []byte) (bool, []byte, error) {
		ok, err := targetChan.SendRequest(name, wantReply, payload)
		// Replies to channel requests never have a payload.
		return ok, nil, err
	}
	proxyRequests(ctx, log, sendRequest, reqs, closeChannels)
}

func proxyGlobalRequests(
	ctx context.Context,
	targetConn ssh.Conn,
	reqs <-chan *ssh.Request,
	closeConnections func(),
) {
	log := log.With("request_layer", "global")
	sendRequest := targetConn.SendRequest
	proxyRequests(ctx, log, sendRequest, reqs, closeConnections)
}

func proxyRequests(
	ctx context.Context,
	log *slog.Logger,
	sendRequest func(name string, wantReply bool, payload []byte) (bool, []byte, error),
	reqs <-chan *ssh.Request,
	closeRequestSources func(),
) {
	for req := range reqs {
		log := log.With("request_type", req.Type)
		log.DebugContext(ctx, "Proxying SSH request")
		ok, reply, err := sendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			// We failed to send the request, the target must be dead.
			log.DebugContext(ctx, "Failed to forward SSH request", "request_type", req.Type, "error", err)
			// We must first send a reply if one was expected.
			if req.WantReply {
				req.Reply(false, nil)
			}
			// Close both connections or channels to clean up but we must continue handling
			// requests on the chan until it is closed by crypto/ssh.
			closeRequestSources()
			continue
		}
		if !req.WantReply {
			continue
		}
		if err := req.Reply(ok, reply); err != nil {
			// A reply was expected and returned by the target but we failed to
			// forward it back, the connection that initiated the request must
			// be dead.
			log.DebugContext(ctx, "Failed to reply to SSH request", "request_type", req.Type, "error", err)
			// Close both connections or channels to clean up but we must continue handling
			// requests on the chan until it is closed by crypto/ssh.
			closeRequestSources()
		}
	}
	// We get here when the source SSH connection or channel was closed, make
	// sure the opposite one is closed too.
	closeRequestSources()
}
