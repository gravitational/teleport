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
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"
)

// IsConnectRequest returns true if the request is a HTTP CONNECT tunnel
// request.
func IsConnectRequest(req *http.Request) bool {
	return req.Method == "CONNECT"
}

// ForwardProxyConfig is the config for forward proxy.
type ForwardProxyConfig struct {
	// Protocol is the protocol of the requests being tunneled.
	Protocol string
	// Listener is listener running on local machine.
	Listener net.Listener
	// Receivers is a list of receivers that receive from this proxy.
	Receivers []ForwardProxyReceiver
	// DropUnwantedRequest drops the request if no receivers want the request.
	// If false, forward proxy sends the request to original host.
	DropUnwantedRequest bool
	// InsecureSystemProxy allows insecure system proxy when forwarding
	// unwanted requests.
	InsecureSystemProxy bool
	// Log is the logger.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults checks and sets default config values.
func (c *ForwardProxyConfig) CheckAndSetDefaults() error {
	if c.Protocol == "" {
		c.Protocol = "https"
	}
	if c.Protocol != "https" {
		return trace.BadParameter("only https proxy is supported")
	}
	if c.Listener == nil {
		return trace.BadParameter("missing listener")
	}
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "fwdproxy")
	}
	return nil
}

// ForwardProxy is a forward proxy that serves CONNECT tunnel requests from
// clients using HTTPS_PROXY.
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

// Start starts serving the requests.
func (p *ForwardProxy) Start() error {
	err := http.Serve(p.cfg.Listener, p)
	if err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// GetAddr returns the listener address.
func (p *ForwardProxy) GetAddr() string {
	return p.cfg.Listener.Addr().String()
}

// Close closes the server.
func (p *ForwardProxy) Close() error {
	if err := p.cfg.Listener.Close(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ServeHTTP serves the HTTP request. Implements http.Handler.
func (p *ForwardProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !IsConnectRequest(req) {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	clientConn := hijackClientConnection(rw)
	if clientConn == nil {
		p.cfg.Log.Errorf("Failed to hijack client connection.")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	for _, receiver := range p.cfg.Receivers {
		if receiver.Want(req) {
			err := p.forwardClientToHost(clientConn, receiver.GetAddr(), req)
			if err != nil {
				p.cfg.Log.WithError(err).Errorf("Failed to handle forward request for %q.", req.Host)
				writeHeader(clientConn, req.Proto, http.StatusInternalServerError)
			}
			return
		}
	}

	p.handleUnwantedRequest(clientConn, req)
}

// handleUnwantedRequest handles the request when no receivers want it.
func (p *ForwardProxy) handleUnwantedRequest(clientConn net.Conn, req *http.Request) {
	if p.cfg.DropUnwantedRequest {
		p.cfg.Log.Debugf("Dropped forward request for %q.", req.Host)
		writeHeader(clientConn, req.Proto, http.StatusBadRequest)
		return
	}

	// Honors system proxy if exists.
	systemProxyCheck := httpproxy.FromEnvironment().ProxyFunc()
	systemProxyURL, err := systemProxyCheck(&url.URL{
		Host:   req.Host,
		Scheme: p.cfg.Protocol,
	})
	if err == nil && systemProxyURL != nil {
		err = p.forwardClientToSystemProxy(clientConn, systemProxyURL, req)
		if err != nil {
			p.cfg.Log.WithError(err).Errorf("Failed to forward request for %q through system proxy %v.", req.Host, systemProxyURL)
			writeHeader(clientConn, req.Proto, http.StatusInternalServerError)
		}
		return
	}

	// Forward to the original host.
	err = p.forwardClientToHost(clientConn, req.Host, req)
	if err != nil {
		p.cfg.Log.WithError(err).Errorf("Failed to forward request for %q.", req.Host)
		writeHeader(clientConn, req.Proto, http.StatusInternalServerError)
	}
}

// forwardClientToHost forwards client connection to provided host.
func (p *ForwardProxy) forwardClientToHost(clientConn net.Conn, host string, req *http.Request) error {
	serverConn, err := net.Dial("tcp", host)
	if err != nil {
		return trace.Wrap(err)
	}

	defer serverConn.Close()

	// Let client know we are ready for proxying.
	if err := writeHeader(clientConn, req.Proto, http.StatusOK); err != nil {
		return trace.Wrap(err)
	}
	return p.startTunnel(clientConn, serverConn, req)
}

// forwardClientToSystemProxy forwards client connection to provided system proxy.
func (p *ForwardProxy) forwardClientToSystemProxy(clientConn net.Conn, systemProxyURL *url.URL, req *http.Request) error {
	var err error
	var serverConn net.Conn
	switch strings.ToLower(systemProxyURL.Scheme) {
	case "http":
		serverConn, err = net.Dial("tcp", systemProxyURL.Host)
		if err != nil {
			return trace.Wrap(err)
		}

	case "https":
		serverConn, err = tls.Dial("tcp", systemProxyURL.Host, &tls.Config{
			InsecureSkipVerify: p.cfg.InsecureSystemProxy,
		})
		if err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.BadParameter("unsupported system proxy %v", systemProxyURL)
	}

	defer serverConn.Close()
	p.cfg.Log.Debugf("Connected to system proxy %v.", systemProxyURL)

	// Send original CONNECT request to system proxy.
	connectRequestBytes, err := httputil.DumpRequest(req, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err = serverConn.Write(connectRequestBytes); err != nil {
		return trace.Wrap(err)
	}

	return p.startTunnel(clientConn, serverConn, req)
}

// startTunnel starts streaming between client and remote server.
func (p *ForwardProxy) startTunnel(clientConn, serverConn net.Conn, req *http.Request) error {
	p.cfg.Log.Debugf("Starting forwarding request for %q", req.Host)

	wg := sync.WaitGroup{}
	wg.Add(2)
	stream := func(reader, writer net.Conn) {
		_, _ = io.Copy(reader, writer)
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
	return nil
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

// writeHeader writes HTTP status to hijacked client connection.
func writeHeader(conn net.Conn, protocol string, statusCode int) error {
	formatted := fmt.Sprintf("%s %d %s\r\n\r\n", protocol, statusCode, http.StatusText(statusCode))
	_, err := conn.Write([]byte(formatted))
	return trace.Wrap(err)
}
