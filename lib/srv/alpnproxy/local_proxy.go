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
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jackc/pgproto3/v2"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	commonApp "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// LocalProxy allows upgrading incoming connection to TLS where custom TLS values are set SNI ALPN and
// updated connection is forwarded to remote ALPN SNI teleport proxy service.
type LocalProxy struct {
	cfg     LocalProxyConfig
	context context.Context
	cancel  context.CancelFunc
	certMu  sync.RWMutex
}

// LocalProxyConfig is configuration for LocalProxy.
type LocalProxyConfig struct {
	// RemoteProxyAddr is the upstream destination address of remote ALPN proxy service.
	RemoteProxyAddr string
	// Protocol set for the upstream TLS connection.
	Protocols []common.Protocol
	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool
	// Listener is listener running on local machine.
	Listener net.Listener
	// SNI is a ServerName value set for upstream TLS connection.
	SNI string
	// ParentContext is a parent context, used to signal global closure>
	ParentContext context.Context
	// Cert are the client certificates used to connect to the remote Teleport Proxy.
	Cert tls.Certificate
	// RootCAs overwrites the root CAs used in tls.Config if specified.
	RootCAs *x509.CertPool
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
	// Middleware provides callback functions to the local proxy.
	Middleware LocalProxyMiddleware
	// Middleware provides callback functions to the local proxy running in HTTP mode.
	HTTPMiddleware LocalProxyHTTPMiddleware
	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// Log is the Logger.
	Log *slog.Logger
	// CheckCertNeeded determines if the local proxy will check if it should
	// load cert for dialing upstream. Defaults to false, in which case
	// the local proxy will always use whatever cert it has to dial upstream.
	// For example postgres cancel requests are not sent with TLS even if the
	// postgres client was configured to use client cert, so a local proxy
	// needs to always have cert loaded for postgres in case it is needed,
	// but only use the cert as needed.
	CheckCertNeeded bool
	// verifyUpstreamConnection is a callback function to verify upstream connection state.
	verifyUpstreamConnection func(tls.ConnectionState) error
	// onSetCert is a callback when lp.SetCert is called.
	onSetCert func(tls.Certificate)
}

// LocalProxyMiddleware provides callback functions for LocalProxy.
type LocalProxyMiddleware interface {
	// OnNewConnection is a callback triggered when a new downstream connection is
	// accepted by the local proxy. If an error is returned, the connection will be closed
	// by the local proxy.
	OnNewConnection(ctx context.Context, lp *LocalProxy) error
	// OnStart is a callback triggered when the local proxy starts.
	OnStart(ctx context.Context, lp *LocalProxy) error
}

// CheckAndSetDefaults verifies the constraints for LocalProxyConfig.
func (cfg *LocalProxyConfig) CheckAndSetDefaults() error {
	if cfg.RemoteProxyAddr == "" {
		return trace.BadParameter("missing remote proxy address")
	}
	if len(cfg.Protocols) == 0 {
		return trace.BadParameter("missing protocol")
	}
	if cfg.ParentContext == nil {
		return trace.BadParameter("missing parent context")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Log == nil {
		cfg.Log = slog.With(teleport.ComponentKey, "localproxy")
	}

	// set tls cert chain leaf to reduce per-handshake processing.
	if len(cfg.Cert.Certificate) > 0 {
		if err := utils.InitCertLeaf(&cfg.Cert); err != nil {
			return trace.Wrap(err)
		}
	}

	// If SNI is not set, default to cfg.RemoteProxyAddr.
	if cfg.SNI == "" {
		address, err := utils.ParseAddr(cfg.RemoteProxyAddr)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SNI = address.Host()
	}

	// Update the list with Ping protocols.
	cfg.Protocols = common.WithPingProtocols(cfg.Protocols)
	return nil
}

// NewLocalProxy creates a new instance of LocalProxy.
func NewLocalProxy(cfg LocalProxyConfig, opts ...LocalProxyConfigOpt) (*LocalProxy, error) {
	for _, applyOpt := range opts {
		if err := applyOpt(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.ParentContext)
	return &LocalProxy{
		cfg:     cfg,
		context: ctx,
		cancel:  cancel,
	}, nil
}

// Start starts the LocalProxy.
func (l *LocalProxy) Start(ctx context.Context) error {
	if l.cfg.Middleware != nil {
		if err := l.cfg.Middleware.OnStart(ctx, l); err != nil {
			return trace.Wrap(err)
		}
	}

	if l.cfg.HTTPMiddleware != nil {
		return trace.Wrap(l.startHTTPAccessProxy(ctx))
	}

	return trace.Wrap(l.start(ctx))
}

// start starts the LocalProxy for raw TCP or raw TLS (non-HTTP) connections.
func (l *LocalProxy) start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := l.cfg.Listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			l.cfg.Log.ErrorContext(ctx, "Failed to accept client connection", "error", err)
			return trace.Wrap(err)
		}
		l.cfg.Log.DebugContext(ctx, "Accepted downstream connection")

		if l.cfg.Middleware != nil {
			if err := l.cfg.Middleware.OnNewConnection(ctx, l); err != nil {
				l.cfg.Log.ErrorContext(ctx, "Middleware failed to handle client connection", "error", err)
				if err := conn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
					l.cfg.Log.DebugContext(ctx, "Failed to close client connection", "error", err)
				}
				continue
			}
		}

		go func() {
			if err := l.handleDownstreamConnection(ctx, conn); err != nil {
				if utils.IsOKNetworkError(err) {
					return
				}
				l.cfg.Log.ErrorContext(ctx, "Failed to handle connection", "error", err)
			}
		}()
	}
}

// GetAddr returns the LocalProxy listener address.
func (l *LocalProxy) GetAddr() string {
	return l.cfg.Listener.Addr().String()
}

// handleDownstreamConnection proxies the downstreamConn (connection established to the local proxy) and forward the
// traffic to the upstreamConn (TLS connection to remote host).
func (l *LocalProxy) handleDownstreamConnection(ctx context.Context, downstreamConn net.Conn) error {
	defer downstreamConn.Close()

	cert, downstreamConn, err := l.getCertForConn(downstreamConn)
	if err != nil {
		return trace.Wrap(err)
	}

	upstreamConn, err := dialALPNMaybePing(ctx, l.cfg.RemoteProxyAddr, l.getALPNDialerConfig(cert))
	if err != nil {
		return trace.Wrap(err)
	}
	defer upstreamConn.Close()

	return trace.Wrap(utils.ProxyConn(ctx, downstreamConn, upstreamConn))
}

// HandleTCPConnector injects an inbound TCP connection (via [connector]) that doesn't come in through any
// net.Listener. It is used by VNet to share the common local proxy code. [connector] should be called as late
// as possible so that in case of error VNet clients get a failed TCP dial (with RST) rather than a successful
// dial with an immediately closed connection.
func (l *LocalProxy) HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error {
	if l.cfg.Middleware != nil {
		if err := l.cfg.Middleware.OnNewConnection(ctx, l); err != nil {
			return trace.Wrap(err, "middleware failed to handle client connection")
		}
	}

	cert, err := l.getCertWithoutConn()
	if err != nil {
		return trace.Wrap(err)
	}

	upstreamConn, err := dialALPNMaybePing(ctx, l.cfg.RemoteProxyAddr, l.getALPNDialerConfig(cert))
	if err != nil {
		return trace.Wrap(err)
	}
	defer upstreamConn.Close()

	downstreamConn, err := connector()
	if err != nil {
		return trace.Wrap(err, "getting downstream conn")
	}
	defer downstreamConn.Close()

	return trace.Wrap(utils.ProxyConn(ctx, downstreamConn, upstreamConn))
}

// dialALPNMaybePing is a helper to dial using an ALPNDialer, it wraps the tls conn in a ping conn if
// necessary, and returns a net.Conn if successful.
func dialALPNMaybePing(ctx context.Context, addr string, cfg client.ALPNDialerConfig) (net.Conn, error) {
	tlsConn, err := client.DialALPN(ctx, addr, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if common.IsPingProtocol(common.Protocol(tlsConn.ConnectionState().NegotiatedProtocol)) {
		return pingconn.NewTLS(tlsConn), nil
	}
	return tlsConn, nil
}

func (l *LocalProxy) Close() error {
	l.cancel()
	if l.cfg.Listener != nil {
		if err := l.cfg.Listener.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (l *LocalProxy) getALPNDialerConfig(certs ...tls.Certificate) client.ALPNDialerConfig {
	return client.ALPNDialerConfig{
		ALPNConnUpgradeRequired: l.cfg.ALPNConnUpgradeRequired,
		TLSConfig: &tls.Config{
			NextProtos:         common.ProtocolsToString(l.cfg.Protocols),
			InsecureSkipVerify: l.cfg.InsecureSkipVerify,
			ServerName:         l.cfg.SNI,
			Certificates:       certs,
			RootCAs:            l.cfg.RootCAs,
		},
	}
}

func (l *LocalProxy) makeHTTPReverseProxy(certs ...tls.Certificate) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(outReq *http.Request) {
			outReq.URL.Scheme = "https"
			outReq.URL.Host = l.cfg.RemoteProxyAddr
		},
		ModifyResponse: func(response *http.Response) error {
			errHeader := response.Header.Get(commonApp.TeleportAPIErrorHeader)
			if errHeader != "" {
				// TODO: find a cleaner way of formatting the error.
				errHeader = strings.Replace(errHeader, " \t", "\n\t", -1)
				errHeader = strings.Replace(errHeader, " User Message:", "\n\n\tUser Message:", -1)
				l.cfg.Log.WarnContext(response.Request.Context(), "Server response contained an error header", "error_header", errHeader)
			}
			for _, infoHeader := range response.Header.Values(commonApp.TeleportAPIInfoHeader) {
				l.cfg.Log.InfoContext(response.Request.Context(), "Server response info", "header", infoHeader)
			}

			if err := l.cfg.HTTPMiddleware.HandleResponse(response); err != nil {
				return trace.Wrap(err)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			l.cfg.Log.WarnContext(r.Context(), "Failed to handle request ", "error", err, "method", r.Method, "url", logutils.StringerAttr(r.URL))
			code := trace.ErrorToCode(err)
			http.Error(w, http.StatusText(code), code)
		},
		Transport: &http.Transport{
			DialTLSContext: client.NewALPNDialer(l.getALPNDialerConfig(certs...)).DialContext,
		},
	}
}

// startHTTPAccessProxy starts the local HTTP access proxy.
func (l *LocalProxy) startHTTPAccessProxy(ctx context.Context) error {
	if err := l.cfg.HTTPMiddleware.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	l.cfg.Log.InfoContext(ctx, "Starting HTTP access proxy")
	defer l.cfg.Log.InfoContext(ctx, "HTTP access proxy stopped")

	server := &http.Server{
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if l.cfg.Middleware != nil {
				if err := l.cfg.Middleware.OnNewConnection(ctx, l); err != nil {
					l.cfg.Log.ErrorContext(ctx, "Middleware failed to handle client request", "error", err)
					trace.WriteError(rw, trace.Wrap(err))
					return
				}
			}

			if l.cfg.HTTPMiddleware.HandleRequest(rw, req) {
				return
			}

			// Requests from forward proxy have original hostnames instead of
			// localhost. Set appropriate header to keep this information.
			if addr, err := utils.ParseAddr(req.Host); err == nil && !addr.IsLocal() {
				req.Header.Set("X-Forwarded-Host", req.Host)
			} else { // ensure that there is no client provided X-Forwarded-Host
				req.Header.Del("X-Forwarded-Host")
			}

			proxy, err := l.getHTTPReverseProxyForReq(req)
			if err != nil {
				l.cfg.Log.WarnContext(ctx, "Failed to get reverse proxy", "error", err)
				trace.WriteError(rw, trace.Wrap(err))
				return
			}

			proxy.ServeHTTP(rw, req)
		}),
	}

	// Shut down the server when the context is done
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	// Use the custom server to listen and serve
	err := server.Serve(l.cfg.Listener)
	if err != nil && err != http.ErrServerClosed && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (l *LocalProxy) getHTTPReverseProxyForReq(req *http.Request) (*httputil.ReverseProxy, error) {
	certs, err := l.cfg.HTTPMiddleware.OverwriteClientCerts(req)
	if trace.IsNotImplemented(err) {
		return l.makeHTTPReverseProxy(l.getCert()), nil
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	l.cfg.Log.DebugContext(req.Context(), "overwrote certs")
	return l.makeHTTPReverseProxy(certs...), nil
}

// getCert returns the local proxy's configured TLS certificate.
func (l *LocalProxy) getCert() tls.Certificate {
	l.certMu.RLock()
	defer l.certMu.RUnlock()
	return l.cfg.Cert
}

// CheckDBCert checks the proxy certificates for expiration and that the cert subject matches a database route.
func (l *LocalProxy) CheckDBCert(ctx context.Context, dbRoute tlsca.RouteToDatabase) error {
	l.cfg.Log.DebugContext(ctx, "checking local proxy database certs")
	l.certMu.RLock()
	defer l.certMu.RUnlock()

	if len(l.cfg.Cert.Certificate) == 0 {
		return trace.NotFound("local proxy has no TLS certificates configured")
	}

	cert, err := utils.TLSCertLeaf(l.cfg.Cert)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check for cert expiration.
	if err := utils.VerifyCertificateExpiry(cert, l.cfg.Clock); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(CheckDBCertSubject(cert, dbRoute))
}

// CheckCertExpiry checks the proxy certificates for expiration.
func (l *LocalProxy) CheckCertExpiry(ctx context.Context) error {
	l.cfg.Log.DebugContext(ctx, "checking local proxy certs")
	l.certMu.RLock()
	defer l.certMu.RUnlock()

	if len(l.cfg.Cert.Certificate) == 0 {
		return trace.NotFound("local proxy has no TLS certificates configured")
	}

	cert, err := utils.TLSCertLeaf(l.cfg.Cert)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(utils.VerifyCertificateExpiry(cert, l.cfg.Clock))
}

// CheckDBCertSubject checks if the route to the database from the cert matches the provided route in
// terms of username and database (if present).
func CheckDBCertSubject(cert *x509.Certificate, dbRoute tlsca.RouteToDatabase) error {
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}
	if dbRoute.Username != "" && dbRoute.Username != identity.RouteToDatabase.Username {
		return trace.Errorf("certificate subject is for user %s, but need %s",
			identity.RouteToDatabase.Username, dbRoute.Username)
	}
	if dbRoute.Database != "" && dbRoute.Database != identity.RouteToDatabase.Database {
		return trace.Errorf("certificate subject is for database name %s, but need %s",
			identity.RouteToDatabase.Database, dbRoute.Database)
	}

	return nil
}

// SetCert sets the local proxy's configured TLS certificates.
func (l *LocalProxy) SetCert(cert tls.Certificate) {
	l.certMu.Lock()
	defer l.certMu.Unlock()
	l.cfg.Cert = cert

	// Callback, if any.
	if l.cfg.onSetCert != nil {
		l.cfg.onSetCert(cert)
	}
}

// getCertForConn determines if certificates should be used when dialing
// upstream to proxy a new downstream connection.
// After calling getCertForConn function, the returned
// net.Conn should be used for further operation.
func (l *LocalProxy) getCertForConn(downstreamConn net.Conn) (tls.Certificate, net.Conn, error) {
	if !l.cfg.CheckCertNeeded {
		return l.getCert(), downstreamConn, nil
	}
	if l.isPostgresProxy() {
		// `psql` cli doesn't send cancel requests with SSL, unfortunately.
		// This is a problem when the local proxy has no certs configured,
		// because normally the client is responsible for connecting with
		// TLS certificates.
		// So when the local proxy has no certs configured, we inspect
		// the connection to see if it is a postgres cancel request and
		// load certs for the connection.
		startupMessage, conn, err := peekPostgresStartupMessage(downstreamConn)
		if err != nil {
			return tls.Certificate{}, nil, trace.Wrap(err)
		}
		_, isCancelReq := startupMessage.(*pgproto3.CancelRequest)
		if !isCancelReq {
			return tls.Certificate{}, conn, nil
		}
		cert := l.getCert()
		if len(cert.Certificate) == 0 {
			return tls.Certificate{}, nil, trace.NotFound("local proxy has no TLS certificates configured")
		}
		return cert, conn, nil
	}
	return tls.Certificate{}, downstreamConn, nil
}

func (l *LocalProxy) getCertWithoutConn() (tls.Certificate, error) {
	if l.cfg.CheckCertNeeded {
		return tls.Certificate{}, trace.BadParameter("getCertWithoutConn called while CheckCertNeeded is true: this is a bug")
	}
	return l.getCert(), nil
}

func (l *LocalProxy) isPostgresProxy() bool {
	for _, proto := range common.ProtocolsToString(l.cfg.Protocols) {
		if strings.HasPrefix(proto, string(common.ProtocolPostgres)) {
			return true
		}
	}
	return false
}

// peekPostgresStartupMessage reads and returns the startup message from a
// connection. After calling peekPostgresStartupMessage function, the returned
// net.Conn should be used for further operation.
func peekPostgresStartupMessage(conn net.Conn) (pgproto3.FrontendMessage, net.Conn, error) {
	// buffer the bytes we read so we can peek at the conn.
	buff := new(bytes.Buffer)
	// wrap the conn in a read-only conn to be sure the conn is not written to.
	rConn := readOnlyConn{reader: io.TeeReader(conn, buff)}
	// backend acts as a server for the Postgres wire protocol.
	backend := pgproto3.NewBackend(pgproto3.NewChunkReader(rConn), rConn)
	startupMessage, err := backend.ReceiveStartupMessage()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return startupMessage, newBufferedConn(conn, buff), nil
}
