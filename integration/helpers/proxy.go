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

package helpers

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/fixtures"
	apitesthelpers "github.com/gravitational/teleport/api/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
)

// ProxyHandler is a http.Handler that implements a simple HTTP proxy server.
type ProxyHandler = apitesthelpers.ProxyHandler

type ProxyAuthorizer struct {
	next     http.Handler
	authUser string
	authPass string
	authMu   sync.Mutex
	waitersC chan chan error
}

func NewProxyAuthorizer(handler http.Handler, user, pass string) *ProxyAuthorizer {
	return &ProxyAuthorizer{
		next:     handler,
		authUser: user,
		authPass: pass,
		waitersC: make(chan chan error),
	}
}

func (p *ProxyAuthorizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	// we detect if someone is waiting for a new request to come in.
	select {
	case waiter := <-p.waitersC:
		defer func() {
			waiter <- err
		}()
	default:
	}
	defer func() {
		if err != nil {
			trace.WriteError(w, err)
		}
	}()
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		err = trace.AccessDenied("missing Proxy-Authorization header")
		return
	}
	user, password, ok := parseProxyAuth(auth)
	if !ok {
		err = trace.AccessDenied("bad Proxy-Authorization header")
		return
	}

	if !p.isAuthorized(user, password) {
		err = trace.AccessDenied("bad credentials")
		return
	}

	// request is authorized, send it to the next handler
	p.next.ServeHTTP(w, r)
}

// WaitForRequest waits (with a configured timeout) for a new request to be handled and returns the handler's error.
// This function makes no guarantees about which request error will be returned, except that the request
// error will have occurred after this function was called.
func (p *ProxyAuthorizer) WaitForRequest(timeout time.Duration) error {
	timeoutC := time.After(timeout)

	errC := make(chan error, 1)
	// wait for a new request to come in.
	select {
	case <-timeoutC:
		return trace.BadParameter("timed out waiting for request to proxy authorizer")
	case p.waitersC <- errC:
	}

	// get some error that occurred after the new request came in.
	select {
	case <-timeoutC:
		return trace.BadParameter("timed out waiting for proxy authorizer request error")
	case err := <-errC:
		return err
	}
}

func (p *ProxyAuthorizer) SetCredentials(user, pass string) {
	p.authMu.Lock()
	defer p.authMu.Unlock()
	p.authUser = user
	p.authPass = pass
}

func (p *ProxyAuthorizer) isAuthorized(user, pass string) bool {
	p.authMu.Lock()
	defer p.authMu.Unlock()
	return p.authUser == user && p.authPass == pass
}

// parse "Proxy-Authorization" header by leveraging the stdlib basic auth parsing for "Authorization" header
func parseProxyAuth(proxyAuth string) (user, password string, ok bool) {
	fakeHeader := make(http.Header)
	fakeHeader.Add("Authorization", proxyAuth)
	fakeReq := &http.Request{
		Header: fakeHeader,
	}
	return fakeReq.BasicAuth()
}

func MakeProxyAddr(user, pass, host string) string {
	userPass := url.UserPassword(user, pass).String()
	return fmt.Sprintf("%v@%v", userPass, host)
}

// MockAWSALBProxy is a mock proxy server that simulates an AWS application
// load balancer where ALPN is not supported. Note that this mock does not
// actually balance traffic.
type MockAWSALBProxy struct {
	net.Listener
	proxyAddr string
	cert      tls.Certificate
}

func (m *MockAWSALBProxy) serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := m.Accept()
		if err != nil {
			slog.DebugContext(ctx, "Failed to accept conn", "error", err)
			return
		}

		go func() {
			defer conn.Close()

			// Handshake with incoming client and drops ALPN.
			downstreamConn := tls.Server(conn, &tls.Config{
				Certificates: []tls.Certificate{m.cert},
			})

			// api.Client may try different connection methods. Just close the
			// connection when something goes wrong.
			if err := downstreamConn.HandshakeContext(ctx); err != nil {
				slog.DebugContext(ctx, "Failed to handshake", "error", err)
				return
			}

			// Make a connection to the proxy server with ALPN protos.
			upstreamConn, err := tls.Dial("tcp", m.proxyAddr, &tls.Config{
				InsecureSkipVerify: true,
			})
			if err != nil {
				slog.DebugContext(ctx, "Failed to dial upstream", "error", err)
				return
			}
			utils.ProxyConn(ctx, downstreamConn, upstreamConn)
		}()
	}
}

// MustStartMockALBProxy creates and starts a MockAWSALBProxy.
func MustStartMockALBProxy(t *testing.T, proxyAddr string) *MockAWSALBProxy {
	t.Helper()

	cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := &MockAWSALBProxy{
		proxyAddr: proxyAddr,
		Listener:  MustCreateListener(t),
		cert:      cert,
	}
	go m.serve(ctx)
	return m
}
