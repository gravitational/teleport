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
	"crypto/x509/pkix"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestForwardProxy(t *testing.T) {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)

	ca, err := tls.X509KeyPair(caCert, caKey)
	require.NoError(t, err)

	// Use a different status code for each destination.
	receiverCode := http.StatusAccepted
	originalHostCode := http.StatusCreated

	// Setup a receiver that wants a specific domain. The receiver uses
	// CertGenListener to generate certificate for this domain on the fly.
	receiverListener := mustCreateCertGenListener(t, ca)
	receiverHandler := NewForwardToHostHandler(ForwardToHostHandlerConfig{
		MatchFunc: func(req *http.Request) bool {
			return req.Host == "receiver.wanted.com:443"
		},
		Host: receiverListener.Addr().String(),
	})
	go http.Serve(receiverListener, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(receiverCode)
	}))

	// Setup a HTTPS server to simulate original host.
	originalHostListener := mustCreateCertGenListener(t, ca)
	originalHostAddress := strings.ReplaceAll(originalHostListener.Addr().String(), "127.0.0.1", "localhost")
	go http.Serve(originalHostListener, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(originalHostCode)
	}))

	// client -> forward proxy -> receiver
	t.Run("to receiver", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, receiverHandler, NewForwardToOriginalHostHandler())
		client := httpsClientWithProxyURL(forwardProxy.GetAddr(), caCert)

		mustCallHTTPSServerAndReceiveCode(t, "receiver.wanted.com:443", *client, receiverCode)
	})

	// client -> forward proxy -> original host
	t.Run("to original host", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, receiverHandler, NewForwardToOriginalHostHandler())
		client := httpsClientWithProxyURL(forwardProxy.GetAddr(), caCert)

		mustCallHTTPSServerAndReceiveCode(t, originalHostAddress, *client, originalHostCode)
	})

	// client -> forward proxy -> system proxy -> original host
	t.Run("to system proxy", func(t *testing.T) {
		systemProxyHTTPServer := createSystemProxy(t, mustCreateLocalListener(t))

		forwardToSystemProxyHandler := NewForwardToSystemProxyHandler(ForwardToSystemProxyHandlerConfig{
			SystemProxyFunc: func(*url.URL) (*url.URL, error) {
				return url.Parse("http://" + systemProxyHTTPServer.GetAddr())
			},
		})

		forwardProxy := createForwardProxy(t, forwardToSystemProxyHandler)
		client := httpsClientWithProxyURL(forwardProxy.GetAddr(), caCert)

		mustCallHTTPSServerAndReceiveCode(t, originalHostAddress, *client, originalHostCode)
	})

	// client -> forward proxy -> system proxy (HTTPS) -> original host
	t.Run("to system proxy (HTTPS)", func(t *testing.T) {
		// This test is the same as previous one except the system proxy is a
		// HTTPS server.
		systemProxyHTTPSServer := createSystemProxy(t, mustCreateCertGenListener(t, ca))

		forwardToSystemProxyHandler := NewForwardToSystemProxyHandler(ForwardToSystemProxyHandlerConfig{
			InsecureSystemProxy: true,
			SystemProxyFunc: func(*url.URL) (*url.URL, error) {
				return url.Parse("https://" + systemProxyHTTPSServer.GetAddr())
			},
		})

		forwardProxy := createForwardProxy(t, forwardToSystemProxyHandler)
		client := httpsClientWithProxyURL(forwardProxy.GetAddr(), caCert)

		mustCallHTTPSServerAndReceiveCode(t, originalHostAddress, *client, originalHostCode)
	})
}

// createForwardProxy creates a ForwardProxy with provided handlers.
func createForwardProxy(t *testing.T, handlers ...ConnectRequestHandler) *ForwardProxy {
	return createForwardProxyWithConfig(t, ForwardProxyConfig{
		Listener: mustCreateLocalListener(t),
		Handlers: handlers,
	})
}

// createSystemProxy creates a ForwardProxy to simulate a system proxy.
func createSystemProxy(t *testing.T, listener net.Listener) *ForwardProxy {
	return createForwardProxyWithConfig(t, ForwardProxyConfig{
		Listener: listener,
		Handlers: []ConnectRequestHandler{
			NewForwardToOriginalHostHandler(),
		},
	})
}

// createForwardProxyWithConfig creates a ForwardProxy with provided config.
func createForwardProxyWithConfig(t *testing.T, config ForwardProxyConfig) *ForwardProxy {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	config.CloseContext = ctx

	forwardProxy, err := NewForwardProxy(config)
	require.NoError(t, err)

	t.Cleanup(func() {
		forwardProxy.Close()
	})

	go func() {
		assert.NoError(t, forwardProxy.Start())
	}()
	return forwardProxy
}

func TestMatchGCPRequests(t *testing.T) {
	mkReq := func(url string) *http.Request {
		request, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		return request
	}

	tests := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{
			name: "non gcp, no port",
			req:  mkReq("https://localhost/foo/bar"),
			want: false,
		},
		{
			name: "non gcp, port",
			req:  mkReq("https://localhost:8080/foo/bar"),
			want: false,
		},
		{
			name: "non gcp, port, http",
			req:  mkReq("http://localhost:1234/foo/bar"),
			want: false,
		},
		{
			name: "non gcp, port, no schema",
			req:  mkReq("localhost:1234/foo/bar"),
			want: false,
		},

		{
			name: "gcp, no port",
			req:  mkReq("https://compute.googleapis.com/foo/bar"),
			want: true,
		},
		{
			name: "gcp, port",
			req:  mkReq("https://compute.googleapis.com:8080/foo/bar"),
			want: true,
		},
		{
			name: "gcp, port, http",
			req:  mkReq("http://compute.googleapis.com:1234/foo/bar"),
			want: true,
		},

		// NOTE: we only want to match XXX.googleapis.com.
		{
			name: "top level gcp, no port",
			req:  mkReq("https://googleapis.com/foo/bar"),
			want: false,
		},
		{
			name: "top level gcp, port",
			req:  mkReq("https://googleapis.com:8080/foo/bar"),
			want: false,
		},
		{
			name: "top level gcp, port, http",
			req:  mkReq("http://googleapis.com:1234/foo/bar"),
			want: false,
		},

		{
			name: "fake gcp, no port",
			req:  mkReq("https://compute.googleapis.com.fake.com/foo/bar"),
			want: false,
		},
		{
			name: "fake gcp, port",
			req:  mkReq("https://compute.googleapis.com.fake.com:8080/foo/bar"),
			want: false,
		},
		{
			name: "fake gcp, port, http",
			req:  mkReq("http://compute.googleapis.com.fake.com:1234/foo/bar"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MatchGCPRequests(tt.req))
		})
	}
}

func TestMatchAWSRequests(t *testing.T) {
	makeRequest := func(url string) *http.Request {
		// Forward proxy always receives CONNECT requests.
		request, err := http.NewRequest("CONNECT", url, nil)
		require.NoError(t, err)
		return request
	}
	tests := []struct {
		name  string
		req   *http.Request
		check require.BoolAssertionFunc
	}{
		{
			name:  "AWS request",
			req:   makeRequest("http://s3.ca-central-1.amazonaws.com"),
			check: require.True,
		},
		{
			name:  "non-AWS request",
			req:   makeRequest("https://registry.terraform.io"),
			check: require.False,
		},
		{
			name:  "SSM API",
			req:   makeRequest("https://ssm.ca-central-1.amazonaws.com"),
			check: require.True,
		},
		{
			name:  "SSM session WebSocket",
			req:   makeRequest("wss://ssmmessages.ca-central-1.amazonaws.com"),
			check: require.False,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, MatchAWSRequests(tt.req))
		})
	}
}
