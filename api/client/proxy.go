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

package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"golang.org/x/net/proxy"

	"github.com/gravitational/teleport/api/utils/tlsutils"
)

// PROXYHeaderGetter is used if present to get signed PROXY headers to propagate client's IP.
// Used by proxy's web server to make calls on behalf of connected clients.
type PROXYHeaderGetter func() ([]byte, error)

type dialProxyConfig = dialConfig

// DialProxyOption allows setting options as functional arguments to DialProxy.
type DialProxyOption = DialOption

// WithTLSConfig provides the dialer with the TLS config to use when using an
// HTTPS proxy.
func WithTLSConfig(tlsConfig *tls.Config) DialProxyOption {
	return func(cfg *dialProxyConfig) {
		cfg.tlsConfig = tlsConfig
	}
}

// DialProxy creates a connection to a server via an HTTP or SOCKS5 Proxy.
func DialProxy(ctx context.Context, proxyURL *url.URL, addr string, opts ...DialProxyOption) (net.Conn, error) {
	var cfg dialProxyConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var dialer ContextDialer = &net.Dialer{}
	if cfg.proxyHeaderGetter != nil {
		dialer = NewPROXYHeaderDialer(dialer, cfg.proxyHeaderGetter)
	}

	return DialProxyWithDialer(ctx, proxyURL, addr, dialer, opts...)
}

// DialProxyWithDialer creates a connection to a server via an HTTP or SOCKS5
// Proxy using a specified dialer.
func DialProxyWithDialer(
	ctx context.Context,
	proxyURL *url.URL,
	addr string,
	dialer ContextDialer,
	opts ...DialProxyOption,
) (net.Conn, error) {
	if proxyURL == nil {
		return nil, trace.BadParameter("missing proxy url")
	}

	var cfg dialProxyConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	switch proxyURL.Scheme {
	case "http", "https":
		conn, err := dialProxyWithHTTPDialer(ctx, proxyURL, addr, dialer, cfg.tlsConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	case "socks5":
		conn, err := dialProxyWithSOCKSDialer(ctx, proxyURL, addr, dialer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	default:
		return nil, trace.BadParameter("proxy url scheme %q not supported", proxyURL.Scheme)
	}
}

// dialProxyWithHTTPDialer creates a connection to a server via an HTTP Proxy.
func dialProxyWithHTTPDialer(
	ctx context.Context,
	proxyURL *url.URL,
	addr string,
	dialer ContextDialer,
	tlsConfig *tls.Config,
) (net.Conn, error) {
	var conn net.Conn
	var err error
	if proxyURL.Scheme == "https" {
		conn, err = tlsutils.TLSDial(ctx, dialer, "tcp", proxyURL.Host, tlsConfig.Clone())
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", proxyURL.Host)
	}
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	header := make(http.Header)
	if proxyURL.User != nil {
		// dont use User.String() because it performs url encoding (rfc 1738),
		// which we don't want in our header
		password, _ := proxyURL.User.Password()
		// empty user/pass is permitted by the spec. The minimum required is a single colon.
		// see: https://datatracker.ietf.org/doc/html/rfc1945#section-11
		creds := proxyURL.User.Username() + ":" + password
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
		header.Add("Proxy-Authorization", basicAuth)
	}
	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: header,
	}

	if err := connectReq.Write(conn); err != nil {
		return nil, trace.Wrap(err)
	}

	// Read in the response. http.ReadResponse will read in the status line, mime
	// headers, and potentially part of the response body. the body itself will
	// not be read, but kept around so it can be read later.
	br := bufio.NewReader(conn)
	// Per the above comment, we're only using ReadResponse to check the status
	// and then hand off the underlying connection to the caller.
	// resp.Body.Close() would drain conn and close it, we don't need to do it
	// here. Disabling bodyclose linter for this edge case.
	//nolint:bodyclose // avoid draining the connection
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, trace.BadParameter("unable to proxy connection: %v", resp.Status)
	}

	// Return a bufferedConn that wraps a net.Conn and a *bufio.Reader. this
	// needs to be done because http.ReadResponse will buffer part of the
	// response body in the *bufio.Reader that was passed in. reads must first
	// come from anything buffered, then from the underlying connection otherwise
	// data will be lost.
	return &bufferedConn{
		Conn:   conn,
		reader: br,
	}, nil
}

type socksDialerAdapter struct {
	dialer ContextDialer
}

func (d *socksDialerAdapter) Dial(network, addr string) (c net.Conn, err error) {
	return d.dialer.DialContext(context.Background(), network, addr)
}

// DialContext dials with context. Even though socks dialer interface requires just Dial() function
// internally it will use dialing with context.
func (d *socksDialerAdapter) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	return d.dialer.DialContext(ctx, network, addr)
}

// dialProxyWithSOCKSDialer creates a connection to a server via a SOCKS5 Proxy.
func dialProxyWithSOCKSDialer(
	ctx context.Context,
	proxyURL *url.URL,
	addr string,
	dialer ContextDialer,
) (net.Conn, error) {
	var proxyAuth *proxy.Auth
	if proxyURL.User != nil {
		proxyAuth = &proxy.Auth{
			User: proxyURL.User.Username(),
		}
		if password, ok := proxyURL.User.Password(); ok {
			proxyAuth.Password = password
		}
	}

	socksDialer, err := proxy.SOCKS5("tcp", proxyURL.Host, proxyAuth, &socksDialerAdapter{dialer: dialer})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctxDialer, ok := socksDialer.(ContextDialer)
	if !ok {
		return nil, trace.Errorf("failed type assertion: wanted ContextDialer got %T", socksDialer)
	}

	conn, err := ctxDialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return conn, nil
}

// bufferedConn is used when part of the data on a connection has already been
// read by a *bufio.Reader. Reads will first try and read from the
// *bufio.Reader and when everything has been read, reads will go to the
// underlying connection.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read first reads from the *bufio.Reader any data that has already been
// buffered. Once all buffered data has been read, reads go to the net.Conn.
func (bc *bufferedConn) Read(b []byte) (n int, err error) {
	if bc.reader.Buffered() > 0 {
		return bc.reader.Read(b)
	}
	return bc.Conn.Read(b)
}
