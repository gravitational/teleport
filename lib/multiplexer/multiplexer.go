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
	"github.com/gravitational/teleport/api/constants"
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

// CertAuthorityGetter allows to get cluster's host CA for verification of signed PROXY headers.
// We define our own version to not create dependency on the 'services' package, which causes circular references
type CertAuthorityGetter = func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

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
	// EnableExternalProxyProtocol enables proxy protocol from external (unsigned) sources
	EnableExternalProxyProtocol bool
	// ID is an identifier used for debugging purposes
	ID string
	// CertAuthorityGetter is used to get CA to verify singed PROXY headers sent internally by teleport
	CertAuthorityGetter CertAuthorityGetter
	// LocalClusterName set the local cluster for the multiplexer, it's used in PROXY headers verification.
	LocalClusterName string
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
			trace.Component: teleport.Component("mx", cfg.ID),
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
	sshListener *Listener
	tlsListener *Listener
	dbListener  *Listener
	context     context.Context
	cancel      context.CancelFunc
	waitContext context.Context
	waitCancel  context.CancelFunc
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

	connWrapper, err := m.detect(conn)
	if err != nil {
		if trace.Unwrap(err) != io.EOF {
			m.logLimiter.Log(m.Entry, log.WarnLevel, trace.DebugReport(err))
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
	proxyProtocolDisabledError,
	externalProxyProtocolDisabledError,
	duplicateProxyLineError,
	duplicateSignedProxyLineError,
	duplicateUnsignedProxyLineError,
	invalidProxyLineError,
	invalidProxyV2LineError,
	invalidProxySignatureError,
	unknownProtocolError,
}

const (
	// maxDetectionPasses sets maximum amount of passes to detect final protocol to account
	// for 1 unsigned header, 1 signed header and the final protocol itself
	maxDetectionPasses = 3

	failedToPeekConnectionError           = "failed to peek connection"
	failedToDetectConnectionProtocolError = "failed to detect connection protocol"
	proxyProtocolDisabledError            = "proxy protocol support is disabled"
	externalProxyProtocolDisabledError    = "external proxy protocol support is disabled"
	duplicateProxyLineError               = "duplicate proxy line"
	duplicateSignedProxyLineError         = "duplicate signed proxy line"
	duplicateUnsignedProxyLineError       = "duplicate unsigned proxy line"
	invalidProxyLineError                 = "invalid proxy line"
	invalidProxyV2LineError               = "invalid proxy v2 line"
	invalidProxySignatureError            = "could not verify PROXY signature for connection"
	unknownProtocolError                  = "unknown protocol"
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
	for i := 0; i < maxDetectionPasses; i++ {
		proto, err := detectProto(reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch proto {
		case ProtoProxy:
			if !m.EnableExternalProxyProtocol {
				return nil, trace.BadParameter(proxyProtocolDisabledError)
			}
			// We allow only one unsigned proxy line
			if proxyLine != nil {
				return nil, trace.BadParameter(duplicateProxyLineError)
			}
			proxyLine, err = ReadProxyLine(reader)
			if err != nil {
				return nil, trace.Wrap(err, invalidProxyLineError)
			}
			// repeat the cycle to detect the protocol
		case ProtoProxyV2:
			newProxyLine, err := ReadProxyLineV2(reader)
			if err != nil {
				return nil, trace.Wrap(err, invalidProxyV2LineError)
			}
			if newProxyLine == nil {
				continue
			}

			// If proxyline is not signed, so we don't try to verify to avoid unnecessary load
			if m.CertAuthorityGetter != nil && m.LocalClusterName != "" && newProxyLine.IsSigned() {
				err = newProxyLine.VerifySignature(m.context, m.CertAuthorityGetter, m.LocalClusterName, m.Clock)
				if errors.Is(err, ErrNoHostCA) {
					m.WithFields(log.Fields{
						"src_addr": conn.RemoteAddr(),
						"dst_addr": conn.LocalAddr(),
					}).Warnf("%s - could not get host CA", invalidProxySignatureError)
					continue
				}
				// DELETE IN 14.0, early 12 versions could send PROXY headers to remote auth server
				if errors.Is(err, ErrNonLocalCluster) {
					m.WithFields(log.Fields{
						"src_addr": conn.RemoteAddr(),
						"dst_addr": conn.LocalAddr(),
					}).Debugf("%s - signed by non local cluster", invalidProxySignatureError)
					continue
				}
				if err != nil {
					return nil, trace.Wrap(err, "%s %s -> %s", invalidProxySignatureError, conn.RemoteAddr(), conn.LocalAddr())
				}
				m.WithFields(log.Fields{
					"conn_src_addr":   conn.RemoteAddr(),
					"conn_dst_addr":   conn.LocalAddr(),
					"client_src_addr": newProxyLine.Source.String(),
				}).Tracef("Successfully verified signed PROXYv2 header")
			}

			// If proxy line is signed and successfully verified and there's no already signed proxy header,
			// we accept, otherwise reject
			if newProxyLine.IsVerified {
				if proxyLine != nil && proxyLine.IsVerified {
					return nil, trace.BadParameter(duplicateSignedProxyLineError)
				}

				proxyLine = newProxyLine
				continue
			}

			if m.CertAuthorityGetter != nil && newProxyLine.IsSigned() && !newProxyLine.IsVerified {
				return nil, trace.BadParameter("could not verify proxy line signature")
			}

			// This is unsigned proxy line, return error if external PROXY protocol is not enabled
			if !m.EnableExternalProxyProtocol {
				return nil, trace.BadParameter(externalProxyProtocolDisabledError)
			}

			// If current proxy line was signed and verified, it takes precedence over new not signed proxy line
			if proxyLine != nil && proxyLine.IsVerified {
				continue
			}

			// We allow only one unsigned proxy line
			if proxyLine != nil {
				return nil, trace.BadParameter(duplicateUnsignedProxyLineError)
			}

			proxyLine = newProxyLine
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
	// if code ended here after three attempts, something is wrong
	return nil, trace.BadParameter(unknownProtocolError)
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
	ProxyV2Prefix    = []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	sshPrefix        = []byte{'S', 'S', 'H'}
	tlsPrefix        = []byte{0x16}
	proxyHelloPrefix = []byte(constants.ProxyHelloSignature)
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
	case bytes.HasPrefix(in, proxyHelloPrefix[:8]):
		// Support for SSH connections opened with the ProxyHelloSignature for
		// Teleport to Teleport connections.
		in, err = r.Peek(len(proxyHelloPrefix))
		if err != nil {
			return ProtoUnknown, trace.Wrap(err, failedToPeekConnectionError)
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
