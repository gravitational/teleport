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

// Close closes the connection and drains all channels.
func (c *sshConn) Close() error {
	err := trace.Wrap(c.conn.Close())
	go ssh.DiscardRequests(c.reqs)
	for newChan := range c.chans {
		newChan.Reject(0, "")
	}
	return err
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
	closeConnections := sync.OnceFunc(func() {
		clientConn.Close()
		serverConn.Close()
	})
	// Close both connections if the context is canceled.
	stop := context.AfterFunc(ctx, closeConnections)
	defer stop()

	// Avoid leaking goroutines by tracking them with a waitgroup.
	// If any task exits make sure to close both connections so that all other
	// tasks can terminate.
	var wg sync.WaitGroup
	runTask := func(task func()) {
		wg.Add(1)
		go func() {
			task()
			closeConnections()
			wg.Done()
		}()
	}

	// Proxy channels initiated by either connection.
	runTask(func() {
		proxyChannels(ctx, serverConn.conn, clientConn.chans, closeConnections)
	})
	runTask(func() {
		proxyChannels(ctx, clientConn.conn, serverConn.chans, closeConnections)
	})

	// Proxy global requests in both directions.
	runTask(func() {
		proxyGlobalRequests(ctx, serverConn.conn, clientConn.reqs, closeConnections)
	})
	runTask(func() {
		proxyGlobalRequests(ctx, clientConn.conn, serverConn.reqs, closeConnections)
	})

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
		// Failed to open the channel on the target, newChan must be rejected.
		var (
			rejectionReason  ssh.RejectionReason
			rejectionMessage string
			openChannelErr   *ssh.OpenChannelError
		)
		if errors.As(err, &openChannelErr) {
			// The target rejected the channel, this is totally expected.
			rejectionReason = openChannelErr.Reason
			rejectionMessage = openChannelErr.Message
		} else {
			// We got an unexpected error type trying to open the channel on the
			// target, this is fatal, log and kill the connection.
			log.DebugContext(ctx, "Unexpected error opening SSH channel on target",
				"error", err)
			closeConnections()
			// newChan still has to be rejected below to satisfy the crypto/ssh
			// API, but the underlying network connection is already closed so
			// we just leave the reason and message empty.
		}
		if err := newChan.Reject(rejectionReason, rejectionMessage); err != nil {
			// Failed to reject the incoming channel, this is fatal, log and
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
		// accepted is fatal. Kill the connection, close the channel we
		// just opened on the target and drain the request channel.
		log.DebugContext(ctx, "Failed to accept SSH channel request already accepted by the target, killing the connection",
			"error", err)
		closeConnections()
		go ssh.DiscardRequests(targetChanRequests)
		_ = targetChan.Close()
		return
	}

	// Copy channel requests in both directions concurrently. If either fails or
	// exits it will cancel the context so that utils.ProxyConn below will close
	// both channels so the other goroutine can also exit.
	var wg sync.WaitGroup
	wg.Add(2)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		proxyChannelRequests(ctx, log, targetChan, incomingChanRequests, cancel)
		cancel()
		wg.Done()
	}()
	go func() {
		proxyChannelRequests(ctx, log, incomingChan, targetChanRequests, cancel)
		cancel()
		wg.Done()
	}()

	// ProxyConn copies channel data bidirectionally. If the context is
	// canceled it will terminate, it always closes both channels before
	// returning.
	if err := utils.ProxyConn(ctx, incomingChan, targetChan); err != nil &&
		!utils.IsOKNetworkError(err) && !errors.Is(err, context.Canceled) {
		log.DebugContext(ctx, "Unexpected error proxying channel data", "error", err)
	}

	// Wait for all goroutines to terminate.
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
			// Close both connections or channels to clean up but we must
			// continue handling requests on the chan until it is closed by
			// crypto/ssh.
			closeRequestSources()
			_ = req.Reply(false, nil)
			continue
		}
		if err := req.Reply(ok, reply); err != nil {
			// A reply was expected and returned by the target but we failed to
			// forward it back, the connection that initiated the request must
			// be dead.
			log.DebugContext(ctx, "Failed to reply to SSH request", "request_type", req.Type, "error", err)
			// Close both connections or channels to clean up but we must
			// continue handling requests on the chan until it is closed by
			// crypto/ssh.
			closeRequestSources()
		}
	}
}
