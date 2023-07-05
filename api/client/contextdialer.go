/*
Copyright 2020 Gravitational, Inc.

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
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
)

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials the specified address
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// ContextDialerFunc is a function wrapper that implements the ContextDialer interface.
type ContextDialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// DialContext is a function that dials to the specified address
func (f ContextDialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

// newDirectDialer makes a new dialer to connect directly to an Auth server.
func newDirectDialer(keepAlivePeriod, dialTimeout time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlivePeriod,
	}
}

// tracedDialer ensures that the provided ContextDialerFunc is given a context
// which contains tracing information. In the event that a grpc dial occurs without
// a grpc.WithBlock dialing option, the context provided to the dial function will
// be context.Background(), which doesn't contain any tracing information. To get around
// this limitation, any tracing context from the provided context.Context will be extracted
// and used instead.
func tracedDialer(ctx context.Context, fn ContextDialerFunc) ContextDialerFunc {
	return func(dialCtx context.Context, network, addr string) (net.Conn, error) {
		traceCtx := dialCtx
		if spanCtx := oteltrace.SpanContextFromContext(dialCtx); !spanCtx.IsValid() {
			traceCtx = oteltrace.ContextWithSpanContext(traceCtx, oteltrace.SpanContextFromContext(ctx))
		}

		traceCtx, span := tracing.DefaultProvider().Tracer("dialer").Start(traceCtx, "client/DirectDial")
		defer span.End()

		return fn(traceCtx, network, addr)
	}
}

// NewDialer makes a new dialer that connects to an Auth server either directly or via an HTTP proxy, depending
// on the environment.
func NewDialer(ctx context.Context, keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	return tracedDialer(ctx, func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := newDirectDialer(keepAlivePeriod, dialTimeout)
		if proxyURL := utils.GetProxyURL(addr); proxyURL != nil {
			return DialProxyWithDialer(ctx, proxyURL, addr, dialer)
		}
		return dialer.DialContext(ctx, network, addr)
	})
}

// NewProxyDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
// The dialer will ping the web client to discover the tunnel proxy address on each dial.
func NewProxyDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure bool) ContextDialer {
	dialer := newTunnelDialer(ssh, keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
		resp, err := webclient.Find(&webclient.Config{Context: ctx, ProxyAddr: discoveryAddr, Insecure: insecure})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tunnelAddr, err := resp.Proxy.TunnelAddr()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err = dialer.DialContext(ctx, network, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return conn, nil
	})
}

// newTunnelDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
func newTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	dialer := newDirectDialer(keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		if proxyURL := utils.GetProxyURL(addr); proxyURL != nil {
			conn, err = DialProxyWithDialer(ctx, proxyURL, addr, dialer)
		} else {
			conn, err = dialer.DialContext(ctx, network, addr)
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshConnect(ctx, conn, ssh, dialTimeout, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sconn, nil
	})
}

// newTLSRoutingTunnelDialer makes a reverse tunnel TLS Routing dialer to connect to an Auth server
// through the SSH reverse tunnel on the proxy.
func newTLSRoutingTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure bool) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		resp, err := webclient.Find(&webclient.Config{Context: ctx, ProxyAddr: discoveryAddr, Insecure: insecure})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !resp.Proxy.TLSRoutingEnabled {
			return nil, trace.NotImplemented("TLS routing is not enabled")
		}

		tunnelAddr, err := resp.Proxy.TunnelAddr()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dialer := &net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: keepAlivePeriod,
		}
		conn, err = dialer.DialContext(ctx, network, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		host, _, err := webclient.ParseHostPort(tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tlsConn := tls.Client(conn, &tls.Config{
			NextProtos:         []string{constants.ALPNSNIProtocolReverseTunnel},
			InsecureSkipVerify: insecure,
			ServerName:         host,
		})
		if err := tlsConn.Handshake(); err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshConnect(ctx, tlsConn, ssh, dialTimeout, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return sconn, nil
	})
}

// sshConnect upgrades the underling connection to ssh and connects to the Auth service.
func sshConnect(ctx context.Context, conn net.Conn, ssh ssh.ClientConfig, dialTimeout time.Duration, addr string) (net.Conn, error) {
	ssh.Timeout = dialTimeout
	sconn, err := tracessh.NewClientConnWithDeadline(ctx, conn, addr, &ssh)
	if err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}

	// Build a net.Conn over the tunnel. Make this an exclusive connection:
	// close the net.Conn as well as the channel upon close.
	conn, _, err = sshutils.ConnectProxyTransport(sconn.Conn, &sshutils.DialReq{
		Address: constants.RemoteAuthServer,
	}, true)
	if err != nil {
		return nil, trace.NewAggregate(err, sconn.Close())
	}
	return conn, nil
}
