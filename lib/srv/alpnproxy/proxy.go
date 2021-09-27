/*
Copyright 2021 Gravitational, Inc.

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

package alpnproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	dbcommon "github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	// KubeSNIPrefix is a SNI Kubernetes prefix used for distinguishing the Kubernetes HTTP traffic.
	KubeSNIPrefix = "kube"
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
	Log logrus.FieldLogger
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
	AccessPoint auth.AccessPoint
	// ClusterName is the name of the teleport cluster.
	ClusterName string
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

// MatchByProtocol creates match function based on client TLS ALPN protocol.
func MatchByProtocol(protocols ...common.Protocol) MatchFunc {
	m := make(map[common.Protocol]struct{})
	for _, v := range protocols {
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

// CheckAndSetDefaults verifies the constraints for Router.
func (r *Router) CheckAndSetDefaults() error {
	for _, v := range r.alpnHandlers {
		if err := v.CheckAndSetDefaults(); err != nil {
			return err
		}
	}
	return nil
}

// AddKubeHandler adds the handle for Kubernetes protocol (distinguishable by  "kube." SNI prefix).
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
	// TLS termination to the protocol handler (Used in Kube handler case)
	ForwardTLS bool
	// MatchFunc is a routing route match function based on ALPN SNI TLS values.
	// If is evaluated to true the current HandleDesc will be used
	// for connection handling.
	MatchFunc MatchFunc
}

func (h *HandlerDecs) CheckAndSetDefaults() error {
	if h.Handler != nil && h.HandlerWithConnInfo != nil {
		return trace.BadParameter("can't create route with both Handler and HandlerWithConnInfo handlers")
	}
	if h.MatchFunc == nil {
		return trace.BadParameter("missing param MatchFunc")
	}
	return nil
}

func (h *HandlerDecs) handle(ctx context.Context, conn net.Conn, info ConnectionInfo) error {
	if h.HandlerWithConnInfo != nil {
		return h.HandlerWithConnInfo(ctx, conn, info)
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
	log                logrus.FieldLogger
	cancel             context.CancelFunc
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
		c.Log = logrus.WithField(trace.Component, "alpn:proxy")
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

	if c.IdentityTLSConfig == nil {
		return trace.BadParameter("missing identity tls config")
	}
	c.IdentityTLSConfig = c.IdentityTLSConfig.Clone()
	c.IdentityTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	fn := auth.WithClusterCAs(c.IdentityTLSConfig, c.AccessPoint, c.ClusterName, c.Log)
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
	p.cancel = cancel
	p.cfg.WebTLSConfig.NextProtos = common.ProtocolsToString(p.supportedProtocols)
	for {
		clientConn, err := p.cfg.Listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		go func() {
			// In case of successful handleConn call leave the connection Close() call up to service handler.
			// For example in ReverseTunnel handles connection asynchronously and closing conn after
			// service handler returned will break service logic.
			// https://github.com/gravitational/teleport/blob/master/lib/sshutils/server.go#L397
			if err := p.handleConn(ctx, clientConn); err != nil {
				if err := clientConn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
					p.log.WithError(err).Warnf("Failed to close client connection.")
				}
				p.log.WithError(err).Warnf("Failed to handle client connection.")
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
// 1) Read TLS hello message without TLS termination and returns conn that will be used for further operations.
// 2) Get routing rules for p.Router.Router based on SNI and ALPN fields read in step 1.
// 3) If the selected handler was configured with the ForwardTLS
//    forwards the connection to the handler without TLS termination.
// 4) Trigger TLS handshake and terminates the TLS connection.
// 5) For backward compatibility check RouteToDatabase identity field
//    was set if yes forward to the generic TLS DB handler.
// 6) Forward connection to the handler obtained in step 2.
func (p *Proxy) handleConn(ctx context.Context, clientConn net.Conn) error {
	hello, conn, err := p.readHelloMessageWithoutTLSTermination(clientConn)
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

	if handlerDesc.ForwardTLS {
		return trace.Wrap(handlerDesc.handle(ctx, conn, connInfo))
	}

	tlsConn := tls.Server(conn, p.cfg.WebTLSConfig)
	if err := tlsConn.SetReadDeadline(p.cfg.Clock.Now().Add(p.cfg.ReadDeadline)); err != nil {
		return trace.Wrap(err)
	}
	if err := tlsConn.Handshake(); err != nil {
		return trace.Wrap(err)
	}
	if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
		return trace.Wrap(err)
	}

	isDatabaseConnection, err := dbcommon.IsDatabaseConnection(tlsConn.ConnectionState())
	if err != nil {
		p.log.WithError(err).Debug("Failed to check if connection is database connection.")
	}
	if isDatabaseConnection {
		return trace.Wrap(p.handleDatabaseConnection(ctx, tlsConn, connInfo))
	}
	return trace.Wrap(handlerDesc.handle(ctx, tlsConn, connInfo))
}

// readHelloMessageWithoutTLSTermination allows reading a ClientHelloInfo message without termination of
// incoming TLS connection. After calling readHelloMessageWithoutTLSTermination function a returned
// net.Conn should be used for further operation.
func (p *Proxy) readHelloMessageWithoutTLSTermination(conn net.Conn) (*tls.ClientHelloInfo, net.Conn, error) {
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
	err := tlsConn.Handshake()
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
			p.log.WithError(err).Error("Failed to close TLS connection.")
		}
		return trace.Wrap(err)
	}
	if err := tlsConn.Handshake(); err != nil {
		return trace.Wrap(err)
	}
	if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
		if err := tlsConn.Close(); err != nil {
			p.log.WithError(err).Error("Failed to close TLS connection.")
		}
		return trace.Wrap(err)
	}

	isDatabaseConnection, err := dbcommon.IsDatabaseConnection(tlsConn.ConnectionState())
	if err != nil {
		p.log.WithError(err).Debug("Failed to check if connection is database connection.")
	}
	if !isDatabaseConnection {
		return trace.BadParameter("not database connection")
	}
	return trace.Wrap(p.handleDatabaseConnection(ctx, tlsConn, info))
}

func isDBTLSProtocol(protocol common.Protocol) bool {
	switch protocol {
	case common.ProtocolMongoDB:
		return true
	default:
		return false
	}
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
	protocol := common.ProtocolDefault
	if len(clientHelloInfo.SupportedProtos) != 0 {
		protocol = common.Protocol(clientHelloInfo.SupportedProtos[0])
	}

	if isDBTLSProtocol(protocol) {
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
	return nil, trace.BadParameter("unsupported ALPN protocol %q", protocol)
}

func shouldRouteToKubeService(sni string) bool {
	return strings.HasPrefix(sni, KubeSNIPrefix)
}

// Close the Proxy server.
func (p *Proxy) Close() error {
	if p.cancel != nil {
		p.cancel()
	}
	if err := p.cfg.Listener.Close(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
