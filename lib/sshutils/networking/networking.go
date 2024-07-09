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

// Package networking handles ssh networking requests via the teleport networking subprocess,
// including port forwarding, agent forwarding, and x11 forwarding.
//
// IPC protocol summary:
//   - Start: The parent teleport process creates a unix socket pair and passes one side to the
//     networking subprocess on start. This is used as a unidirectional pipe for the parent
//     to make networking requests.
//   - Request: The parent creates a new request-level socket pair and sends one side through the
//     main pipe, along with the request payload (e.g. dial tcp 8080).
//   - Handle: The subprocess watches for new requests on the main pipe. When a request is received,
//     the subprocess prepares a networking file matching the request (e.g. tcp conn file) and writes
//     it (or an error) to the request-level socket.
//   - Response: The parent reads the networking file from the request-level socket, keeping the file
//     and closing the request-level socket.
package networking

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils/uds"
)

// Process represents an instance of a networking process.
type Process struct {
	// Conn is the socket used to request a dialer or listener in the process.
	Conn *net.UnixConn
	// Closer contains and extra io.Closer to run when the process as a whole
	// is closed.
	Closer io.Closer
}

// Request is a networking request.
type Request struct {
	// Operation is a networking operation.
	Operation Operation
	// Network is a network type.
	Network string
	// Address is a network address.
	Address string
	// X11Request contains additional info needed for x11 networking requests.
	X11Request X11Request
}

// Operation is a networking operation.
type Operation string

const (
	// NetworkingOperationDial is used to connect to a network address.
	NetworkingOperationDial Operation = "dial"
	// NetworkingOperationListen is used to create a local network listener.
	NetworkingOperationListen Operation = "listen"
	// NetworkingOperationListenAgent is used to create a local ssh-agent listener.
	NetworkingOperationListenAgent Operation = "listen-agent"
	// NetworkingOperationListenX11 is used to create a local x11 listener.
	NetworkingOperationListenX11 Operation = "listen-x11"
)

// X11Config contains information used by the child process to set up X11 forwarding.
type X11Request struct {
	x11.ForwardRequestPayload
	// DisplayOffset is the first display that we should try to get a unix socket for.
	DisplayOffset int
	// MaxDisplay is the last display that we should try to get a unix socket for, if all
	// displays before it are taken.
	MaxDisplay int
}

// Close stops the process and frees up its related resources.
func (p *Process) Close() error {
	var errs []error
	if p.Conn != nil {
		errs = append(errs, p.Conn.Close())
	}
	if p.Closer != nil {
		errs = append(errs, p.Closer.Close())
	}
	return trace.NewAggregate(errs...)
}

// Dial requests a network connection from the networking subprocess.
func (p *Process) Dial(ctx context.Context, network string, addr string) (net.Conn, error) {
	connFD, err := p.sendRequest(ctx, Request{
		Operation: NetworkingOperationDial,
		Network:   network,
		Address:   addr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := net.FileConn(connFD)
	return conn, trace.Wrap(err)
}

// Listen requests a local listener from the networking subprocess.
func (p *Process) Listen(ctx context.Context, network string, addr string) (net.Listener, error) {
	listenerFD, err := p.sendRequest(ctx, Request{
		Operation: NetworkingOperationListen,
		Network:   network,
		Address:   addr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateListenerSocket(listenerFD); err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.FileListener(listenerFD)
	return listener, trace.Wrap(err)
}

// ListenAgent requests a local ssh-agent listener from the networking subprocess.
func (p *Process) ListenAgent(ctx context.Context) (net.Listener, error) {
	listenerFD, err := p.sendRequest(ctx, Request{
		Operation: NetworkingOperationListenAgent,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.FileListener(listenerFD)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, trace.BadParameter("expected *net.UnixListener but got %T", listener)
	}

	unixListener.SetUnlinkOnClose(true)
	return unixListener, nil
}

// ListenAgent requests a local ssh-agent listener from the networking subprocess.
func (p *Process) ListenX11(ctx context.Context, req X11Request) (*net.UnixListener, error) {
	listenerFD, err := p.sendRequest(ctx, Request{
		Operation:  NetworkingOperationListenX11,
		X11Request: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.FileListener(listenerFD)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, trace.BadParameter("expected *net.UnixListener but got %T", listener)
	}

	unixListener.SetUnlinkOnClose(true)
	return unixListener, nil
}

const requestBufferSize = 1024

// sendRequest sends a networking request to the networking process and waits
// for a file corresponding to an open network connection or listener.
func (p *Process) sendRequest(ctx context.Context, req Request) (*os.File, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	slog.With("request", req).Debug("Sending networking request to child process")

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localConn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	context.AfterFunc(ctx, func() { localConn.Close() })

	remoteFD, err := remoteConn.File()
	if err != nil {
		remoteConn.Close()
		return nil, trace.Wrap(err)
	}
	remoteConn.Close()

	if _, _, err = uds.WriteWithFDs(p.Conn, jsonReq, []*os.File{remoteFD}); err != nil {
		remoteFD.Close()
		return nil, trace.Wrap(err)
	}
	remoteFD.Close()

	buf := make([]byte, requestBufferSize)
	fbuf := make([]*os.File, 1)
	n, fn, err := uds.ReadWithFDs(localConn, buf, fbuf)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if fn == 0 {
		if n > 0 {
			// the networking process only ever writes to the request conn if an error occurs.
			return nil, trace.Errorf("error returned by networking process: %v", string(buf[:n]))
		}
		return nil, trace.BadParameter("networking process did not return a listener")
	}

	return fbuf[0], nil
}
