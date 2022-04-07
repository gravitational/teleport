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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"
)

// IsConnectRequest returns true if the request is a HTTP CONNECT tunnel
// request.
//
// https://datatracker.ietf.org/doc/html/rfc7231#section-4.3.6
func IsConnectRequest(req *http.Request) bool {
	return req.Method == "CONNECT"
}

// ConnectRequestHandler defines handler for handling CONNECT requests.
type ConnectRequestHandler interface {
	// Match returns true if this handler wants to handle the provided request.
	Match(host *http.Request) bool

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

// Start starts serving requests.
func (p *ForwardProxy) Start() error {
	err := http.Serve(p.cfg.Listener, p)
	if err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// Close closes the forward proxy.
func (p *ForwardProxy) Close() error {
	if err := utils.CloseListener(p.cfg.Listener); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAddr returns the listener address.
func (l *ForwardProxy) GetAddr() string {
	return l.cfg.Listener.Addr().String()
}

// ServeHTTP serves HTTP requests. Implements http.Handler.
func (p *ForwardProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Clients usually send plain HTTP requests directly without CONNECT tunnel
	// when proxying HTTP requests. These requests are rejected as only
	// requests through CONNECT tunnel are allowed.
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

	writeHeaderToHijackedConnection(clientConn, req.Proto, http.StatusBadRequest)
}

// ForwardToHostHandlerConfig is the config for ForwardToHostHandler.
type ForwardToHostHandlerConfig struct {
	// Match returns true if this handler wants to handle the provided request.
	MatchFunc func(req *http.Request) bool

	// Host to forward the request to. If empty, request is forwarded to its
	// original host.
	Host string
}

// ForwardToHostHandler is a ConnectRequestHandler that forwards requests to
// designated host.
type ForwardToHostHandler struct {
	cfg ForwardToHostHandlerConfig
}

// NewForwardToHostHandler creates a new ForwardToHostHandler.
func NewForwardToHostHandler(cfg ForwardToHostHandlerConfig) *ForwardToHostHandler {
	return &ForwardToHostHandler{
		cfg: cfg,
	}
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
		log.WithError(err).Errorf("Failed to connect to host %q", host)
		writeHeaderToHijackedConnection(clientConn, req.Proto, http.StatusInternalServerError)
		return
	}
	defer serverConn.Close()

	// Let client know we are ready for proxying.
	if ok := writeHeaderToHijackedConnection(clientConn, req.Proto, http.StatusOK); !ok {
		return
	}

	startForwardProxy(ctx, clientConn, serverConn, req.Host)
}

// NewForwardToOriginalHostHandler creates a new CONNECT request handler that
// forwards all requests to their original hosts.
func NewForwardToOriginalHostHandler() *ForwardToHostHandler {
	return NewForwardToHostHandler(ForwardToHostHandlerConfig{
		MatchFunc: func(req *http.Request) bool {
			return true
		},
	})
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
	if c.SystemProxyFunc == nil {
		c.SystemProxyFunc = httpproxy.FromEnvironment().ProxyFunc()
	}
}

// ForwardToSystemProxyHandler is a ConnectRequestHandler that forwards
// requests to system proxy.
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
		// Should not happen assuming Match is called beforehand.
		writeHeaderToHijackedConnection(clientConn, req.Proto, http.StatusBadRequest)
		return
	}

	serverConn, err := h.connectToSystemProxy(systemProxyURL)
	if err != nil {
		log.WithError(err).Errorf("Failed to connect to system proxy %q", systemProxyURL.Host)
		writeHeaderToHijackedConnection(clientConn, req.Proto, http.StatusBadGateway)
		return
	}

	defer serverConn.Close()
	log.Debugf("Connected to system proxy %v.", systemProxyURL)

	// Send original CONNECT request to system proxy.
	if err = req.WriteProxy(serverConn); err != nil {
		log.WithError(err).Errorf("Failed to send CONNTECT request to system proxy %q", systemProxyURL.Host)
		writeHeaderToHijackedConnection(clientConn, req.Proto, http.StatusBadGateway)
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
	log.Debugf("Started forwarding request for %q", host)
	defer log.Debugf("Stopped forwarding request for %q", host)

	closeContext, closeCancel := context.WithCancel(ctx)
	defer closeCancel()

	// Force close connections when close context is done.
	go func() {
		<-closeContext.Done()

		clientConn.Close()
		serverConn.Close()
	}()

	wg := sync.WaitGroup{}
	wg.Add(2)
	stream := func(reader, writer net.Conn) {
		_, err := io.Copy(reader, writer)
		if err != nil && !utils.IsOKNetworkError(err) {
			log.WithError(err).Errorf("Failed to stream from %q to %q", reader.LocalAddr(), writer.LocalAddr())
		}
		if readerConn, ok := reader.(*net.TCPConn); ok {
			readerConn.CloseRead()
		}
		if writerConn, ok := writer.(*net.TCPConn); ok {
			writerConn.CloseWrite()
		}
		wg.Done()
	}
	go stream(clientConn, serverConn)
	go stream(serverConn, clientConn)
	wg.Wait()
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
func writeHeaderToHijackedConnection(conn net.Conn, protocol string, statusCode int) bool {
	formatted := fmt.Sprintf("%s %d %s\r\n\r\n", protocol, statusCode, http.StatusText(statusCode))
	_, err := conn.Write([]byte(formatted))
	if err != nil && !utils.IsOKNetworkError(err) {
		log.WithError(err).Errorf("Failed to write status code %d to client connection", statusCode)
		return false
	}
	return true
}
