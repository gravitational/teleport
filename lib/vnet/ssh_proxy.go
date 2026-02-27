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
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const (
	moshIPRequestType  = "mosh-ip@goteleport.com"
	moshNodeIDReqType  = "mosh-node-id@goteleport.com"
	moshTokenReqType   = "mosh-token@goteleport.com"
	moshProxyIDReqType = "mosh-proxy-id@goteleport.com"
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
	moshAttemptReporter moshAttemptReporter,
	openMoshTunnel func(context.Context, string, string, string, *net.UDPAddr) (net.Conn, error),
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

	// Proxy global requests in both directions.
	runTask(func() {
		proxyGlobalRequests(ctx, serverConn.conn, clientConn.reqs, closeConnections)
	})
	runTask(func() {
		proxyGlobalRequests(ctx, clientConn.conn, serverConn.reqs, closeConnections)
	})

	reportMoshAttempt := func(port uint16) {
		success, reply, err := clientConn.conn.SendRequest(moshIPRequestType, true, nil)
		if err != nil || !success {
			log.WarnContext(ctx, "Mosh IP request failed", "error", err, "success", success)
			return
		}
		if len(reply) == 0 {
			log.WarnContext(ctx, "Mosh IP request returned an empty IP")
			return
		}
		nodeSuccess, nodeReply, err := clientConn.conn.SendRequest(moshNodeIDReqType, true, nil)
		if err != nil || !nodeSuccess || len(nodeReply) == 0 {
			log.WarnContext(ctx, "Mosh node ID request failed", "error", err, "success", nodeSuccess)
			return
		}
		nodeID := string(nodeReply)
		remoteIP := net.IP(append([]byte(nil), reply...))
		var tokenPayload [2]byte
		binary.BigEndian.PutUint16(tokenPayload[:], port)
		tokenSuccess, tokenReply, err := clientConn.conn.SendRequest(moshTokenReqType, true, tokenPayload[:])
		if err != nil || !tokenSuccess || len(tokenReply) == 0 {
			log.WarnContext(ctx, "Mosh token request failed", "error", err, "success", tokenSuccess)
			return
		}
		token := string(tokenReply)
		proxySuccess, proxyReply, err := clientConn.conn.SendRequest(moshProxyIDReqType, true, nil)
		if err != nil || !proxySuccess || len(proxyReply) == 0 {
			log.WarnContext(ctx, "Mosh proxy ID request failed", "error", err, "success", proxySuccess)
			return
		}
		proxyID := string(proxyReply)
		log.DebugContext(ctx, "Mosh IP request succeeded", "port", port)
		attempt := moshAttempt{
			ip:      append([]byte(nil), reply...),
			port:    port,
			nodeID:  nodeID,
			token:   token,
			proxyID: proxyID,
		}
		if openMoshTunnel != nil {
			attempt.openTunnel = func(ctx context.Context) (net.Conn, error) {
				return openMoshTunnel(ctx, nodeID, token, proxyID, &net.UDPAddr{IP: remoteIP, Port: int(port)})
			}
		}
		moshAttemptReporter.reportMoshAttempt(moshAttempt{
			ip:         attempt.ip,
			port:       attempt.port,
			nodeID:     attempt.nodeID,
			token:      attempt.token,
			proxyID:    attempt.proxyID,
			openTunnel: attempt.openTunnel,
		})
	}

	// Proxy channels initiated by either connection.
	runTask(func() {
		proxyChannels(ctx, serverConn.conn, clientConn.chans, closeConnections, reportMoshAttempt)
	})
	runTask(func() {
		proxyChannels(ctx, clientConn.conn, serverConn.chans, closeConnections, reportMoshAttempt)
	})

	wg.Wait()
}

func proxyChannels(
	ctx context.Context,
	targetConn ssh.Conn,
	chans <-chan ssh.NewChannel,
	closeConnections func(),
	reportMoshAttempt func(uint16),
) {
	// Proxy each SSH channel in its own goroutine, make sure they don't leak by
	// tracking with a WaitGroup.
	var wg sync.WaitGroup
	for newChan := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proxyChannel(ctx, targetConn, newChan, closeConnections, reportMoshAttempt)
		}()
	}
	wg.Wait()
}

func proxyChannel(
	ctx context.Context,
	targetConn ssh.Conn,
	newChan ssh.NewChannel,
	closeConnections func(),
	reportMoshAttempt func(uint16),
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

	// Copy channel data and requests from the incoming channel to the target
	// channel, and vice-versa.
	target := newSSHChan(targetChan, targetChanRequests, slog.With("direction", "client->target"), reportMoshAttempt)
	incoming := newSSHChan(incomingChan, incomingChanRequests, slog.With("direction", "target->client"), reportMoshAttempt)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		target.writeFrom(ctx, incoming)
		wg.Done()
	}()
	go func() {
		incoming.writeFrom(ctx, target)
		wg.Done()
	}()
	wg.Wait()
}

// sshChan manages all writes to an SSH channel and handles closing the channel
// once no more data or requests will be written to it.
type sshChan struct {
	ch                ssh.Channel
	requests          <-chan *ssh.Request
	log               *slog.Logger
	reportMoshAttempt func(uint16)
}

func newSSHChan(ch ssh.Channel, requests <-chan *ssh.Request, log *slog.Logger, reportMoshAttempt func(uint16)) *sshChan {
	return &sshChan{
		ch:                ch,
		requests:          requests,
		log:               log,
		reportMoshAttempt: reportMoshAttempt,
	}
}

// writeFrom writes channel data and requests from the source to this SSH channel.
//
// In the happy path it waits for:
// - channel data reads from source to return EOF
// - the source request channel to be closed
// and then closes this channel.
//
// Channel data reads from source can return EOF at any time if it has sent
// SSH_MSG_CHANNEL_EOF but it is still valid to send more channel requests
// after this.
//
// If an unrecoverable error is encountered it immediately closes both
// channels.
func (c *sshChan) writeFrom(ctx context.Context, source *sshChan) {
	// Close the channel after all data and request writes are complete.
	defer c.ch.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		c.writeDataFrom(ctx, source)
		wg.Done()
	}()
	go func() {
		c.writeRequestsFrom(ctx, source)
		wg.Done()
	}()
	wg.Wait()
}

// writeDataFrom writes channel data from source to this SSH channel.
// It handles standard channel data and extended channel data of type stderr.
func (c *sshChan) writeDataFrom(ctx context.Context, source *sshChan) {
	// Close the channel for writes only after both the standard and stderr
	// streams are finished writing.
	defer c.ch.CloseWrite()

	moshDetector := newMoshDetector(c.reportMoshAttempt)

	errors := make(chan error, 2)
	go func() {
		r := io.TeeReader(source.ch, moshDetector)
		_, err := io.Copy(c.ch, r)
		errors <- err
	}()
	go func() {
		_, err := io.Copy(c.ch.Stderr(), source.ch.Stderr())
		errors <- err
	}()

	// Read both errors to make sure both goroutines terminate, but only do
	// anything on the first non-nil error, the second error is likely either
	// the same as the first one or caused by closing the channel.
	handledError := false
	for range 2 {
		err := <-errors
		if err != nil && !handledError {
			handledError = true
			// Failed to write channel data from source to this channel. This was
			// not an EOF from source or io.Copy would have returned nil. The
			// stream might be missing data so close both channels.
			//
			// This should also unblock the stderr stream if the regular stream
			// returned an error, and vice-versa.
			c.log.ErrorContext(ctx, "Fatal error proxying SSH channel data", "error", err)
			c.ch.Close()
			source.ch.Close()
		}
	}
}

// writeRequestsFrom forwards channel requests from source to this SSH channel.
func (c *sshChan) writeRequestsFrom(ctx context.Context, source *sshChan) {
	log := c.log.With("request_layer", "channel")
	sendRequest := func(name string, wantReply bool, payload []byte) (bool, []byte, error) {
		ok, err := c.ch.SendRequest(name, wantReply, payload)
		// Replies to channel requests never have a payload.
		return ok, nil, err
	}
	// Must forcibly close both channels if there was a fatal error proxying
	// channel requests so that we don't continue in a bad state.
	onFatalError := func() {
		c.ch.Close()
		source.ch.Close()
	}
	proxyRequests(ctx, log, sendRequest, source.requests, onFatalError)
}

type moshDetector struct {
	data              []byte
	decided           bool
	reportMoshAttempt func(uint16)
}

func newMoshDetector(reportMoshAttempt func(uint16)) *moshDetector {
	return &moshDetector{
		reportMoshAttempt: reportMoshAttempt,
	}
}

func (d *moshDetector) Write(b []byte) (int, error) {
	if d.decided {
		return len(b), nil
	}
	d.data = append(d.data, b...)
	const maxInspectBytes = 4096
	if port, ok := parseMoshConnectPort(d.data); ok {
		d.reportMoshAttempt(port)
		d.decided = true
		return len(b), nil
	}
	if len(d.data) < len("MOSH CONNECT 0") && len(d.data) < maxInspectBytes {
		return len(b), nil
	}
	if len(d.data) > maxInspectBytes {
		d.data = d.data[len(d.data)-maxInspectBytes:]
	}
	return len(b), nil
}

func parseMoshConnectPort(data []byte) (uint16, bool) {
	idx := bytes.Index(data, []byte("MOSH CONNECT "))
	if idx < 0 {
		return 0, false
	}
	start := idx + len("MOSH CONNECT ")
	end := start
	for end < len(data) && data[end] >= '0' && data[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	p, err := strconv.ParseUint(string(data[start:end]), 10, 16)
	if err != nil || p == 0 {
		return 0, false
	}
	return uint16(p), true
}

func proxyGlobalRequests(
	ctx context.Context,
	targetConn ssh.Conn,
	reqs <-chan *ssh.Request,
	onFatalError func(),
) {
	log := log.With("request_layer", "global")
	sendRequest := targetConn.SendRequest
	proxyRequests(ctx, log, sendRequest, reqs, onFatalError)
}

func proxyRequests(
	ctx context.Context,
	log *slog.Logger,
	sendRequest func(name string, wantReply bool, payload []byte) (bool, []byte, error),
	reqs <-chan *ssh.Request,
	onFatalError func(),
) {
	for req := range reqs {
		log := log.With("request_type", req.Type)
		log.DebugContext(ctx, "Proxying SSH request")
		ok, reply, err := sendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			// We failed to send the request, the target must be dead.
			log.DebugContext(ctx, "Failed to forward SSH request", "error", err)
			onFatalError()
			req.Reply(false, nil)
			ssh.DiscardRequests(reqs)
			return
		}
		if req.Type == "exit-status" || req.Type == "exec" {
			log.DebugContext(ctx, "Forwarding SSH request",
				"type", req.Type,
				"payload_len", len(req.Payload),
				"payload", string(req.Payload),
				"reply_payload_len", len(reply),
				"reply_payload", string(reply))
		}
		if err := req.Reply(ok, reply); err != nil {
			// A reply was expected and returned by the target but we failed to
			// forward it back, the connection that initiated the request must
			// be dead.
			log.DebugContext(ctx, "Failed to reply to SSH request", "error", err)
			onFatalError()
			ssh.DiscardRequests(reqs)
			return
		}
	}
}
