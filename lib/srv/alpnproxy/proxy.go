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

package alpnproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ProxyConfig  is the configuration for an ALPN proxy server.
type ProxyConfig struct {
	// Listener is a listener to serve requests on.
	Listener net.Listener
	// WebTLSConfig specifies the TLS configuration used by the Proxy server.
	WebTLSConfig *tls.Config
	// Router contains definition of protocol routing and handlers description.
	Router *Router
	// Log is used for logging.
	Log *slog.Logger
	// Clock is a clock to override in tests, set to real time clock
	// by default
	Clock clockwork.Clock
	// ReadDeadline is a connection read deadline during the TLS handshake (start
	// of the connection). It is set to defaults.HandshakeReadDeadline if
	// unspecified.
	ReadDeadline time.Duration
	// IdentityTLSConfig is the TLS ProxyRole identity used in servers with localhost SANs values.
	IdentityTLSConfig *tls.Config
	// AccessPoint is the auth server client.
	AccessPoint authclient.CAGetter
	// ClusterName is the name of the teleport cluster.
	ClusterName string
	// PingInterval defines the ping interval for ping-wrapped connections.
	PingInterval time.Duration
}

// NewRouter creates a ALPN new router.
func NewRouter() *Router {
	return &Router{
		alpnHandlers: make([]*HandlerDecs, 0),
	}
}

// Router contains information about protocol handlers and routing rules.
type Router struct {
	alpnHandlers       []*HandlerDecs
	kubeHandler        *HandlerDecs
	databaseTLSHandler *HandlerDecs
	mtx                sync.Mutex
}

// MatchFunc is a type of the match route functions.
type MatchFunc func(sni, alpn string) bool

// MatchByProtocol creates a match function that matches the client TLS ALPN
// protocol against the provided list of ALPN protocols and their corresponding
// Ping protocols.
func MatchByProtocol(protocols ...common.Protocol) MatchFunc {
	m := make(map[common.Protocol]struct{})
	for _, v := range common.WithPingProtocols(protocols) {
		m[v] = struct{}{}
	}
	return func(sni, alpn string) bool {
		_, ok := m[common.Protocol(alpn)]
		return ok
	}
}

// MatchByALPNPrefix creates match function based on client TLS ALPN protocol prefix.
func MatchByALPNPrefix(prefix string) MatchFunc {
	return func(sni, alpn string) bool {
		return strings.HasPrefix(alpn, prefix)
	}
}

// ExtractMySQLEngineVersion returns a pre-process function for MySQL connections that tries to extract MySQL server version
// from incoming connection.
func ExtractMySQLEngineVersion(fn func(ctx context.Context, conn net.Conn) error) HandlerFuncWithInfo {
	return func(ctx context.Context, conn net.Conn, info ConnectionInfo) error {
		const mysqlVerStart = len(common.ProtocolMySQLWithVerPrefix)

		for _, alpn := range info.ALPN {
			if strings.HasSuffix(alpn, string(common.ProtocolPingSuffix)) ||
				!strings.HasPrefix(alpn, string(common.ProtocolMySQLWithVerPrefix)) ||
				len(alpn) == mysqlVerStart {
				continue
			}
			// The version should never be longer than 255 characters including
			// the prefix, but better to be safe.
			versionEnd := 255
			if len(alpn) < versionEnd {
				versionEnd = len(alpn)
			}

			mysqlVersionBase64 := alpn[mysqlVerStart:versionEnd]
			mysqlVersionBytes, err := base64.StdEncoding.DecodeString(mysqlVersionBase64)
			if err != nil {
				continue
			}

			ctx = context.WithValue(ctx, dbutils.ContextMySQLServerVersion, string(mysqlVersionBytes))
			break
		}

		return fn(ctx, conn)
	}
}

// CheckAndSetDefaults verifies the constraints for Router.
func (r *Router) CheckAndSetDefaults() error {
	for _, v := range r.alpnHandlers {
		if err := v.CheckAndSetDefaults(); err != nil {
			return err
		}
	}
	return nil
}

// AddKubeHandler adds the handle for Kubernetes protocol (distinguishable by  "kube-teleport-proxy-alpn." SNI prefix).
func (r *Router) AddKubeHandler(handler HandlerFunc) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.kubeHandler = &HandlerDecs{
		Handler:    handler,
		ForwardTLS: true,
	}
}

// AddDBTLSHandler adds the handler for DB TLS traffic.
func (r *Router) AddDBTLSHandler(handler HandlerFunc) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.databaseTLSHandler = &HandlerDecs{
		Handler: handler,
	}
}

// Add sets the handler for DB TLS traffic.
func (r *Router) Add(desc HandlerDecs) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.alpnHandlers = append(r.alpnHandlers, &desc)
}

// HandlerDecs describes the handler for particular protocols.
type HandlerDecs struct {
	// Handler is protocol handling logic.
	Handler HandlerFunc
	// HandlerWithConnInfo is protocol handler function providing additional TLS insight.
	// Used in cases where internal handler function must have access to hello message values without
	// terminating the TLS connection.
	HandlerWithConnInfo HandlerFuncWithInfo
	// ForwardTLS tells is ALPN proxy service should terminate TLS traffic or delegate the
	// TLS termination to the protocol handler (Used in Kube handler case).
	//
	// It is the upstream servers responsibility to provide the appropriate [tls.Config.NextProtos]
	// to confirm the negotiated protocol.
	ForwardTLS bool
	// MatchFunc is a routing route match function based on ALPN SNI TLS values.
	// If is evaluated to true the current HandleDesc will be used
	// for connection handling.
	MatchFunc MatchFunc
	// TLSConfig is TLS configuration that allows switching TLS settings for the handle.
	// By default, the ProxyConfig.WebTLSConfig configuration is used to TLS terminate incoming connections,
	// but if [HandlerDecs.TLSConfig] is present, it will take precedence over [ProxyConfig.WebTLSConfig].
	//
	// It is the responsibility of the creator of the [tls.Config] to provide the appropriate [tls.Config.NextProtos]
	// to confirm the negotiated protocol.
	TLSConfig *tls.Config
}

func (h *HandlerDecs) CheckAndSetDefaults() error {
	if h.Handler != nil && h.HandlerWithConnInfo != nil {
		return trace.BadParameter("can't create route with both Handler and HandlerWithConnInfo handlers")
	}
	if h.MatchFunc == nil {
		return trace.BadParameter("missing param MatchFunc")
	}

	if h.ForwardTLS && h.TLSConfig != nil {
		return trace.BadParameter("the ForwardTLS flag and TLSConfig can't be used at the same time")
	}
	return nil
}

func (h *HandlerDecs) handle(ctx context.Context, conn net.Conn, info ConnectionInfo) error {
	if h.HandlerWithConnInfo != nil {
		return h.HandlerWithConnInfo(ctx, conn, info)
	}
	if h.Handler == nil {
		return trace.BadParameter("failed to find ALPN handler for ALPN: %v, SNI %v", info.ALPN, info.SNI)
	}
	return h.Handler(ctx, conn)
}

// HandlerFunc is a common function signature used to handle downstream with
// particular ALPN protocol.
type HandlerFunc func(ctx context.Context, conn net.Conn) error

// Proxy server allows routing downstream connections based on
// TLS SNI ALPN values to particular service.
type Proxy struct {
	cfg                ProxyConfig
	supportedProtocols []common.Protocol
	log                *slog.Logger

	// mu guards cancel
	mu     sync.Mutex
	cancel context.CancelFunc
}

// CheckAndSetDefaults checks and sets default values of ProxyConfig
func (c *ProxyConfig) CheckAndSetDefaults() error {
	if c.WebTLSConfig == nil {
		return trace.BadParameter("tls config missing")
	}
	if c.Listener == nil {
		return trace.BadParameter("listener missing")
	}
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, "alpn:proxy")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.ReadDeadline == 0 {
		c.ReadDeadline = defaults.HandshakeReadDeadline
	}
	if c.Router == nil {
		return trace.BadParameter("missing parameter router")
	}
	if err := c.Router.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing access point")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if c.PingInterval == 0 {
		c.PingInterval = defaults.ProxyPingInterval
	}

	if c.IdentityTLSConfig == nil {
		return trace.BadParameter("missing identity tls config")
	}
	c.IdentityTLSConfig = c.IdentityTLSConfig.Clone()
	c.IdentityTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	fn := authclient.WithClusterCAs(c.IdentityTLSConfig, c.AccessPoint, c.ClusterName, c.Log)
	c.IdentityTLSConfig.GetConfigForClient = fn

	return nil
}

// New creates a new instance of the Proxy.
func New(cfg ProxyConfig) (*Proxy, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Proxy{
		cfg:                cfg,
		log:                cfg.Log,
		supportedProtocols: common.SupportedProtocols,
	}, nil
}

// Serve starts accepting connections.
func (p *Proxy) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return trace.BadParameter("Serve may only be called once")
	}
	p.cancel = cancel
	p.mu.Unlock()

	p.cfg.WebTLSConfig.NextProtos = common.ProtocolsToString(p.supportedProtocols)
	for {
		clientConn, err := p.cfg.Listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) || ctx.Err() != nil {
				return nil
			}
			return trace.Wrap(err)
		}
		go func() {
			// In case of successful handleConn call leave the connection Close() call up to service handler.
			// For example in ReverseTunnel handles connection asynchronously and closing conn after
			// service handler returned will break service logic.
			// https://github.com/gravitational/teleport/blob/master/lib/sshutils/server.go#L397
			if err := p.handleConn(ctx, clientConn, nil); err != nil {
				if cerr := clientConn.Close(); cerr != nil && !utils.IsOKNetworkError(cerr) {
					p.log.WarnContext(ctx, "Failed to close client connection", "error", cerr)
				}
				switch {
				case trace.IsBadParameter(err):
					p.log.WarnContext(ctx, "Failed to handle client connection", "error", err)
				case utils.IsOKNetworkError(err):
				case isConnRemoteError(err):
					p.log.DebugContext(ctx, "Connection rejected by client", "error", err, "remote_addr", clientConn.RemoteAddr())
				default:
					p.log.WarnContext(ctx, "Failed to handle client connection", "error", err)
				}
			}
		}()
	}
}

// ConnectionInfo contains details about TLS connection.
type ConnectionInfo struct {
	// SNI is ServerName value obtained from TLS hello message.
	SNI string
	// ALPN protocols obtained from TLS hello message.
	ALPN []string
}

// HandlerFuncWithInfo is protocol handler function providing additional TLS insight.
// Used in cases where internal handler function must have access to hello message values without
// terminating the TLS connection.
type HandlerFuncWithInfo func(ctx context.Context, conn net.Conn, info ConnectionInfo) error

// handleConn routes incoming connection based on SNI TLS information to the proper Handler by following steps:
//  1. Read TLS hello message without TLS termination and returns conn that will be used for further operations.
//  2. Get routing rules for p.Router.Router based on SNI and ALPN fields read in step 1.
//  3. If the selected handler was configured with the ForwardTLS
//     forwards the connection to the handler without TLS termination.
//  4. Trigger TLS handshake and terminates the TLS connection.
//  5. For backward compatibility check RouteToDatabase identity field
//     was set if yes forward to the generic TLS DB handler.
//  6. Forward connection to the handler obtained in step 2.
func (p *Proxy) handleConn(ctx context.Context, clientConn net.Conn, defaultOverride *tls.Config) error {
	hello, conn, err := p.readHelloMessageWithoutTLSTermination(ctx, clientConn)
	if err != nil {
		return trace.Wrap(err)
	}

	handlerDesc, err := p.getHandlerDescBaseOnClientHelloMsg(hello)
	if err != nil {
		return trace.Wrap(err)
	}

	connInfo := ConnectionInfo{
		SNI:  hello.ServerName,
		ALPN: hello.SupportedProtos,
	}
	ctx = authz.ContextWithClientAddrs(ctx, clientConn.RemoteAddr(), clientConn.LocalAddr())

	if handlerDesc.ForwardTLS {
		return trace.Wrap(handlerDesc.handle(ctx, conn, connInfo))
	}

	tlsConn := tls.Server(conn, p.getTLSConfig(handlerDesc, defaultOverride))
	if err := tlsConn.SetReadDeadline(p.cfg.Clock.Now().Add(p.cfg.ReadDeadline)); err != nil {
		return trace.Wrap(err)
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return trace.Wrap(err)
	}
	if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
		return trace.Wrap(err)
	}

	// We try to do quick early IP pinning check, if possible, and stop it on the proxy, without going further.
	// It's based only on client cert. Client can still fail full IP pinning check later if their role now requires
	// IP pinning but cert isn't pinned.
	if err := p.checkCertIPPinning(ctx, tlsConn); err != nil {
		return trace.Wrap(err)
	}

	var handlerConn net.Conn = tlsConn
	// Check if ping is supported/required by the client.
	if common.IsPingProtocol(common.Protocol(tlsConn.ConnectionState().NegotiatedProtocol)) {
		handlerConn = p.handlePingConnection(ctx, tlsConn)
	}

	isDatabaseConnection, err := dbutils.IsDatabaseConnection(tlsConn.ConnectionState())
	if err != nil {
		p.log.DebugContext(ctx, "Failed to check if connection is database connection", "error", err)
	}
	if isDatabaseConnection {
		return trace.Wrap(p.handleDatabaseConnection(ctx, handlerConn, connInfo))
	}
	return trace.Wrap(handlerDesc.handle(ctx, handlerConn, connInfo))
}

func (p *Proxy) checkCertIPPinning(ctx context.Context, tlsConn *tls.Conn) error {
	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		return nil
	}

	identity, err := tlsca.FromSubject(state.PeerCertificates[0].Subject, state.PeerCertificates[0].NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	clientIP, port, err := net.SplitHostPort(tlsConn.RemoteAddr().String())
	if err != nil {
		return trace.Wrap(err)
	}

	if identity.PinnedIP != "" && (clientIP != identity.PinnedIP || port == "0") {
		if port == "0" {
			p.log.DebugContext(ctx, "pinned IP doesn't match observed client IP",
				"client_ip", clientIP,
				"pinned_ip", identity.PinnedIP,
			)
		}
		return trace.Wrap(authz.ErrIPPinningMismatch)
	}

	return nil
}

// handlePingConnection starts the server ping routine and returns `pingConn`.
func (p *Proxy) handlePingConnection(ctx context.Context, conn *tls.Conn) utils.TLSConn {
	pingConn := pingconn.NewTLS(conn)

	// Start ping routine. It will continuously send pings in a defined
	// interval.
	go func() {
		ticker := time.NewTicker(p.cfg.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := pingConn.WritePing()
				if err != nil {
					if !utils.IsOKNetworkError(err) {
						p.log.WarnContext(ctx, "Failed to write ping message", "error", err)
					}

					return
				}
			}
		}
	}()

	return pingConn
}

// getTLSConfig picks the TLS config with the following priority:
//   - TLS config found in the provided handler.
//   - A default override.
//   - The default TLS config (cfg.WebTLSConfig).
func (p *Proxy) getTLSConfig(desc *HandlerDecs, defaultOverride *tls.Config) *tls.Config {
	if desc.TLSConfig != nil {
		return desc.TLSConfig
	}
	if defaultOverride != nil {
		return defaultOverride
	}
	return p.cfg.WebTLSConfig
}

// readHelloMessageWithoutTLSTermination allows reading a ClientHelloInfo message without termination of
// incoming TLS connection. After calling readHelloMessageWithoutTLSTermination function a returned
// net.Conn should be used for further operation.
func (p *Proxy) readHelloMessageWithoutTLSTermination(ctx context.Context, conn net.Conn) (*tls.ClientHelloInfo, net.Conn, error) {
	buff := new(bytes.Buffer)
	var hello *tls.ClientHelloInfo
	tlsConn := tls.Server(readOnlyConn{reader: io.TeeReader(conn, buff)}, &tls.Config{
		GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = info
			return nil, nil
		},
	})
	if err := conn.SetReadDeadline(p.cfg.Clock.Now().Add(p.cfg.ReadDeadline)); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Following TLS handshake fails on the server side with error: "no certificates configured" after server
	// receives a TLS hello message from the client. If handshake was able to read hello message it indicates successful
	// flow otherwise TLS handshake error is returned.
	err := tlsConn.HandshakeContext(ctx)
	if hello == nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return hello, newBufferedConn(conn, buff), nil
}

func (p *Proxy) handleDatabaseConnection(ctx context.Context, conn net.Conn, connInfo ConnectionInfo) error {
	if p.cfg.Router.databaseTLSHandler == nil {
		return trace.BadParameter("database handle not enabled")
	}
	return p.cfg.Router.databaseTLSHandler.handle(ctx, conn, connInfo)
}

func (p *Proxy) databaseHandlerWithTLSTermination(ctx context.Context, conn net.Conn, info ConnectionInfo) error {
	// DB Protocols like Mongo have native support for TLS thus TLS connection needs to be terminated twice.
	// First time for custom local proxy connection and the second time from Mongo protocol where TLS connection is used.
	//
	// Terminate the CLI TLS connection established by a database client that supports native TLS protocol like mongo.
	// Mongo client establishes a connection to SNI ALPN Local Proxy with server name 127.0.0.1 where LocalProxy wraps
	// the connection in TLS and forward to Teleport SNI ALPN Proxy where first TLS layer is terminated
	// by Proxy.handleConn using ProxyConfig.WebTLSConfig.
	tlsConn := tls.Server(conn, p.cfg.IdentityTLSConfig)
	if err := tlsConn.SetReadDeadline(p.cfg.Clock.Now().Add(p.cfg.ReadDeadline)); err != nil {
		if err := tlsConn.Close(); err != nil {
			p.log.ErrorContext(ctx, "Failed to close TLS connection", "error", err)
		}
		return trace.Wrap(err)
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return trace.Wrap(err)
	}
	if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
		if err := tlsConn.Close(); err != nil {
			p.log.ErrorContext(ctx, "Failed to close TLS connection", "error", err)
		}
		return trace.Wrap(err)
	}

	isDatabaseConnection, err := dbutils.IsDatabaseConnection(tlsConn.ConnectionState())
	if err != nil {
		p.log.DebugContext(ctx, "Failed to check if connection is database connection", "error", err)
	}
	if !isDatabaseConnection {
		return trace.BadParameter("not database connection")
	}
	return trace.Wrap(p.handleDatabaseConnection(ctx, tlsConn, info))
}

func (p *Proxy) getHandlerDescBaseOnClientHelloMsg(clientHelloInfo *tls.ClientHelloInfo) (*HandlerDecs, error) {
	if shouldRouteToKubeService(clientHelloInfo.ServerName) {
		if p.cfg.Router.kubeHandler == nil {
			return nil, trace.BadParameter("received kube request but k8 service is disabled")
		}
		return p.cfg.Router.kubeHandler, nil
	}
	return p.getHandleDescBasedOnALPNVal(clientHelloInfo)
}

// getHandleDescBasedOnALPNVal returns the HandlerDesc base on ALPN field read from ClientHelloInfo message.
func (p *Proxy) getHandleDescBasedOnALPNVal(clientHelloInfo *tls.ClientHelloInfo) (*HandlerDecs, error) {
	// Add the HTTP protocol as a default protocol. If client supported
	// list is empty the default HTTP handler will be returned.
	clientProtocols := clientHelloInfo.SupportedProtos
	if len(clientProtocols) == 0 {
		clientProtocols = []string{string(common.ProtocolHTTP)}
	}

	for _, v := range clientProtocols {
		protocol := common.Protocol(v)
		if common.IsDBTLSProtocol(protocol) {
			return &HandlerDecs{
				MatchFunc:           MatchByProtocol(protocol),
				HandlerWithConnInfo: p.databaseHandlerWithTLSTermination,
				ForwardTLS:          false,
			}, nil
		}

		for _, h := range p.cfg.Router.alpnHandlers {
			if ok := h.MatchFunc(clientHelloInfo.ServerName, string(protocol)); ok {
				return h, nil
			}
		}
	}
	return nil, trace.BadParameter(
		"failed to find ALPN handler based on received client supported protocols %q", clientProtocols)
}

func shouldRouteToKubeService(sni string) bool {
	return strings.HasPrefix(sni, constants.KubeTeleportProxyALPNPrefix)
}

// Close the Proxy server.
func (p *Proxy) Close() error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	if err := p.cfg.Listener.Close(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// MakeConnectionHandler creates a ConnectionHandler which provides a callback
// to handle incoming connections by this ALPN proxy server.
func (p *Proxy) MakeConnectionHandler(defaultOverride *tls.Config) ConnectionHandler {
	return func(ctx context.Context, conn net.Conn) error {
		return p.handleConn(ctx, conn, defaultOverride)
	}
}

// ConnectionHandler defines a function for serving incoming connections.
type ConnectionHandler func(ctx context.Context, conn net.Conn) error

// ConnectionHandlerWrapper is a wrapper of ConnectionHandler. This wrapper is
// mainly used as a placeholder to resolve circular dependencies.
type ConnectionHandlerWrapper struct {
	h ConnectionHandler
}

// Set updates inner ConnectionHandler to use.
func (w *ConnectionHandlerWrapper) Set(h ConnectionHandler) {
	w.h = h
}

// HandleConnection implements ConnectionHandler.
func (w *ConnectionHandlerWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	if w.h == nil {
		return trace.NotFound("missing ConnectionHandler")
	}
	return w.h(ctx, conn)
}

// isConnRemoteError checks if an error origin is from the client side like:
// TLS client side handshake error when the telepot proxy CA is not recognized by a client.
func isConnRemoteError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "remote error"
}
