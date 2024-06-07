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
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/loglimit"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	// ErrBadIP is returned when there's a problem with client source or destination IP address
	ErrBadIP = trace.BadParameter(
		"client source and destination addresses should be valid same TCP version non-nil IP addresses")
)

// PROXYProtocolMode controls behavior related to unsigned PROXY protocol headers.
// Possible values:
// - 'on': one PROXY header is accepted and required per incoming connection.
// - 'off': no PROXY headers are allows, otherwise connection is rejected.
// If unspecified - one PROXY header is allowed, but not required. Connection is marked with source port set to 0
// and IP pinning will not be allowed. It is supposed to be used only as default mode for test setups.
// In production you should always explicitly set the mode based on your network setup - if you have L4 load balancer
// with enabled PROXY protocol in front of Teleport you should set it to 'on', if you don't have it, set it to 'off'
type PROXYProtocolMode string

const (
	PROXYProtocolOn          PROXYProtocolMode = "on"
	PROXYProtocolOff         PROXYProtocolMode = "off"
	PROXYProtocolUnspecified PROXYProtocolMode = ""
)

// CertAuthorityGetter allows to get cluster's host CA for verification of signed PROXY headers.
// We define our own version to not create dependency on the 'services' package, which causes circular references
type CertAuthorityGetter = func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

type (
	// PreDetectFunc is used in [Mux]'s [Config] as the PreDetect hook.
	PreDetectFunc = func(net.Conn) (PostDetectFunc, error)

	// PostDetectFunc is optionally returned by a [PreDetectFunc].
	PostDetectFunc = func(*Conn) net.Conn
)

// Config is a multiplexer config
type Config struct {
	// Listener is listener to multiplex connection on
	Listener net.Listener
	// Context is a context to signal stops, cancellations
	Context context.Context
	// DetectTimeout is a timeout applied to the whole detection phase of the
	// connection, set to defaults.ReadHeadersTimeout if unspecified
	DetectTimeout time.Duration
	// Clock is a clock to override in tests, set to real time clock
	// by default
	Clock clockwork.Clock
	// PROXYProtocolMode controls behavior related to unsigned PROXY protocol headers.
	PROXYProtocolMode PROXYProtocolMode
	// SuppressUnexpectedPROXYWarning makes multiplexer not issue warnings if it receives PROXY
	// line when running in PROXYProtocolMode=PROXYProtocolUnspecified
	SuppressUnexpectedPROXYWarning bool
	// ID is an identifier used for debugging purposes
	ID string
	// CertAuthorityGetter is used to get CA to verify singed PROXY headers sent internally by teleport
	CertAuthorityGetter CertAuthorityGetter
	// LocalClusterName set the local cluster for the multiplexer, it's used in PROXY headers verification.
	LocalClusterName string

	// IgnoreSelfConnections is used for tests, it makes multiplexer ignore the fact that it's self
	// connection (coming from same IP as the listening address) when deciding if it should drop connection with
	// missing required PROXY header. This is needed since all connections in tests are self connections.
	IgnoreSelfConnections bool

	// PreDetect, if set, is called on each incoming connection before protocol
	// detection; the returned [PostDetectFunc] (if any) will then be called
	// after protocol detection, and will have the ability to modify or wrap the
	// [*Conn] before it's passed to the listener; if the PostDetectFunc returns
	// a nil [net.Conn], the connection will not be handled any further by the
	// multiplexer, and it's the responsibility of the PostDetectFunc to arrange
	// for it to be eventually closed.
	PreDetect PreDetectFunc
}

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Listener == nil {
		return trace.BadParameter("missing parameter Listener")
	}
	if c.Context == nil {
		c.Context = context.TODO()
	}
	if c.DetectTimeout == 0 {
		c.DetectTimeout = defaults.ReadHeadersTimeout
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
	logLimiter, err := loglimit.New(loglimit.Config{
		Context:           ctx,
		MessageSubstrings: errorSubstrings,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	waitContext, waitCancel := context.WithCancel(context.TODO())
	return &Mux{
		Entry: log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.Component("mx", cfg.ID),
		}),
		Config:      cfg,
		context:     ctx,
		cancel:      cancel,
		waitContext: waitContext,
		waitCancel:  waitCancel,
		logLimiter:  logLimiter,
	}, nil
}

// Mux supports having both SSH and TLS on the same listener socket
type Mux struct {
	sync.RWMutex
	*log.Entry
	Config
	sshListener  *Listener
	tlsListener  *Listener
	dbListener   *Listener
	httpListener *Listener
	context      context.Context
	cancel       context.CancelFunc
	waitContext  context.Context
	waitCancel   context.CancelFunc
	// logLimiter is a goroutine responsible for deduplicating multiplexer errors
	// (over a 1min window) that occur when detecting the types of new connections.
	// This ensures that health checkers / malicious actors cannot overpower /
	// pollute the logs with warnings when such connections are invalid or unknown
	// to the multiplexer.
	logLimiter *loglimit.LogLimiter
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

// HTTP returns listener that receives plain HTTP connections
func (m *Mux) HTTP() net.Listener {
	m.Lock()
	defer m.Unlock()
	if m.httpListener == nil {
		m.httpListener = newListener(m.context, m.Config.Listener.Addr())
	}
	return m.httpListener
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
	m.Debugf("Starting serving MUX, ID %q on address %s", m.Config.ID, m.Config.Listener.Addr())
	defer m.waitCancel()

	for {
		conn, err := m.Listener.Accept()
		if err == nil {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetKeepAlive(true)
				tcpConn.SetKeepAlivePeriod(3 * time.Minute)
			}
			go m.detectAndForward(conn)

			select {
			case <-m.context.Done():
				return trace.Wrap(m.Close())
			default:
				continue
			}
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
	case ProtoHTTP:
		return m.httpListener

	}
	return nil
}

// detectAndForward detects the protocol for conn and forwards to a
// registered protocol listener (SSH, TLS, DB). Connections for a
// protocol without a registered protocol listener are closed. This
// method is called as a goroutine by Serve for each connection.
func (m *Mux) detectAndForward(conn net.Conn) {
	if err := conn.SetDeadline(m.Clock.Now().Add(m.DetectTimeout)); err != nil {
		m.Warning(err.Error())
		conn.Close()
		return
	}

	var postDetect PostDetectFunc
	if m.PreDetect != nil {
		var err error
		postDetect, err = m.PreDetect(conn)
		if err != nil {
			if !utils.IsOKNetworkError(err) {
				m.WithFields(log.Fields{
					"src_addr":   conn.RemoteAddr(),
					"dst_addr":   conn.LocalAddr(),
					log.ErrorKey: err,
				}).Warn("Failed to send early data.")
			}
			conn.Close()
			return
		}
	}

	connWrapper, err := m.detect(conn)
	if err != nil {
		if !errors.Is(trace.Unwrap(err), io.EOF) {
			m.logLimiter.Log(m.Entry.WithFields(log.Fields{
				"src_addr": conn.RemoteAddr(),
				"dst_addr": conn.LocalAddr(),
			}), log.WarnLevel, trace.DebugReport(err))
		}
		conn.Close()
		return
	}

	if err := connWrapper.SetDeadline(time.Time{}); err != nil {
		m.Warning(trace.DebugReport(err))
		connWrapper.Close()
		return
	}

	listener := m.protocolListener(connWrapper.protocol)
	if listener == nil {
		if connWrapper.protocol == ProtoHTTP {
			m.WithFields(log.Fields{
				"src_addr": connWrapper.RemoteAddr(),
				"dst_addr": connWrapper.LocalAddr(),
			}).Debug("Detected an HTTP request. If this is for a health check, use an HTTPS request instead.")
		}
		m.WithFields(log.Fields{
			"src_addr": connWrapper.RemoteAddr(),
			"dst_addr": connWrapper.LocalAddr(),
		}).Debugf("Closing %[1]s connection: %[1]s listener is disabled.", connWrapper.protocol)
		connWrapper.Close()
		return
	}

	conn = connWrapper
	if postDetect != nil {
		conn = postDetect(connWrapper)
		if conn == nil {
			// the post detect hook hijacked the connection or had an error
			return
		}
	}

	listener.HandleConnection(m.context, conn)
}

// JWTPROXYSigner provides ability to created JWT for signed PROXY headers.
type JWTPROXYSigner interface {
	SignPROXYJWT(p jwt.PROXYSignParams) (string, error)
}

func getTCPAddr(a net.Addr) net.TCPAddr {
	if a == nil {
		return net.TCPAddr{}
	}

	addr, ok := a.(*net.TCPAddr)
	if ok { // Hot path
		return *addr
	}

	parsedAddr := utils.FromAddr(a)
	return net.TCPAddr{
		IP:   net.ParseIP(parsedAddr.Host()),
		Port: parsedAddr.Port(-1),
	}
}

func isDifferentTCPVersion(addr1, addr2 net.TCPAddr) bool {
	return (addr1.IP.To4() != nil && addr2.IP.To4() == nil) || (addr2.IP.To4() != nil && addr1.IP.To4() == nil)
}

func signPROXYHeader(sourceAddress, destinationAddress net.Addr, clusterName string, signingCert []byte, signer JWTPROXYSigner) ([]byte, error) {
	sAddr := getTCPAddr(sourceAddress)
	dAddr := getTCPAddr(destinationAddress)
	if sAddr.IP == nil || dAddr.IP == nil || isDifferentTCPVersion(sAddr, dAddr) {
		return nil, trace.Wrap(ErrBadIP, "source address: %s, destination address: %s", sourceAddress, destinationAddress)
	}
	if sAddr.Port < 0 || dAddr.Port < 0 {
		return nil, trace.BadParameter("could not parse port (source:%q, destination: %q)",
			sourceAddress.String(), destinationAddress.String())
	}

	signature, err := signer.SignPROXYJWT(jwt.PROXYSignParams{
		SourceAddress:      sAddr.String(),
		DestinationAddress: dAddr.String(),
		ClusterName:        clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err, "could not sign jwt token for PROXY line")
	}

	protocol := TCP4
	if sAddr.IP.To4() == nil {
		protocol = TCP6
	}
	pl := ProxyLine{
		Protocol:    protocol,
		Source:      sAddr,
		Destination: dAddr,
	}
	err = pl.AddSignature([]byte(signature), signingCert)
	if err != nil {
		return nil, trace.Wrap(err, "could not add signature to proxy line")
	}

	b, err := pl.Bytes()
	if err != nil {
		return nil, trace.Wrap(err, "could not get bytes from proxy line")
	}

	return b, nil
}

// errorSubstrings includes all the error substrings that can be returned by `Mux.detect`.
// These are used to deduplicate the errors returned by the multiplexer that occur
// when detecting the type of a new connection just established.
// This ensures that health checkers / malicious actors cannot pollute / overpower
// the logs with warnings when such connections are invalid or unknown to the multiplexer.
var errorSubstrings = []string{
	failedToPeekConnectionError,
	failedToDetectConnectionProtocolError,
	externalProxyProtocolDisabledError,
	duplicateSignedProxyLineError,
	duplicateUnsignedProxyLineError,
	invalidProxyLineError,
	invalidProxyV2LineError,
	invalidProxySignatureError,
	unknownProtocolError,
	missingProxyLineError,
	unexpectedPROXYLineError,
	unsignedPROXYLineAfterSignedError,
}

const (
	// maxDetectionPasses sets maximum amount of passes to detect final protocol to account
	// for 1 unsigned header, 1 signed header and the final protocol itself
	maxDetectionPasses = 3

	failedToPeekConnectionError           = "failed to peek connection"
	failedToDetectConnectionProtocolError = "failed to detect connection protocol"
	externalProxyProtocolDisabledError    = "external PROXY protocol support is disabled"
	duplicateSignedProxyLineError         = "duplicate signed PROXY line"
	duplicateUnsignedProxyLineError       = "duplicate unsigned PROXY line"
	invalidProxyLineError                 = "invalid PROXY line"
	invalidProxyV2LineError               = "invalid PROXY v2 line"
	invalidProxySignatureError            = "could not verify PROXY signature for connection"
	missingProxyLineError                 = `connection (%s -> %s) rejected because PROXY protocol is enabled but required
PROXY protocol line wasn't received. 
Make sure you have correct configuration, only enable "proxy_protocol: on" in config if Teleport is running behind L4 
load balancer with enabled PROXY protocol.`
	unknownProtocolError     = "unknown protocol"
	unexpectedPROXYLineError = `received unexpected PROXY protocol line. Connection will be allowed, but this is usually a result of misconfiguration - 
if Teleport is running behind L4 load balancer with enabled PROXY protocol you should explicitly set config field "proxy_protocol" to "on".
See documentation for more details`
	unsignedPROXYLineAfterSignedError = "received unsigned PROXY line after already receiving signed PROXY line"
)

// detect finds out a type of the connection and returns wrapper that support PROXY protocol
func (m *Mux) detect(conn net.Conn) (*Conn, error) {
	reader := bufio.NewReader(conn)

	// Before actual protocol traffic flows, we try to parse optional PROXY protocol headers,
	// that can be injected by load balancers or our own proxies. There can be multiple PROXY
	// headers. After they are parsed, last pass does the actual protocol detection itself.
	// We allow only one unsigned PROXY header from external sources, if it's enabled, and one
	// signed header from our own proxies, which take precedence.
	var proxyLine *ProxyLine
	unsignedPROXYLineReceived := false
	for i := 0; i < maxDetectionPasses; i++ {
		proto, err := detectProto(reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch proto {
		case ProtoProxy:
			newPROXYLine, err := ReadProxyLine(reader)
			if err != nil {
				return nil, trace.Wrap(err, invalidProxyLineError)
			}

			if m.PROXYProtocolMode == PROXYProtocolOff {
				return nil, trace.BadParameter(externalProxyProtocolDisabledError)
			}

			if unsignedPROXYLineReceived {
				// We allow only one unsigned PROXY line
				return nil, trace.BadParameter(duplicateUnsignedProxyLineError)
			}
			unsignedPROXYLineReceived = true

			if m.PROXYProtocolMode == PROXYProtocolUnspecified && !m.SuppressUnexpectedPROXYWarning {
				m.logLimiter.Log(m.WithFields(log.Fields{
					"direct_src_addr": conn.RemoteAddr(),
					"direct_dst_addr": conn.LocalAddr(),
					"proxy_src_addr:": newPROXYLine.Source.String(),
					"proxy_dst_addr:": newPROXYLine.Destination.String(),
				}), log.ErrorLevel, unexpectedPROXYLineError)
				newPROXYLine.Source.Port = 0 // Mark connection, so if later IP pinning check is used on it we can reject it.
			}

			if proxyLine != nil && proxyLine.IsVerified {
				// Unsigned PROXY line after signed one should not happen
				return nil, trace.BadParameter(unsignedPROXYLineAfterSignedError)
			}

			proxyLine = newPROXYLine

			// repeat the cycle to detect the protocol
		case ProtoProxyV2:
			newPROXYLine, err := ReadProxyLineV2(reader)
			if err != nil {
				return nil, trace.Wrap(err, invalidProxyV2LineError)
			}
			if newPROXYLine == nil {
				if unsignedPROXYLineReceived {
					// We allow only one unsigned PROXY line
					return nil, trace.BadParameter(duplicateUnsignedProxyLineError)
				}
				unsignedPROXYLineReceived = true
				continue // Skipping LOCAL command of PROXY protocol
			}

			// If proxyline is not signed, so we don't try to verify to avoid unnecessary load
			if m.CertAuthorityGetter != nil && m.LocalClusterName != "" && newPROXYLine.IsSigned() {
				err = newPROXYLine.VerifySignature(m.context, m.CertAuthorityGetter, m.LocalClusterName, m.Clock)
				if errors.Is(err, ErrNoHostCA) {
					m.WithFields(log.Fields{
						"src_addr": conn.RemoteAddr(),
						"dst_addr": conn.LocalAddr(),
					}).Warnf("%s - could not get host CA", invalidProxySignatureError)
					continue
				}
				if err != nil {
					return nil, trace.Wrap(err, "%s %s -> %s", invalidProxySignatureError, conn.RemoteAddr(), conn.LocalAddr())
				}
				m.WithFields(log.Fields{
					"conn_src_addr":   conn.RemoteAddr(),
					"conn_dst_addr":   conn.LocalAddr(),
					"client_src_addr": newPROXYLine.Source.String(),
				}).Tracef("Successfully verified signed PROXYv2 header")
			}

			// If proxy line is signed and successfully verified and there's no already signed proxy header,
			// we accept, otherwise reject
			if newPROXYLine.IsVerified {
				if proxyLine != nil && proxyLine.IsVerified {
					return nil, trace.BadParameter(duplicateSignedProxyLineError)
				}

				proxyLine = newPROXYLine
				continue
			}

			if m.CertAuthorityGetter != nil && newPROXYLine.IsSigned() && !newPROXYLine.IsVerified {
				return nil, trace.BadParameter("could not verify PROXY line signature")
			}

			// This is unsigned proxy line, return error if external PROXY protocol is not enabled
			if m.PROXYProtocolMode == PROXYProtocolOff {
				return nil, trace.BadParameter(externalProxyProtocolDisabledError)
			}

			if unsignedPROXYLineReceived {
				// We allow only one unsigned PROXY line
				return nil, trace.BadParameter(duplicateUnsignedProxyLineError)
			}
			unsignedPROXYLineReceived = true

			if m.PROXYProtocolMode == PROXYProtocolUnspecified && !m.SuppressUnexpectedPROXYWarning {
				m.logLimiter.Log(m.WithFields(log.Fields{
					"direct_src_addr": conn.RemoteAddr(),
					"direct_dst_addr": conn.LocalAddr(),
					"proxy_src_addr:": newPROXYLine.Source.String(),
					"proxy_dst_addr:": newPROXYLine.Destination.String(),
				}), log.ErrorLevel, unexpectedPROXYLineError)
				newPROXYLine.Source.Port = 0 // Mark connection, so if later IP pinning check is used on it we can reject it.
			}

			// Unsigned PROXY line after signed should not happen
			if proxyLine != nil && proxyLine.IsVerified {
				return nil, trace.BadParameter(unsignedPROXYLineAfterSignedError)
			}

			proxyLine = newPROXYLine
			// repeat the cycle to detect the protocol
		case ProtoTLS, ProtoSSH, ProtoHTTP, ProtoPostgres:
			if err := m.checkPROXYProtocolRequirement(conn, unsignedPROXYLineReceived); err != nil {
				return nil, trace.Wrap(err)
			}

			return &Conn{
				protocol:  proto,
				Conn:      conn,
				reader:    reader,
				proxyLine: proxyLine,
			}, nil
		}
	}
	// if code ended here after three attempts, something is wrong
	return nil, trace.BadParameter(unknownProtocolError)
}

// checkPROXYProtocolRequirement checks that if multiplexer is required to receive unsigned PROXY line
// that requirement is fulfilled, or exceptions apply - self connections and connections that are passed
// from upstream multiplexed listener (as it happens for alpn proxy).
func (m *Mux) checkPROXYProtocolRequirement(conn net.Conn, unsignedPROXYLineReceived bool) error {
	if m.PROXYProtocolMode != PROXYProtocolOn {
		return nil
	}

	// Proxy and other services might call itself directly, avoiding
	// load balancer, so we shouldn't fail connections without PROXY headers for such cases.
	selfConnection, err := m.isSelfConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}

	// We try to get inner multiplexer connection, if we succeed and there is on, it means conn was passed
	// to us from another multiplexer listener and unsigned PROXY protocol requirement was handled there.
	innerConn := unwrapMuxConn(conn)

	if !selfConnection && innerConn == nil && !unsignedPROXYLineReceived {
		return trace.BadParameter(missingProxyLineError, conn.RemoteAddr().String(), conn.LocalAddr().String())
	}

	return nil
}

func unwrapMuxConn(conn net.Conn) *Conn {
	type netConn interface {
		NetConn() net.Conn
	}

	for {
		if muxConn, ok := conn.(*Conn); ok {
			return muxConn
		}

		connGetter, ok := conn.(netConn)
		if !ok {
			return nil
		}
		conn = connGetter.NetConn()
	}
}

func (m *Mux) isSelfConnection(conn net.Conn) (bool, error) {
	if m.IgnoreSelfConnections {
		return false, nil
	}

	remoteHost, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return false, trace.Wrap(err)
	}
	localHost, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return false, trace.Wrap(err)
	}

	return remoteHost == localHost, nil
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
	proxyPrefix   = []byte{'P', 'R', 'O', 'X', 'Y'}
	ProxyV2Prefix = []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	sshPrefix     = []byte{'S', 'S', 'H'}
	tlsPrefix     = []byte{0x16}
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
		return ProtoUnknown, trace.Wrap(err, failedToPeekConnectionError)
	}

	switch {
	case bytes.HasPrefix(in, proxyPrefix):
		return ProtoProxy, nil
	case bytes.HasPrefix(in, ProxyV2Prefix[:8]):
		// if the first 8 bytes matches the first 8 bytes of the proxy
		// protocol v2 magic bytes, read more of the connection so we can
		// ensure all magic bytes match
		in, err = r.Peek(len(ProxyV2Prefix))
		if err != nil {
			return ProtoUnknown, trace.Wrap(err, failedToPeekConnectionError)
		}
		if bytes.HasPrefix(in, ProxyV2Prefix) {
			return ProtoProxyV2, nil
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

	return ProtoUnknown, trace.BadParameter("%s, first few bytes were: %#v", failedToDetectConnectionProtocolError, in)
}

// PROXYHeaderSigner allows to sign PROXY headers for securely propagating original client IP information
type PROXYHeaderSigner interface {
	SignPROXYHeader(source, destination net.Addr) ([]byte, error)
}

// PROXYSigner implements PROXYHeaderSigner to sign PROXY headers
type PROXYSigner struct {
	signingCertDER []byte
	clusterName    string
	jwtSigner      JWTPROXYSigner
}

// NewPROXYSigner returns a new instance of PROXYSigner
func NewPROXYSigner(signingCert *x509.Certificate, jwtSigner JWTPROXYSigner) (*PROXYSigner, error) {
	identity, err := tlsca.FromSubject(signingCert.Subject, signingCert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ok := checkForSystemRole(identity, types.RoleProxy); !ok {
		return nil, trace.Wrap(ErrIncorrectRole)
	}

	return &PROXYSigner{
		signingCertDER: signingCert.Raw,
		clusterName:    identity.TeleportCluster,
		jwtSigner:      jwtSigner,
	}, nil
}

// SignPROXYHeader creates a signed PROXY header with provided source and destination addresses
func (p *PROXYSigner) SignPROXYHeader(source, destination net.Addr) ([]byte, error) {
	header, err := signPROXYHeader(source, destination, p.clusterName, p.signingCertDER, p.jwtSigner)
	if err == nil {
		log.WithFields(log.Fields{
			"src_addr":     fmt.Sprintf("%v", source),
			"dst_addr":     fmt.Sprintf("%v", destination),
			"cluster_name": p.clusterName}).Trace("Successfully generated signed PROXY header")
	}
	return header, trace.Wrap(err)
}
