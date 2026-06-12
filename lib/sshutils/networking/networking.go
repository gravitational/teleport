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
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

// RequestBufferSize is the maximum amount of data we're comfortable writing at
// once through a default unix datagram socket. Should fit comfortably in both
// linux and darwin with default settings.
const RequestBufferSize = 1024

// Process represents an instance of a networking process.
type Process struct {
	// cmd is the running process command.
	cmd *exec.Cmd
	// conn is the socket used to request a dialer or listener in the process.
	conn *net.UnixConn
	// done signals when the process completes.
	done chan struct{}
	// killed is set to true when the process was killed forcibly.
	killed atomic.Bool
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
	// XauthFile is an optional XauthFile to use instead of the default ~/.Xauthority. Used in tests.
	XauthFile string
}

// NewProcess starts a new networking process with the given command, which should
// be pre-configured from a ssh server context with Teleport reexec settings.
func NewProcess(ctx context.Context, cmd *exec.Cmd) (*Process, error) {
	// Create the socket to communicate over.
	remoteConn, localConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer remoteConn.Close()
	remoteFD, err := remoteConn.File()
	if err != nil {
		localConn.Close()
		return nil, trace.Wrap(err)
	}
	defer remoteFD.Close()
	cmd.ExtraFiles = append(cmd.ExtraFiles, remoteFD)

	// Propagate stderr from the spawned Teleport process to log any errors.
	cmd.Stderr = os.Stderr

	proc := &Process{
		cmd:  cmd,
		conn: localConn,
		done: make(chan struct{}),
	}

	if err := proc.start(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return proc, nil
}

// start the the networking process.
func (p *Process) start(ctx context.Context) error {
	if err := p.cmd.Start(); err != nil {
		p.conn.Close()
		return trace.Wrap(err)
	}

	// The child process writes errors to the parent connection for logging purposes.
	go func() {
		for {
			buf := make([]byte, RequestBufferSize)
			n, err := p.conn.Read(buf)
			if err != nil {
				if utils.IsOKNetworkError(err) {
					return
				}
				slog.WarnContext(ctx, "Failed to read error from networking process.", "error", err)
				return
			}

			if n > 0 {
				slog.WarnContext(ctx, "Received unexpected error from networking process.", "error", string(buf[:n]))
			}
		}
	}()

	go func() {
		defer close(p.done)
		defer p.conn.Close()
		// Ensure unexpected cmd failures get logged.
		if err := p.cmd.Wait(); err != nil && !p.killed.Load() {
			slog.WarnContext(ctx, "Networking process exited early with unexpected error.", "error", err)
		}
	}()

	return nil
}

// Close stops the process and frees up its related resources.
func (p *Process) Close() error {
	p.conn.Close()
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		slog.WarnContext(context.Background(), "Killing networking subprocess.")

		// Kill the process and wait for it to successfully terminate.
		p.killed.Store(true)
		p.cmd.Process.Kill()
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			// Wait for the kill signal to result in the termination of process, otherwise tests
			// that create a temporary user may fail to delete the user at the end of the test
			// while the kill signal is propagating.
			slog.WarnContext(context.Background(), "Networking subprocess still running after kill signal.")
		}
	}
	return nil
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
	defer connFD.Close()

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
	defer listenerFD.Close()

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
	defer listenerFD.Close()

	listener, err := net.FileListener(listenerFD)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return listener, trace.Wrap(err)
}

// ListenX11 requests a local XServer listener from the networking subprocess.
func (p *Process) ListenX11(ctx context.Context, req X11Request) (net.Listener, error) {
	listenerFD, err := p.sendRequest(ctx, Request{
		Operation:  NetworkingOperationListenX11,
		X11Request: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer listenerFD.Close()

	listener, err := net.FileListener(listenerFD)
	return listener, trace.Wrap(err)
}

// sendRequest sends a networking request to the networking process and waits
// for a file corresponding to an open network connection or listener.
func (p *Process) sendRequest(ctx context.Context, req Request) (*os.File, error) {
	ctx, cancel := context.WithTimeout(ctx, defaults.DefaultIOTimeout)
	defer cancel()

	slog.DebugContext(ctx, "Sending networking request to child process", "request", req)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We can use a stream instead of a datagram because we only expect bytes
	// or a file descriptor in response, not both.
	requestConn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeStream)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Close the request stream once the context is closed or the process is closed.
	// This avoids a race condition where the process is closed mid request and thus
	// fails to close the stream, resulting in a deadlock on read below.
	go func() {
		defer requestConn.Close()
		select {
		case <-ctx.Done():
		case <-p.done:
		}
	}()

	remoteFD, err := remoteConn.File()
	remoteConn.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer remoteFD.Close()

	if _, _, err = uds.WriteWithFDs(p.conn, jsonReq, []*os.File{remoteFD}); err != nil {
		return nil, trace.Wrap(err)
	}

	file, err := readResponse(requestConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return file, nil
}

// readResponse attempts to read a file descriptor from the given connection until it is closed.
func readResponse(conn *net.UnixConn) (*os.File, error) {
	buf := make([]byte, RequestBufferSize)
	fbuf := make([]*os.File, 1)
	n, fn, err := uds.ReadWithFDs(conn, buf, fbuf)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if fn == 0 {
		if n > 0 {
			// The networking process only ever writes to the request conn if an error occurs.
			// Read the rest of the connection to ensure we don't return just a partial stream.
			errMsg, err := io.ReadAll(io.LimitReader(conn, int64(cap(buf)-len(buf))))
			if err != nil {
				return nil, trace.Wrap(err)
			}

			errMsg = append(buf[:n], errMsg...)
			return nil, trace.Errorf("error returned by networking process: %v", string(errMsg))
		}
		return nil, trace.BadParameter("networking process did not return a listener")
	}

	return fbuf[0], nil
}
