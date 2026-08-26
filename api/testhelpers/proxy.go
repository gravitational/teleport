// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testhelpers

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// ProxyHandler is a http.Handler that implements a simple HTTP proxy server.
type ProxyHandler struct {
	sync.Mutex
	count int
}

// ServeHTTP only accepts the CONNECT verb and will tunnel your connection to
// the specified host. Also tracks the number of connections that it proxies for
// debugging purposes.
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate http connect parameters.
	if r.Method != http.MethodConnect {
		trace.WriteError(w, trace.BadParameter("%v not supported", r.Method))
		return
	}
	if r.Host == "" {
		trace.WriteError(w, trace.BadParameter("host not set"))
		return
	}

	// Dial to the target host, this is done before hijacking the connection to
	// ensure the target host is accessible.
	dialer := net.Dialer{}
	dconn, err := dialer.DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		trace.WriteError(w, err)
		return
	}
	defer dconn.Close()

	// Once the client receives 200 OK, the rest of the data will no longer be
	// http, but whatever protocol is being tunneled.
	w.WriteHeader(http.StatusOK)

	// Hijack request so we can get underlying connection.
	hj, ok := w.(http.Hijacker)
	if !ok {
		trace.WriteError(w, trace.AccessDenied("unable to hijack connection"))
		return
	}
	sconn, buf, err := hj.Hijack()
	if err != nil {
		trace.WriteError(w, err)
		return
	}
	defer sconn.Close()

	// Success, we're proxying data now.
	p.Lock()
	p.count++
	p.Unlock()

	// Copy from src to dst and dst to src.
	errc := make(chan error, 2)
	replicate := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go replicate(sconn, dconn)
	go replicate(dconn, io.MultiReader(buf, sconn))

	// Wait until done.
	select {
	case <-r.Context().Done():
	case <-errc:
	}
}

// Count returns the number of requests that have been proxied.
func (p *ProxyHandler) Count() int {
	p.Lock()
	defer p.Unlock()
	return p.count
}

// Reset sets the counter for proxied requests to zero.
func (p *ProxyHandler) Reset() {
	p.Lock()
	defer p.Unlock()
	p.count = 0
}

// GetLocalIP gets the non-loopback IP address of this host.
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}
		if !ip.IsLoopback() && ip.IsPrivate() {
			return ip.String(), nil
		}
	}
	return "", trace.NotFound("No non-loopback local IP address found")
}

type TestServerOption func(*testing.T, *httptest.Server)

func WithTestServerAddress(ip string) TestServerOption {
	return func(t *testing.T, srv *httptest.Server) {
		// Replace the test server's address.
		_, originalPort, err := net.SplitHostPort(srv.Listener.Addr().String())
		require.NoError(t, err)
		require.NoError(t, srv.Listener.Close())
		l, err := net.Listen("tcp", net.JoinHostPort(ip, originalPort))
		require.NoError(t, err)
		srv.Listener = l
	}
}

func MakeTestServer(t *testing.T, h http.Handler, opts ...TestServerOption) *httptest.Server {
	svr := httptest.NewUnstartedServer(h)
	for _, opt := range opts {
		opt(t, svr)
	}
	svr.StartTLS()
	t.Cleanup(svr.Close)
	return svr
}
