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
	"bufio"
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Conn is a connection wrapper that supports
// communicating remote address from proxy protocol
// and replays first several bytes read during
// protocol detection
type Conn struct {
	net.Conn
	protocol  Protocol
	proxyLine *ProxyLine
	reader    *bufio.Reader
}

// NewConn returns a net.Conn wrapper that supports peeking into the connection.
func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

// NetConn returns the underlying net.Conn.
func (c *Conn) NetConn() net.Conn {
	return c.Conn
}

// Read reads from connection
func (c *Conn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// Peek is [*bufio.Reader.Peek].
func (c *Conn) Peek(n int) ([]byte, error) {
	return c.reader.Peek(n)
}

// Discard is [*bufio.Reader.Discard].
func (c *Conn) Discard(n int) (discarded int, err error) {
	return c.reader.Discard(n)
}

// ReadByte is [*bufio.Reader.ReadByte].
func (c *Conn) ReadByte() (byte, error) {
	return c.reader.ReadByte()
}

// LocalAddr returns local address of the connection
func (c *Conn) LocalAddr() net.Addr {
	if c.proxyLine != nil {
		return &c.proxyLine.Destination
	}
	return c.Conn.LocalAddr()
}

// RemoteAddr returns remote address of the connection
func (c *Conn) RemoteAddr() net.Addr {
	if c.proxyLine != nil {
		if addr := c.proxyLine.ResolveSource(); addr != nil {
			return addr
		}
	}

	return c.Conn.RemoteAddr()
}

// Protocol returns the detected connection protocol
func (c *Conn) Protocol() Protocol {
	return c.protocol
}

// Detect detects the connection protocol by peeking into the first few bytes.
func (c *Conn) Detect() (Protocol, error) {
	proto, err := detectProto(c.reader)
	if err != nil && !trace.IsBadParameter(err) {
		return ProtoUnknown, trace.Wrap(err)
	}
	return proto, nil
}

// ReadProxyLine reads proxy-line from the connection.
func (c *Conn) ReadProxyLine() (*ProxyLine, error) {
	var proxyLine *ProxyLine
	protocol, err := c.Detect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if protocol == ProtoProxyV2 {
		proxyLine, err = ReadProxyLineV2(c.reader)
	} else {
		proxyLine, err = ReadProxyLine(c.reader)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.proxyLine = proxyLine
	return proxyLine, nil
}

// returns a Listener that pretends to be listening on addr, closed whenever the
// parent context is done.
func newListener(parent context.Context, addr net.Addr) *Listener {
	context, cancel := context.WithCancel(parent)
	return &Listener{
		addr:    addr,
		connC:   make(chan net.Conn),
		cancel:  cancel,
		context: context,
	}
}

// Listener is a listener that receives
// connections from multiplexer based on the connection type
type Listener struct {
	addr    net.Addr
	connC   chan net.Conn
	cancel  context.CancelFunc
	context context.Context
}

// Addr returns listener addr, the address of multiplexer listener
func (l *Listener) Addr() net.Addr {
	return l.addr
}

// Accept accepts connections from parent multiplexer listener
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case <-l.context.Done():
		return nil, trace.ConnectionProblem(net.ErrClosed, "listener is closed")
	case conn := <-l.connC:
		return conn, nil
	}
}

// HandleConnection injects the connection into the Listener, blocking until the
// context expires, the connection is accepted or the Listener is closed.
func (l *Listener) HandleConnection(ctx context.Context, conn net.Conn) {
	select {
	case <-ctx.Done():
		conn.Close()
	case <-l.context.Done():
		conn.Close()
	case l.connC <- conn:
	}
}

// Close closes the listener.
func (l *Listener) Close() error {
	l.cancel()
	return nil
}

// PROXYEnabledListener wraps provided listener and can receive and apply PROXY headers and then pass connection up the chain.
type PROXYEnabledListener struct {
	cfg Config
	mux *Mux
	net.Listener
}

// NewPROXYEnabledListener creates news instance of PROXYEnabledListener
func NewPROXYEnabledListener(cfg Config) (*PROXYEnabledListener, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	mux, err := New(cfg) // Creating Mux to leverage protocol detection with PROXY headers
	if err != nil {
		return nil, trace.Wrap(err)
	}
	muxListener := mux.SSH()
	go func() {
		if err := mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
			mux.Entry.WithError(err).Error("Mux encountered err serving")
		}
	}()
	pl := &PROXYEnabledListener{
		cfg:      cfg,
		mux:      mux,
		Listener: muxListener,
	}

	return pl, nil
}

func (p *PROXYEnabledListener) Close() error {
	return trace.Wrap(p.mux.Close())
}

// Accept gets connection from the wrapped listener and detects whether we receive PROXY headers on it,
// after first non PROXY protocol detected it returns connection with PROXY addresses applied to it.
func (p *PROXYEnabledListener) Accept() (net.Conn, error) {
	conn, err := p.Listener.Accept()
	return conn, trace.Wrap(err)
}
