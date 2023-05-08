/*
Copyright 2017-2021 Gravitational, Inc.

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
package multiplexer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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
		waitContext: waitContext,
		waitCancel:  waitCancel,
	}, nil
}

// Mux supports having both SSH and TLS on the same listener socket
type Mux struct {
	sync.RWMutex
	*log.Entry
	Config
	sshListener *Listener
	tlsListener *Listener
	dbListener  *Listener
	context     context.Context
	cancel      context.CancelFunc
	waitContext context.Context
	waitCancel  context.CancelFunc
}

// SSH returns listener that receives SSH connections
func (m *Mux) SSH() net.Listener {
	m.Lock()
	defer m.Unlock()
	if m.sshListener == nil {
		m.sshListener = newListener(m.context, m.Config.Listener.Addr())
	}
	return m.sshListener
}

// TLS returns listener that receives TLS connections
func (m *Mux) TLS() net.Listener {
	m.Lock()
	defer m.Unlock()
	if m.tlsListener == nil {
		m.tlsListener = newListener(m.context, m.Config.Listener.Addr())
	}
	return m.tlsListener
}

// DB returns listener that receives database connections
func (m *Mux) DB() net.Listener {
	m.Lock()
	defer m.Unlock()
	if m.dbListener == nil {
		m.dbListener = newListener(m.context, m.Config.Listener.Addr())
	}
	return m.dbListener
}

func (m *Mux) closeListener() {
	m.Lock()
	defer m.Unlock()
	// propagate close signal to other listeners
	m.cancel()
	if m.Listener == nil {
		return
	}
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
		if utils.IsUseOfClosedNetworkError(err) {
			<-m.context.Done()
			return nil
		}
		select {
		case <-m.context.Done():
			return nil
		case <-time.After(5 * time.Second):
			m.WithError(err).Debugf("Backoff on accept error.")
		}
	}
}

// protocolListener returns a registered listener for Protocol proto
// and is safe for concurrent access.
func (m *Mux) protocolListener(proto Protocol) *Listener {
	m.RLock()
	defer m.RUnlock()
	switch proto {
	case ProtoTLS:
		return m.tlsListener
	case ProtoSSH:
		return m.sshListener
	case ProtoPostgres:
		return m.dbListener
	}
	return nil
}

// detectAndForward detects the protocol for conn and forwards to a
// registered protocol listener (SSH, TLS, DB). Connections for a
// protocol without a registered protocol listener are closed. This
// method is called as a goroutine by Serve for each connection.
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
		connWrapper.Close()
		return
	}

	listener := m.protocolListener(connWrapper.protocol)
	if listener == nil {
		if connWrapper.protocol == ProtoHTTP {
			m.Debug("Detected an HTTP request. If this is for a health check, use an HTTPS request instead.")
		}
		m.Debugf("Closing %[1]s connection: %[1]s listener is disabled.", connWrapper.protocol)
		connWrapper.Close()
		return
	}

	listener.HandleConnection(m.context, connWrapper)
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
		proto, err := detectProto(reader)
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
		case ProtoProxyV2:
			if !enableProxyProtocol {
				return nil, trace.BadParameter("proxy protocol support is disabled")
			}
			if proxyLine != nil {
				return nil, trace.BadParameter("duplicate proxy line")
			}
			proxyLine, err = ReadProxyLineV2(reader)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// repeat the cycle to detect the protocol
		case ProtoTLS, ProtoSSH, ProtoHTTP, ProtoPostgres:
			return &Conn{
				protocol:  proto,
				Conn:      conn,
				reader:    reader,
				proxyLine: proxyLine,
			}, nil
		}
	}
	// if code ended here after two attempts, something is wrong
	return nil, trace.BadParameter("unknown protocol")
}

// Protocol defines detected protocol type.
type Protocol int

const (
	// ProtoUnknown is for unknown protocol
	ProtoUnknown Protocol = iota
	// ProtoTLS is TLS protocol
	ProtoTLS
	// ProtoSSH is SSH protocol
	ProtoSSH
	// ProtoProxy is a HAProxy proxy line protocol
	ProtoProxy
	// ProtoProxyV2 is a HAProxy binary protocol
	ProtoProxyV2
	// ProtoHTTP is HTTP protocol
	ProtoHTTP
	// ProtoPostgres is PostgreSQL wire protocol
	ProtoPostgres
)

// protocolStrings defines strings for each Protocol.
var protocolStrings = map[Protocol]string{
	ProtoUnknown:  "Unknown",
	ProtoTLS:      "TLS",
	ProtoSSH:      "SSH",
	ProtoProxy:    "Proxy",
	ProtoProxyV2:  "ProxyV2",
	ProtoHTTP:     "HTTP",
	ProtoPostgres: "Postgres",
}

// String returns the string representation of Protocol p.
// An empty string is returned when the protocol is not defined.
func (p Protocol) String() string {
	return protocolStrings[p]
}

var (
	proxyPrefix      = []byte{'P', 'R', 'O', 'X', 'Y'}
	proxyV2Prefix    = []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	sshPrefix        = []byte{'S', 'S', 'H'}
	tlsPrefix        = []byte{0x16}
	proxyHelloPrefix = []byte(sshutils.ProxyHelloSignature)
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
	// postgresGSSEncRequest is sent first by a Postgres client
	// to check whether the server supports GSS encryption.
	// It is currently unsupported and our postgres engine will always respond 'N'
	// for "not supported".
	postgresGSSEncRequest = []byte{0x0, 0x0, 0x0, 0x8, 0x4, 0xd2, 0x16, 0x30}
)

var httpMethods = [...][]byte{
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

// isHTTP returns true if the first few bytes of the prefix indicate
// the use of an HTTP method.
func isHTTP(in []byte) bool {
	for _, verb := range httpMethods {
		if bytes.HasPrefix(in, verb) {
			return true
		}
	}
	return false
}

// detectProto tries to determine the network protocol used from the first
// few bytes of a connection.
func detectProto(r *bufio.Reader) (Protocol, error) {
	// read the first 8 bytes without advancing the reader, some connections
	// won't send more than 8 bytes at first
	in, err := r.Peek(8)
	if err != nil {
		return ProtoUnknown, trace.Wrap(err, "failed to peek connection")
	}

	switch {
	case bytes.HasPrefix(in, proxyPrefix):
		return ProtoProxy, nil
	case bytes.HasPrefix(in, proxyV2Prefix[:8]):
		// if the first 8 bytes matches the first 8 bytes of the proxy
		// protocol v2 magic bytes, read more of the connection so we can
		// ensure all magic bytes match
		in, err = r.Peek(len(proxyV2Prefix))
		if err != nil {
			return ProtoUnknown, trace.Wrap(err, "failed to peek connection")
		}
		if bytes.HasPrefix(in, proxyV2Prefix) {
			return ProtoProxyV2, nil
		}
	case bytes.HasPrefix(in, proxyHelloPrefix[:8]):
		// Support for SSH connections opened with the ProxyHelloSignature for
		// Teleport to Teleport connections.
		in, err = r.Peek(len(proxyHelloPrefix))
		if err != nil {
			return ProtoUnknown, trace.Wrap(err, "failed to peek connection")
		}
		if bytes.HasPrefix(in, proxyHelloPrefix) {
			return ProtoSSH, nil
		}
	case bytes.HasPrefix(in, sshPrefix):
		return ProtoSSH, nil
	case bytes.HasPrefix(in, tlsPrefix):
		return ProtoTLS, nil
	case isHTTP(in):
		return ProtoHTTP, nil
	case bytes.HasPrefix(in, postgresSSLRequest),
		bytes.HasPrefix(in, postgresCancelRequest),
		bytes.HasPrefix(in, postgresGSSEncRequest):
		return ProtoPostgres, nil
	}

	return ProtoUnknown, trace.BadParameter("multiplexer failed to detect connection protocol, first few bytes were: %#v", in)
}
