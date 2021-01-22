/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package multiplexer implements SSH and TLS multiplexing
// on the same listener
//
// mux, _ := multiplexer.New(Config{Listener: listener})
// mux.SSH() // returns listener getting SSH connections
// mux.TLS() // returns listener getting TLS connections
//
package multiplexer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// Config is a multiplexer config
type Config struct {
	// Listener is listener to multiplex connection on
	Listener net.Listener
	// Context is a context to signal stops, cancellations
	Context context.Context
	// ReadDeadline is a connection read deadline,
	// set to defaults.ReadHeadersTimeout if unspecified
	ReadDeadline time.Duration
	// Clock is a clock to override in tests, set to real time clock
	// by default
	Clock clockwork.Clock
	// EnableProxyProtocol enables proxy protocol
	EnableProxyProtocol bool
	// DisableSSH disables SSH socket
	DisableSSH bool
	// DisableTLS disables TLS socket
	DisableTLS bool
	// DisableDB disables database access proxy listener
	DisableDB bool
	// ID is an identifier used for debugging purposes
	ID string
}

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Listener == nil {
		return trace.BadParameter("missing parameter Listener")
	}
	if c.Context == nil {
		c.Context = context.TODO()
	}
	if c.ReadDeadline == 0 {
		c.ReadDeadline = defaults.ReadHeadersTimeout
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// New returns a new instance of multiplexer
func New(cfg Config) (*Mux, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.Context)
	waitContext, waitCancel := context.WithCancel(context.TODO())
	return &Mux{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.Component("mx", cfg.ID),
		}),
		Config:      cfg,
		context:     ctx,
		cancel:      cancel,
		sshListener: newListener(ctx, cfg.Listener.Addr()),
		tlsListener: newListener(ctx, cfg.Listener.Addr()),
		dbListener:  newListener(ctx, cfg.Listener.Addr()),
		waitContext: waitContext,
		waitCancel:  waitCancel,
	}, nil
}

// Mux supports having both SSH and TLS on the same listener socket
type Mux struct {
	sync.RWMutex
	*log.Entry
	Config
	listenerClosed bool
	sshListener    *Listener
	tlsListener    *Listener
	dbListener     *Listener
	context        context.Context
	cancel         context.CancelFunc
	waitContext    context.Context
	waitCancel     context.CancelFunc
}

// SSH returns listener that receives SSH connections
func (m *Mux) SSH() net.Listener {
	return m.sshListener
}

// TLS returns listener that receives TLS connections
func (m *Mux) TLS() net.Listener {
	return m.tlsListener
}

// DB returns listener that receives database connections
func (m *Mux) DB() net.Listener {
	return m.dbListener
}

func (m *Mux) isClosed() bool {
	m.RLock()
	defer m.RUnlock()
	return m.listenerClosed
}

func (m *Mux) closeListener() {
	m.Lock()
	defer m.Unlock()
	// propagate close signal to other listeners
	m.cancel()
	if m.Listener == nil {
		return
	}
	if m.listenerClosed {
		return
	}
	m.listenerClosed = true
	m.Listener.Close()
}

// Close closes listener
func (m *Mux) Close() error {
	m.closeListener()
	return nil
}

// Wait waits until listener shuts down and stops accepting new connections
// this is to workaround issue https://github.com/golang/go/issues/10527
// in tests
func (m *Mux) Wait() {
	<-m.waitContext.Done()
}

// Serve is a blocking function that serves on the listening socket
// and accepts requests. Every request is served in a separate goroutine
func (m *Mux) Serve() error {
	defer m.waitCancel()
	backoffTimer := time.NewTicker(5 * time.Second)
	defer backoffTimer.Stop()
	for {
		conn, err := m.Listener.Accept()
		if err == nil {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetKeepAlive(true)
				tcpConn.SetKeepAlivePeriod(3 * time.Minute)
			}
			go m.detectAndForward(conn)
			continue
		}
		if m.isClosed() {
			return nil
		}
		select {
		case <-backoffTimer.C:
			m.Debugf("backoff on accept error: %v", trace.DebugReport(err))
		case <-m.context.Done():
			return nil
		}
	}
}

func (m *Mux) detectAndForward(conn net.Conn) {
	err := conn.SetReadDeadline(m.Clock.Now().Add(m.ReadDeadline))
	if err != nil {
		m.Warning(err.Error())
		conn.Close()
		return
	}
	connWrapper, err := detect(conn, m.EnableProxyProtocol)
	if err != nil {
		if trace.Unwrap(err) != io.EOF {
			m.Warning(trace.DebugReport(err))
		}
		conn.Close()
		return
	}

	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		m.Warning(trace.DebugReport(err))
		conn.Close()
		return
	}

	switch connWrapper.protocol {
	case ProtoTLS:
		if m.DisableTLS {
			m.Debug("Closing TLS connection: TLS listener is disabled.")
			conn.Close()
			return
		}
		select {
		case m.tlsListener.connC <- connWrapper:
		case <-m.context.Done():
			connWrapper.Close()
			return
		}
	case ProtoSSH:
		if m.DisableSSH {
			m.Debug("Closing SSH connection: SSH listener is disabled.")
			conn.Close()
			return
		}
		select {
		case m.sshListener.connC <- connWrapper:
		case <-m.context.Done():
			connWrapper.Close()
			return
		}
	case ProtoHTTP:
		m.Debug("Detected an HTTP request. If this is for a health check, use an HTTPS request instead.")
		conn.Close()
	case ProtoPostgres:
		m.WithField("protocol", connWrapper.protocol).Debug("Detected Postgres client connection.")
		if m.DisableDB {
			m.Debug("Closing Postgres client connection: db proxy listener is disabled.")
			conn.Close()
			return
		}
		select {
		case m.dbListener.connC <- connWrapper:
		case <-m.context.Done():
			connWrapper.Close()
			return
		}
	default:
		// should not get here, handle this just in case
		connWrapper.Close()
		m.Errorf("detected but unsupported protocol: %v", connWrapper.protocol)
	}
}

func detect(conn net.Conn, enableProxyProtocol bool) (*Conn, error) {
	reader := bufio.NewReader(conn)

	// the first attempt is to parse optional proxy
	// protocol line that is injected by load balancers
	// before actual protocol traffic flows.
	// if the first attempt encounters proxy it
	// goes to the second pass to do protocol detection
	var proxyLine *ProxyLine
	for i := 0; i < 2; i++ {
		bytes, err := reader.Peek(8)
		if err != nil {
			return nil, trace.Wrap(err, "failed to peek connection")
		}

		proto, err := detectProto(bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch proto {
		case ProtoProxy:
			if !enableProxyProtocol {
				return nil, trace.BadParameter("proxy protocol support is disabled")
			}
			if proxyLine != nil {
				return nil, trace.BadParameter("duplicate proxy line")
			}
			proxyLine, err = ReadProxyLine(reader)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// repeat the cycle to detect the protocol
		case ProtoTLS, ProtoSSH, ProtoHTTP:
			return &Conn{
				protocol:  proto,
				Conn:      conn,
				reader:    reader,
				proxyLine: proxyLine,
			}, nil
		case ProtoPostgres:
			return &Conn{
				protocol: proto,
				Conn:     conn,
				reader:   reader,
			}, nil
		}
	}
	// if code ended here after two attempts, something is wrong
	return nil, trace.BadParameter("unknown protocol")
}

const (
	// ProtoUnknown is for unknown protocol
	ProtoUnknown = iota
	// ProtoTLS is TLS protocol
	ProtoTLS
	// ProtoSSH is SSH protocol
	ProtoSSH
	// ProtoProxy is a HAProxy proxy line protocol
	ProtoProxy
	// ProtoHTTP is HTTP protocol
	ProtoHTTP
	// ProtoPostgres is PostgreSQL wire protocol
	ProtoPostgres
)

var (
	proxyPrefix = []byte{'P', 'R', 'O', 'X', 'Y'}
	sshPrefix   = []byte{'S', 'S', 'H'}
	tlsPrefix   = []byte{0x16}
)

// This section defines Postgres wire protocol messages detected by Teleport:
//
// https://www.postgresql.org/docs/13/protocol-message-formats.html
var (
	// postgresSSLRequest is always sent first by a Postgres client (e.g. psql)
	// to check whether the server supports TLS.
	postgresSSLRequest = []byte{0x0, 0x0, 0x0, 0x8, 0x4, 0xd2, 0x16, 0x2f}
	// postgresCancelRequest is sent when a Postgres client requests
	// cancellation of a long-running query.
	//
	// TODO(r0mant): It is currently unsupported because it is sent over a
	// separate plain connection, but we're detecting it anyway so it at
	// least appears in the logs as "unsupported" for debugging.
	postgresCancelRequest = []byte{0x0, 0x0, 0x0, 0x10, 0x4, 0xd2, 0x16, 0x2e}
)

// isHTTP returns true if the first 3 bytes of the prefix indicate
// the use of an HTTP method.
func isHTTP(in []byte) bool {
	methods := [...][]byte{
		[]byte("GET"),
		[]byte("POST"),
		[]byte("PUT"),
		[]byte("DELETE"),
		[]byte("HEAD"),
		[]byte("CONNECT"),
		[]byte("OPTIONS"),
		[]byte("TRACE"),
		[]byte("PATCH"),
	}
	for _, verb := range methods {
		// we only get 3 bytes, so can only compare the first 3 bytes of each verb
		if bytes.HasPrefix(verb, in[:3]) {
			return true
		}
	}
	return false
}

func detectProto(in []byte) (int, error) {
	switch {
	// reader peeks only 3 bytes, slice the longer proxy prefix
	case bytes.HasPrefix(in, proxyPrefix[:3]):
		return ProtoProxy, nil
	case bytes.HasPrefix(in, sshPrefix):
		return ProtoSSH, nil
	case bytes.HasPrefix(in, tlsPrefix):
		return ProtoTLS, nil
	case isHTTP(in):
		return ProtoHTTP, nil
	case bytes.HasPrefix(in, postgresSSLRequest), bytes.HasPrefix(in, postgresCancelRequest):
		return ProtoPostgres, nil
	default:
		return ProtoUnknown, trace.BadParameter("multiplexer failed to detect connection protocol, first few bytes were: %#v", in)
	}
}
