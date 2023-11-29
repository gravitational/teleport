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

package tcpip

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/socketpair"
)

// ForwardRequest is the payload sent from dialer to forwarder when starting
// a new forwarding connection.
type ForwardRequest struct {
	// DestinationAddress is the target address to dial to.
	DestinationAddress string `json:"dst_addr"`
}

// ForwardResponse is the payload sent from forwarder to dialer indicating
// success/failure in dialing the target address.
type ForwardResponse struct {
	// Success is true if the target address was successfully dialed.
	Success bool `json:"success"`
	// Error is the error string associated with a failed dial.
	Error string `json:"error"`
}

// NewDialer wraps a [socketpair.Dialer], sending dials as messages understood by the
// corresponding [Forwarder].  Using this type in conjunction with
// the socket pair dialer/listener helpers from utils makes it easy to forward arbitrary
// TCP dials via a socket.
func NewDialer(inner *socketpair.Dialer) *Dialer {
	return &Dialer{
		inner: inner,
	}
}

// Dialer is a tcp dialer that forwards dials across a socket.
type Dialer struct {
	inner *socketpair.Dialer
}

// Dial dials the target address.
func (d *Dialer) Dial(addr string) (net.Conn, error) {
	conn, err := d.inner.Dial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := ForwardRequest{
		DestinationAddress: addr,
	}

	reqBytes, err := utils.FastMarshal(&req)
	if err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to marshal forward request: %v", err)
	}

	if err := WriteLengthPrefixedMessage(conn, reqBytes); err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to send forward request: %v", err)
	}

	rspBytes, err := ReadLengthPrefixedMessage(conn)
	if err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to read forward response: %v", err)
	}

	var rsp ForwardResponse
	if err := utils.FastUnmarshal(rspBytes, &rsp); err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to unmarshal forward response: %v", err)
	}

	if !rsp.Success {
		conn.Close()
		return nil, trace.Errorf("failed to dial forwarding target: %q", err)
	}

	// connection is now ready to forward
	return conn, nil
}

// Close closes the inner dialer.
func (d *Dialer) Close() error {
	d.inner.Close()
	return nil
}

// Forwarder implements the core logic of the forwarding child process. It is designed to
// sit behind a socket, serving tcpip forward requests passed to it from the parent.
type Forwarder struct {
	listener     net.Listener
	closeContext context.Context
	cancel       context.CancelCauseFunc
}

// NewForwarder builds a forwarder. The passed in listener needs to be assoicated with a
// corresponding [Dialer]. Typically used to wrap the listener created by [socketpair.ListenerFromFD].
func NewForwarder(listener net.Listener) *Forwarder {
	ctx, cancel := context.WithCancelCause(context.Background())
	return &Forwarder{
		listener:     listener,
		closeContext: ctx,
		cancel:       cancel,
	}
}

// errForwarderClosed is used to indicate termination due to explicit closure.
var errForwarderClosed = errors.New("socket forwarder closed")

// Close closes the inner listener and terminates all in-progress forwarding.
func (f *Forwarder) Close() error {
	f.listener.Close()
	f.cancel(errForwarderClosed)
	return nil
}

// Run accepts and forwards connections.
func (f *Forwarder) Run() error {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			return trace.Wrap(err)
		}

		go f.forwardConn(conn)
	}
}

func (f *Forwarder) forwardConn(conn net.Conn) {
	defer conn.Close()

	// read the forward request
	reqBytes, err := ReadLengthPrefixedMessage(conn)
	if err != nil {
		logrus.Warnf("Failed to read forward request: %v", err)
		return
	}

	var req ForwardRequest
	if err := utils.FastUnmarshal(reqBytes, &req); err != nil {
		logrus.Warnf("Failed to decode forward request: %v", err)
		return
	}

	if req.DestinationAddress == "" {
		logrus.Warn("Invalid forward request (missing destination address).")
		return
	}

	// dial the destination and build response message
	var rsp ForwardResponse
	fconn, err := net.Dial("tcp", req.DestinationAddress)
	if err != nil {
		rsp.Success = false
		rsp.Error = err.Error()
	} else {
		rsp.Success = true
	}

	rspBytes, err := utils.FastMarshal(&rsp)
	if err != nil {
		logrus.Warnf("Failed to encode forward response: %v", err)
		return
	}

	if err := WriteLengthPrefixedMessage(conn, rspBytes); err != nil {
		if errors.Is(err, io.EOF) {
			return
		}
		logrus.Warnf("Failed to write forward response: %v", err)
		return
	}

	if !rsp.Success {
		// dial failed and failure has been successfully propagated to
		// parent. no more work to do.
		return
	}

	if err := utils.ProxyConn(f.closeContext, conn, fconn); err != nil {
		if utils.IsOKNetworkError(err) || errors.Is(err, errForwarderClosed /*from context.Cause*/) {
			return
		}

		logrus.Warnf("Failure during conn forwarding: %v", err)
		return
	}
}
