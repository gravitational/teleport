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

package networking

import (
	"context"
	"encoding/json"
	"io"
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
	// Done signals when the process completes.
	Done <-chan struct{}
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

// sendRequest sends a networking request to the networking process and waits
// for a file corresponding to an open network connection or listener.
func (p *Process) sendRequest(ctx context.Context, req Request) (*os.File, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	localConn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer remoteConn.Close()
	defer localConn.Close()

	remoteFD, err := remoteConn.File()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer remoteFD.Close()

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, _, err = uds.WriteWithFDs(p.Conn, jsonReq, []*os.File{remoteFD}); err != nil {
		return nil, trace.Wrap(err)
	}

	fileCh := make(chan *os.File)
	errC := make(chan error)
	// Read in another goroutine so we can cancel it if the networking process stops.
	go func() {
		defer close(fileCh)

		buf := make([]byte, 1024)
		fbuf := make([]*os.File, 1)
		n, fn, err := uds.ReadWithFDs(localConn, buf, fbuf)
		if err != nil {
			errC <- err
		} else if fn == 0 {
			if n > 0 {
				// the networking process only ever writes to the request control
				// conn if an error occurs.
				errC <- trace.Errorf("error returned by networking process: %v", string(buf[:n]))
			} else {
				errC <- trace.BadParameter("networking process did not return a listener")
			}
		}

		select {
		case fileCh <- fbuf[0]:
		case <-p.Done:
			fbuf[0].Close()
		}
	}()

	select {
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	case <-p.Done:
		return nil, trace.Errorf("networking process is closed")
	case err := <-errC:
		return nil, trace.Wrap(err)
	case listenerFD := <-fileCh:
		return listenerFD, nil
	}
}
