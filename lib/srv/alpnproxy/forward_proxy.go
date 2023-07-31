/*
Copyright 2022 Gravitational, Inc.

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
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport/api/types"
	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/api/utils/gcp"
	"github.com/gravitational/teleport/lib/utils"
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
	err := http.Serve(p.cfg.Listener, p)
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
		log.Error("Failed to hijack client connection.")
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
	return awsapiutils.IsAWSEndpoint(req.Host)
}

// MatchAzureRequests is a MatchFunc that returns true if request is an Azure API
// request.
func MatchAzureRequests(req *http.Request) bool {
	h := req.URL.Hostname()
	return azure.IsAzureEndpoint(h) || types.TeleportAzureMSIEndpoint == h
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
		log.WithError(err).Errorf("Failed to connect to host %q.", host)
		writeHeaderToHijackedConnection(clientConn, req, http.StatusServiceUnavailable)
		return
	}
	defer serverConn.Close()

	// Send OK to client to let it know the tunnel is ready.
	if ok := writeHeaderToHijackedConnection(clientConn, req, http.StatusOK); !ok {
		return
	}

	startForwardProxy(ctx, clientConn, serverConn, req.Host)
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

	serverConn, err := h.connectToSystemProxy(systemProxyURL)
	if err != nil {
		log.WithError(err).Errorf("Failed to connect to system proxy %q.", systemProxyURL.Host)
		writeHeaderToHijackedConnection(clientConn, req, http.StatusBadGateway)
		return
	}

	defer serverConn.Close()
	log.Debugf("Connected to system proxy %v.", systemProxyURL)

	// Send original CONNECT request to system proxy.
	if err = req.WriteProxy(serverConn); err != nil {
		log.WithError(err).Errorf("Failed to send CONNECT request to system proxy %q.", systemProxyURL.Host)
		writeHeaderToHijackedConnection(clientConn, req, http.StatusBadGateway)
		return
	}

	startForwardProxy(ctx, clientConn, serverConn, req.Host)
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
		log.WithError(err).Debugf("Failed to get system proxy.")
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
func startForwardProxy(ctx context.Context, clientConn, serverConn net.Conn, host string) {
	log.Debugf("Started forwarding request for %q.", host)
	defer log.Debugf("Stopped forwarding request for %q.", host)

	if err := utils.ProxyConn(ctx, clientConn, serverConn); err != nil {
		log.WithError(err).Errorf("Failed to proxy between %q and %q.", clientConn.LocalAddr(), serverConn.LocalAddr())
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
		log.WithError(err).Errorf("Failed to write status code %d to client connection.", statusCode)
		return false
	}
	return true
}
