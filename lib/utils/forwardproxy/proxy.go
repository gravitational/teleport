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

package forwardproxy

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

// IsConnectRequest returns true if the request is a CONNECT tunnel request.
func IsConnectRequest(req *http.Request) bool {
	return req.Method == "CONNECT"
}

// Config is the config for forward proxy.
type Config struct {
	// Protocol is the protocol of the requests being tunneled.
	Protocol string
	// Listener is listener running on local machine.
	Listener net.Listener
	// Receivers is a list of receivers that receive from this proxy.
	Receivers []Receiver
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
func (c *Config) CheckAndSetDefaults() error {
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

// ProxyServer is a forward proxy that serves CONNECT tunnel requests from
// clients using HTTPS_PROXY.
type ProxyServer struct {
	cfg Config
}

// New creates a new forward proxy server.
func New(cfg Config) (*ProxyServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ProxyServer{
		cfg: cfg,
	}, nil
}

// Start starts serving the requests.
func (s *ProxyServer) Start() error {
	err := http.Serve(s.cfg.Listener, s)
	if err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// GetAddr returns the listener address.
func (s *ProxyServer) GetAddr() string {
	return s.cfg.Listener.Addr().String()
}

// Close closes the server.
func (s *ProxyServer) Close() error {
	if err := s.cfg.Listener.Close(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ServeHTTP serves the HTTP request. Implements http.Handler.
func (s *ProxyServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !IsConnectRequest(req) {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	clientConn := hijackClientConnection(rw)
	if clientConn == nil {
		s.cfg.Log.Errorf("Failed to hijack client connection.")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	for _, receiver := range s.cfg.Receivers {
		if receiver.Want(req) {
			err := s.forwardClientToHost(clientConn, receiver.GetAddr(), req)
			if err != nil {
				s.cfg.Log.WithError(err).Errorf("Failed to handle forward request for %q.", req.Host)
				writeHeader(clientConn, req.Proto, http.StatusInternalServerError)
			}
			return
		}
	}

	s.handleUnwantedRequest(clientConn, req)
}

// handleUnwantedRequest handles the request when no receivers want it.
func (s *ProxyServer) handleUnwantedRequest(clientConn net.Conn, req *http.Request) {
	if s.cfg.DropUnwantedRequest {
		s.cfg.Log.Debugf("Dropped forward request for %q.", req.Host)
		writeHeader(clientConn, req.Proto, http.StatusBadRequest)
		return
	}

	// Honors system proxy if exists. Assuming original request is a HTTPS
	// request.
	systemProxyCheck := httpproxy.FromEnvironment().ProxyFunc()
	systemProxyURL, err := systemProxyCheck(&url.URL{
		Host:   req.Host,
		Scheme: s.cfg.Protocol,
	})
	if err == nil && systemProxyURL != nil {
		err = s.forwardClientToSystemProxy(clientConn, systemProxyURL, req)
		if err != nil {
			s.cfg.Log.WithError(err).Errorf("Failed to forward request for %q through system proxy %v.", req.Host, systemProxyURL)
			writeHeader(clientConn, req.Proto, http.StatusInternalServerError)
		}
		return
	}

	// Forward to the original host.
	err = s.forwardClientToHost(clientConn, req.Host, req)
	if err != nil {
		s.cfg.Log.WithError(err).Errorf("Failed to forward request for %q.", req.Host)
		writeHeader(clientConn, req.Proto, http.StatusInternalServerError)
	}
}

// forwardClientToHost forwards client connection to provided host.
func (s *ProxyServer) forwardClientToHost(clientConn net.Conn, host string, req *http.Request) error {
	serverConn, err := net.Dial("tcp", host)
	if err != nil {
		return trace.Wrap(err)
	}

	defer serverConn.Close()

	// Let client know we are ready for proxying.
	if err := writeHeader(clientConn, req.Proto, http.StatusOK); err != nil {
		return trace.Wrap(err)
	}
	return s.startTunnel(clientConn, serverConn, req)
}

// forwardClientToSystemProxy forwards client connection to provided system proxy.
func (s *ProxyServer) forwardClientToSystemProxy(clientConn net.Conn, systemProxyURL *url.URL, req *http.Request) error {
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
			InsecureSkipVerify: s.cfg.InsecureSystemProxy,
		})
		if err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.BadParameter("unsupported system proxy %v", systemProxyURL)
	}

	defer serverConn.Close()
	s.cfg.Log.Debugf("Connected to system proxy %v.", systemProxyURL)

	// Send original CONNECT request to system proxy.
	connectRequestBytes, err := httputil.DumpRequest(req, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err = serverConn.Write(connectRequestBytes); err != nil {
		return trace.Wrap(err)
	}

	return s.startTunnel(clientConn, serverConn, req)
}

// startTunnel starts streaming between client and remote server.
func (s *ProxyServer) startTunnel(clientConn, serverConn net.Conn, req *http.Request) error {
	s.cfg.Log.Debugf("Starting forwarding request for %q", req.Host)

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
