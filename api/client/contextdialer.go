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
	"crypto/x509"
	"net"
	"net/url"
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

type dialConfig struct {
	tlsConfig *tls.Config
	// alpnConnUpgradeRequired specifies if ALPN connection upgrade is
	// required.
	alpnConnUpgradeRequired bool
	// alpnConnUpgradeWithPing specifies if Ping is required during ALPN
	// connection upgrade. This is only effective when alpnConnUpgradeRequired
	// is true.
	alpnConnUpgradeWithPing bool
	// proxyHeaderGetter is used if present to get signed PROXY headers to propagate client's IP.
	// Used by proxy's web server to make calls on behalf of connected clients.
	proxyHeaderGetter PROXYHeaderGetter
	// proxyURLFunc is a function used to get ProxyURL. Defaults to
	// utils.GetProxyURL if not specified. Currently only used in tests to
	// overwrite the ProxyURL as httpproxy.FromEnvironment skips localhost
	// proxies.
	proxyURLFunc func(dialAddr string) *url.URL
	// baseDialer is the base dialer used for dialing. If not specified, a
	// direct net.Dialer will be used. Currently only used in tests.
	baseDialer ContextDialer
}

func (c *dialConfig) getProxyURL(dialAddr string) *url.URL {
	if c.proxyURLFunc != nil {
		return c.proxyURLFunc(dialAddr)
	}
	return utils.GetProxyURL(dialAddr)
}

// WithInsecureSkipVerify specifies if dialing insecure when using an HTTPS proxy.
func WithInsecureSkipVerify(insecure bool) DialOption {
	return func(cfg *dialProxyConfig) {
		cfg.tlsConfig = &tls.Config{
			InsecureSkipVerify: insecure,
		}
	}
}

// WithALPNConnUpgrade specifies if ALPN connection upgrade is required.
func WithALPNConnUpgrade(alpnConnUpgradeRequired bool) DialOption {
	return func(cfg *dialProxyConfig) {
		cfg.alpnConnUpgradeRequired = alpnConnUpgradeRequired
	}
}

// WithALPNConnUpgradePing specifies if Ping is required during ALPN connection
// upgrade. This is only effective when alpnConnUpgradeRequired is true.
func WithALPNConnUpgradePing(alpnConnUpgradeWithPing bool) DialOption {
	return func(cfg *dialProxyConfig) {
		cfg.alpnConnUpgradeWithPing = alpnConnUpgradeWithPing
	}
}

func withProxyURL(proxyURL *url.URL) DialProxyOption {
	return func(cfg *dialProxyConfig) {
		cfg.proxyURLFunc = func(_ string) *url.URL {
			return proxyURL
		}
	}
}
func withBaseDialer(dialer ContextDialer) DialProxyOption {
	return func(cfg *dialProxyConfig) {
		cfg.baseDialer = dialer
	}
}

// WithPROXYHeaderGetter provides PROXY headers signer so client's real IP could be propagated.
// Used by proxy's web server to make calls on behalf of connected clients.
func WithPROXYHeaderGetter(proxyHeaderGetter PROXYHeaderGetter) DialProxyOption {
	return func(cfg *dialProxyConfig) {
		cfg.proxyHeaderGetter = proxyHeaderGetter
	}
}

// DialOption allows setting options as functional arguments to api.NewDialer.
type DialOption func(cfg *dialConfig)

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

func newProxyURLDialer(proxyURL *url.URL, dialer ContextDialer, opts ...DialProxyOption) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return DialProxyWithDialer(ctx, proxyURL, addr, dialer, opts...)
	})
}

// NewPROXYHeaderDialer makes a new dialer that can propagate client IP if signed PROXY header getter is present
func NewPROXYHeaderDialer(dialer ContextDialer, headerGetter PROXYHeaderGetter) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if headerGetter != nil {
			signedHeader, err := headerGetter()
			if err != nil {
				conn.Close()
				return nil, trace.Wrap(err)
			}
			_, err = conn.Write(signedHeader)
			if err != nil {
				conn.Close()
				return nil, trace.Wrap(err)
			}
		}

		return conn, nil
	})
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
func NewDialer(ctx context.Context, keepAlivePeriod, dialTimeout time.Duration, opts ...DialOption) ContextDialer {
	var cfg dialConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	return tracedDialer(ctx, func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Base direct dialer.
		var dialer ContextDialer = cfg.baseDialer
		if dialer == nil {
			dialer = newDirectDialer(keepAlivePeriod, dialTimeout)
		}

		// Currently there is no use case where both cfg.proxyHeaderGetter and
		// cfg.alpnConnUpgradeRequired are set.
		if cfg.proxyHeaderGetter != nil && cfg.alpnConnUpgradeRequired {
			return nil, trace.NotImplemented("ALPN connection upgrade does not support multiplexer header")
		}

		// Wrap with PROXY header dialer if getter is present.
		// Used by Proxy's web server to propagate real client IP when making calls on behalf of connected clients
		if cfg.proxyHeaderGetter != nil {
			dialer = NewPROXYHeaderDialer(dialer, cfg.proxyHeaderGetter)
		}

		// Wrap with proxy URL dialer if proxy URL is detected.
		if proxyURL := cfg.getProxyURL(addr); proxyURL != nil {
			dialer = newProxyURLDialer(proxyURL, dialer, opts...)
		}

		// Wrap with alpnConnUpgradeDialer if upgrade is required for TLS Routing.
		if cfg.alpnConnUpgradeRequired {
			dialer = newALPNConnUpgradeDialer(dialer, cfg.tlsConfig, cfg.alpnConnUpgradeWithPing)
		}

		// Dial.
		return dialer.DialContext(ctx, network, addr)
	})
}

// NewProxyDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
// The dialer will ping the web client to discover the tunnel proxy address on each dial.
func NewProxyDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure bool, opts ...DialProxyOption) ContextDialer {
	dialer := newTunnelDialer(ssh, keepAlivePeriod, dialTimeout, opts...)
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

// GRPCContextDialer converts a ContextDialer to a function used for
// grpc.WithContextDialer.
func GRPCContextDialer(dialer ContextDialer) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		return conn, trace.Wrap(err)
	}
}

// newTunnelDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
func newTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, opts ...DialProxyOption) ContextDialer {
	dialer := newDirectDialer(keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		if proxyURL := utils.GetProxyURL(addr); proxyURL != nil {
			conn, err = DialProxyWithDialer(ctx, proxyURL, addr, dialer, opts...)
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

// newTLSRoutingWithConnUpgradeDialer makes a reverse tunnel TLS Routing dialer
// through the web proxy with ALPN connection upgrade.
func newTLSRoutingWithConnUpgradeDialer(ssh ssh.ClientConfig, params connectParams) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		insecure := params.cfg.InsecureAddressDiscovery
		resp, err := webclient.Find(&webclient.Config{
			Context:   ctx,
			ProxyAddr: params.addr,
			Insecure:  insecure,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !resp.Proxy.TLSRoutingEnabled {
			return nil, trace.NotImplemented("TLS routing is not enabled")
		}

		host, _, err := webclient.ParseHostPort(params.addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conn, err := DialALPN(ctx, params.addr, ALPNDialerConfig{
			DialTimeout:     params.cfg.DialTimeout,
			KeepAlivePeriod: params.cfg.KeepAlivePeriod,
			TLSConfig: &tls.Config{
				NextProtos:         []string{constants.ALPNSNIProtocolReverseTunnel},
				InsecureSkipVerify: insecure,
				ServerName:         host,
			},
			ALPNConnUpgradeRequired: IsALPNConnUpgradeRequired(ctx, params.addr, insecure),
			GetClusterCAs: func(_ context.Context) (*x509.CertPool, error) {
				// Uses the Root CAs from the TLS Config of the Credentials.
				return params.tlsConfig.RootCAs, nil
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshConnect(ctx, conn, ssh, params.cfg.DialTimeout, params.addr)
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
