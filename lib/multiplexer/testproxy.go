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

package multiplexer

import (
	"context"
	"io"
	"log/slog"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// TestProxy is tcp passthrough proxy that sends a proxy-line when connecting
// to the target server.
type TestProxy struct {
	listener net.Listener
	target   string
	closeCh  chan (struct{})
	log      *slog.Logger
	v2       bool
}

// NewTestProxy creates a new test proxy that sends a proxy-line when
// proxying connections to the provided target address.
func NewTestProxy(target string, v2 bool) (*TestProxy, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &TestProxy{
		listener: listener,
		target:   target,
		closeCh:  make(chan struct{}),
		log:      utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "test:proxy"),
		v2:       v2,
	}, nil
}

// Address returns the proxy listen address.
func (p *TestProxy) Address() string {
	return p.listener.Addr().String()
}

// Serve starts accepting client connections and proxying them to the target.
func (p *TestProxy) Serve() error {
	for {
		clientConn, err := p.listener.Accept()
		if err != nil {
			return trace.Wrap(err)
		}
		p.log.DebugContext(context.Background(), "Accepted connection", "remote_addr", logutils.StringerAttr(clientConn.RemoteAddr()))
		go func() {
			if err := p.handleConnection(clientConn); err != nil {
				p.log.ErrorContext(context.Background(), "Failed to handle connection", "error", err)
			}
		}()
	}
}

// handleConnection dials the target address, sends a proxy line to it and
// then starts proxying all traffic b/w client and target.
func (p *TestProxy) handleConnection(clientConn net.Conn) error {
	serverConn, err := net.Dial("tcp", p.target)
	if err != nil {
		clientConn.Close()
		return trace.Wrap(err)
	}
	defer serverConn.Close()
	errCh := make(chan error, 2)
	go func() { // Client -> server.
		defer clientConn.Close()
		defer serverConn.Close()
		// Write proxy-line first and then start proxying from client.
		err := p.sendProxyLine(clientConn, serverConn)
		if err == nil {
			_, err = io.Copy(serverConn, clientConn)
		}
		errCh <- trace.Wrap(err)
	}()
	go func() { // Server -> client.
		defer clientConn.Close()
		defer serverConn.Close()
		_, err := io.Copy(clientConn, serverConn)
		errCh <- trace.Wrap(err)
	}()
	var errs []error
	for range 2 {
		select {
		case err := <-errCh:
			if err != nil && !utils.IsOKNetworkError(err) {
				errs = append(errs, err)
			}
		case <-p.closeCh:
			p.log.DebugContext(context.Background(), "Closing")
			return trace.NewAggregate(errs...)
		}
	}
	return trace.NewAggregate(errs...)
}

// sendProxyLine sends proxy-line to the server.
func (p *TestProxy) sendProxyLine(clientConn, serverConn net.Conn) error {
	clientAddr, err := utils.ParseAddr(clientConn.RemoteAddr().String())
	if err != nil {
		return trace.Wrap(err)
	}
	serverAddr, err := utils.ParseAddr(serverConn.RemoteAddr().String())
	if err != nil {
		return trace.Wrap(err)
	}
	proxyLine := &ProxyLine{
		Protocol:    TCP4,
		Source:      net.TCPAddr{IP: net.ParseIP(clientAddr.Host()), Port: clientAddr.Port(0)},
		Destination: net.TCPAddr{IP: net.ParseIP(serverAddr.Host()), Port: serverAddr.Port(0)},
	}
	p.log.DebugContext(context.Background(), "Sending proxy line",
		"proxy_line", proxyLine.String(),
		"remote_addr", serverConn.RemoteAddr().String(),
	)
	if p.v2 {
		b, bErr := proxyLine.Bytes()
		if bErr != nil {
			return trace.Wrap(err)
		}
		_, err = serverConn.Write(b)
	} else {
		_, err = serverConn.Write([]byte(proxyLine.String()))
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close closes the proxy listener.
func (p *TestProxy) Close() error {
	close(p.closeCh)
	return p.listener.Close()
}
