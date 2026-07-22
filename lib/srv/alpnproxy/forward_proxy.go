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
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/api/utils/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// IsConnectRequest returns true if the request is a HTTP CONNECT tunnel
// request.
//
// https://datatracker.ietf.org/doc/html/rfc7231#section-4.3.6
func IsConnectRequest(req *http.Request) bool {
	return req.Method == http.MethodConnect
}

// ConnectRequestHandler defines handler for handling CONNECT requests.
type ConnectRequestHandler interface {
	// Match returns true if this handler wants to handle the provided request.
	Match(req *http.Request) bool

	// Handle handles the request with provided client connection.
	Handle(ctx context.Context, clientConn net.Conn, req *http.Request)
}

// ForwardProxyConfig is the config for forward proxy server.
type ForwardProxyConfig struct {
	// Listener is the network listener.
	Listener net.Listener
	// CloseContext is the close context.
	CloseContext context.Context
	// Handlers is a list of CONNECT request handlers.
	Handlers []ConnectRequestHandler
}

// CheckAndSetDefaults checks and sets default config values.
func (c *ForwardProxyConfig) CheckAndSetDefaults() error {
	if c.Listener == nil {
		return trace.BadParameter("missing listener")
	}
	if c.CloseContext == nil {
		return trace.BadParameter("missing close context")
	}
	return nil
}

// ForwardProxy is a forward proxy that serves CONNECT tunnel requests.
type ForwardProxy struct {
	cfg ForwardProxyConfig
}

// NewForwardProxy creates a new forward proxy server.
func NewForwardProxy(cfg ForwardProxyConfig) (*ForwardProxy, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &ForwardProxy{
		cfg: cfg,
	}, nil
}

// Start starts serving on the listener.
func (p *ForwardProxy) Start() error {
	server := &http.Server{
		Handler:           p,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
	}
	err := server.Serve(p.cfg.Listener)
	if err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// Close closes the forward proxy.
func (p *ForwardProxy) Close() error {
	if err := p.cfg.Listener.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// GetAddr returns the listener address.
func (p *ForwardProxy) GetAddr() string {
	return p.cfg.Listener.Addr().String()
}

// ServeHTTP serves HTTP requests. Implements http.Handler.
func (p *ForwardProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Only allow CONNECT tunnel requests. Reject if clients send original HTTP
	// requests without CONNECT tunnel.
	if !IsConnectRequest(req) {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	clientConn := hijackClientConnection(rw)
	if clientConn == nil {
		slog.ErrorContext(req.Context(), "Failed to hijack client connection")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	for _, handler := range p.cfg.Handlers {
		if handler.Match(req) {
			handler.Handle(p.cfg.CloseContext, clientConn, req)
			return
		}
	}

	writeHeaderToHijackedConnection(clientConn, req, http.StatusBadRequest)
}

// ForwardToHostHandlerConfig is the config for ForwardToHostHandler.
type ForwardToHostHandlerConfig struct {
	// Match returns true if this handler wants to handle the provided request.
	MatchFunc func(req *http.Request) bool

	// Host is the destination to forward the request to. If empty, the request
	// is forwarded to its original host.
	Host string
}

// SetDefaults sets default config values.
func (c *ForwardToHostHandlerConfig) SetDefaults() {
	if c.MatchFunc == nil {
		c.MatchFunc = MatchAllRequests
	}
}

// MatchAllRequests is a MatchFunc that returns true for all requests.
func MatchAllRequests(req *http.Request) bool {
	return true
}

// MatchAWSRequests is a MatchFunc that returns true if request is an AWS API
// request.
func MatchAWSRequests(req *http.Request) bool {
	return awsapiutils.IsAWSEndpoint(req.Host) &&
		// Avoid proxying SSM session WebSocket requests and let the forward proxy
		// send it directly to AWS.
		//
		// `aws ssm start-session` first calls ssm.<region>.amazonaws.com to get
		// a stream URL and a token. Then it makes a wss connection with the
		// provided token to the provided stream URL. The stream URL looks like:
		// wss://ssmmessages.region.amazonaws.com/v1/data-channel/session-id?stream=(input|output)
		//
		// The wss request currently respects HTTPS_PROXY but does not
		// respect local CA bundle we provided thus causing a failure. The
		// request is not signed with SigV4 either.
		//
		// Reference:
		// https://github.com/aws/session-manager-plugin/
		!isAWSSSMWebsocketRequest(req)
}

func isAWSSSMWebsocketRequest(req *http.Request) bool {
	return awsapiutils.IsAWSEndpoint(req.Host) &&
		strings.HasPrefix(req.Host, "ssmmessages.")
}

// MatchAzureRequests is a MatchFunc that returns true if request is an Azure API
// request.
func MatchAzureRequests(req *http.Request) bool {
	h := req.URL.Hostname()
	return azure.IsAzureEndpoint(h) || types.TeleportAzureMSIEndpoint == h || types.TeleportAzureIdentityEndpoint == h
}

// MatchGCPRequests is a MatchFunc that returns true if request is an GCP API request.
func MatchGCPRequests(req *http.Request) bool {
	h := req.URL.Hostname()
	return gcp.IsGCPEndpoint(h)
}

// ForwardToHostHandler is a CONNECT request handler that forwards requests to
// designated host.
type ForwardToHostHandler struct {
	cfg ForwardToHostHandlerConfig
}

// NewForwardToHostHandler creates a new ForwardToHostHandler.
func NewForwardToHostHandler(cfg ForwardToHostHandlerConfig) *ForwardToHostHandler {
	cfg.SetDefaults()

	return &ForwardToHostHandler{
		cfg: cfg,
	}
}

// NewForwardToOriginalHostHandler creates a new CONNECT request handler that
// forwards all requests to their original hosts.
func NewForwardToOriginalHostHandler() *ForwardToHostHandler {
	return NewForwardToHostHandler(ForwardToHostHandlerConfig{})
}

// Match returns true if this handler wants to handle the provided request.
func (h *ForwardToHostHandler) Match(req *http.Request) bool {
	return h.cfg.MatchFunc(req)
}

// Handle handles the request with provided client connection.
func (h *ForwardToHostHandler) Handle(ctx context.Context, clientConn net.Conn, req *http.Request) {
	host := h.cfg.Host
	if host == "" {
		host = req.Host
	}

	serverConn, err := net.Dial("tcp", host)
	if err != nil {
		slog.ErrorContext(req.Context(), "Failed to connect to host", "error", err, "host", host)
		writeHeaderToHijackedConnection(clientConn, req, http.StatusServiceUnavailable)
		return
	}
	defer serverConn.Close()

	// Send OK to client to let it know the tunnel is ready.
	if ok := writeHeaderToHijackedConnection(clientConn, req, http.StatusOK); !ok {
		return
	}

	startForwardProxy(ctx, clientConn, serverConn, req.Host, slog.Default())
}

// ForwardToSystemProxyHandlerConfig is the config for
// ForwardToSystemProxyHandler.
type ForwardToSystemProxyHandlerConfig struct {
	// TunnelProtocol is the protocol of the requests being tunneled.
	TunnelProtocol string
	// InsecureSystemProxy allows insecure system proxy when forwarding
	// unwanted requests.
	InsecureSystemProxy bool
	// SystemProxyFunc is the function that determines the system proxy URL to
	// use for provided request URL.
	SystemProxyFunc func(reqURL *url.URL) (*url.URL, error)
}

// SetDefaults sets default config values.
func (c *ForwardToSystemProxyHandlerConfig) SetDefaults() {
	if c.TunnelProtocol == "" {
		c.TunnelProtocol = "https"
	}

	// By default, use the HTTPS_PROXY etc. settings from environment where our
	// server is run.
	if c.SystemProxyFunc == nil {
		c.SystemProxyFunc = httpproxy.FromEnvironment().ProxyFunc()
	}
}

// ForwardToSystemProxyHandler is a CONNECT request handler that forwards
// requests to existing system or corporate forward proxies where our server is
// run.
//
// Here "system" is used to differentiate the forward proxy users have outside
// Teleport from our own forward proxy server. The purpose of this handler is
// to honor "system" proxy settings so the requests are forwarded to "system"
// proxies as intended instead of going to their original hosts.
type ForwardToSystemProxyHandler struct {
	cfg ForwardToSystemProxyHandlerConfig
}

// NewForwardToSystemProxyHandler creates a new ForwardToSystemProxyHandler.
func NewForwardToSystemProxyHandler(cfg ForwardToSystemProxyHandlerConfig) *ForwardToSystemProxyHandler {
	cfg.SetDefaults()

	return &ForwardToSystemProxyHandler{
		cfg: cfg,
	}
}

// Match returns true if this handler wants to handle the provided request.
func (h *ForwardToSystemProxyHandler) Match(req *http.Request) bool {
	return h.getSystemProxyURL(req) != nil
}

// Handle handles the request with provided client connection.
func (h *ForwardToSystemProxyHandler) Handle(ctx context.Context, clientConn net.Conn, req *http.Request) {
	systemProxyURL := h.getSystemProxyURL(req)
	if systemProxyURL == nil {
		writeHeaderToHijackedConnection(clientConn, req, http.StatusBadRequest)
		return
	}

	logger := slog.With("proxy", logutils.StringerAttr(systemProxyURL))
	serverConn, err := h.connectToSystemProxy(systemProxyURL)
	if err != nil {
		logger.ErrorContext(req.Context(), "Failed to connect to system proxy", "error", err)
		writeHeaderToHijackedConnection(clientConn, req, http.StatusBadGateway)
		return
	}

	defer serverConn.Close()
	logger.DebugContext(req.Context(), "Connected to system proxy")

	// Send original CONNECT request to system proxy.
	if err = req.WriteProxy(serverConn); err != nil {
		logger.ErrorContext(req.Context(), "Failed to send CONNECT request to system proxy", "error", err)
		writeHeaderToHijackedConnection(clientConn, req, http.StatusBadGateway)
		return
	}

	startForwardProxy(ctx, clientConn, serverConn, req.Host, logger)
}

// getSystemProxyURL returns the system proxy URL.
func (h *ForwardToSystemProxyHandler) getSystemProxyURL(req *http.Request) *url.URL {
	systemProxyURL, err := h.cfg.SystemProxyFunc(&url.URL{
		Host:   req.Host,
		Scheme: h.cfg.TunnelProtocol,
	})
	if err == nil && systemProxyURL != nil {
		return systemProxyURL
	}

	// If error exists, make a log for debugging purpose.
	if err != nil {
		slog.DebugContext(req.Context(), "Failed to get system proxy", "error", err)
	}
	return nil
}

// connectToSystemProxy connects to the system proxy and returns the server
// connection.
func (h *ForwardToSystemProxyHandler) connectToSystemProxy(systemProxyURL *url.URL) (net.Conn, error) {
	var err error
	var serverConn net.Conn
	switch strings.ToLower(systemProxyURL.Scheme) {

	case "http":
		serverConn, err = net.Dial("tcp", systemProxyURL.Host)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case "https":
		serverConn, err = tls.Dial("tcp", systemProxyURL.Host, &tls.Config{
			InsecureSkipVerify: h.cfg.InsecureSystemProxy,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("unsupported system proxy %v", systemProxyURL)
	}
	return serverConn, nil
}

// startForwardProxy starts streaming between client and server.
func startForwardProxy(ctx context.Context, clientConn, serverConn net.Conn, host string, logger *slog.Logger) {
	logger.DebugContext(ctx, "Started forwarding request to host", "host", host)
	defer logger.DebugContext(ctx, "Stopped forwarding request to host", "host", host)

	if err := utils.ProxyConn(ctx, clientConn, serverConn); err != nil {
		logger.ErrorContext(ctx, "Failed to proxy request", "error", err, "client_addr", clientConn.LocalAddr(), "server_addr", serverConn.LocalAddr())
	}
}

// hijackClientConnection hijacks client connection.
func hijackClientConnection(rw http.ResponseWriter) net.Conn {
	hijacker, ok := rw.(http.Hijacker)
	if !ok {
		return nil
	}

	clientConn, _, _ := hijacker.Hijack()
	return clientConn
}

// writeHeaderToHijackedConnection writes HTTP status to hijacked connection.
func writeHeaderToHijackedConnection(conn net.Conn, req *http.Request, statusCode int) bool {
	resp := http.Response{
		StatusCode: statusCode,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
	}
	err := resp.Write(conn)
	if err != nil && !utils.IsOKNetworkError(err) {
		slog.ErrorContext(req.Context(), "Failed to write status code to client connection", "error", err, "status_code", statusCode)
		return false
	}
	return true
}
